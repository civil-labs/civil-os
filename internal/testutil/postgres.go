package testutil

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// PostgresImage is the PostGIS image used by tests and compose.yaml.
const PostgresImage = "postgis/postgis:18-3.6"

// SearchPath mirrors the database-level search_path that cluster
// infrastructure sets; the reader's unqualified table names rely on it.
const SearchPath = "core, geo, extensions, public"

const (
	pgUser     = "postgres"
	pgPassword = "civil-os-test"
	templateDB = "civil_fixture_template"
)

// Database is one test database inside the shared container.
type Database struct {
	Host     string
	Port     uint16
	User     string
	Password string
	Name     string
}

// URL returns a postgres:// connection string for the database.
func (d Database) URL() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(d.User, d.Password),
		Host:     net.JoinHostPort(d.Host, strconv.Itoa(int(d.Port))),
		Path:     "/" + d.Name,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

// Pool opens a pgx pool to the database, closed when the test ends.
func (d Database) Pool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), d.URL())
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// RepoRoot returns the repository root, found relative to this file.
func RepoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// MigrationFiles returns database/migrations/*.sql in filename order, which
// is the order Atlas applies them.
func MigrationFiles() ([]string, error) {
	files, err := filepath.Glob(filepath.Join(RepoRoot(), "database", "migrations", "*.sql"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// ExecFiles runs each SQL file on conn with the simple protocol, so files may
// hold several statements and their own BEGIN/COMMIT.
func ExecFiles(ctx context.Context, conn *pgx.Conn, files ...string) error {
	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
	}
	return nil
}

type server struct {
	host string
	port uint16
	err  error
}

var (
	serverOnce   sync.Once
	shared       server
	templateOnce sync.Once
	templateErr  error
	dbCounter    atomic.Int64
)

// startServer starts one PostGIS container per test binary. Testcontainers'
// reaper removes it when the process exits.
func startServer() server {
	serverOnce.Do(func() {
		ctx := context.Background()
		c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        PostgresImage,
				ExposedPorts: []string{"5432/tcp"},
				Env: map[string]string{
					"POSTGRES_USER":     pgUser,
					"POSTGRES_PASSWORD": pgPassword,
				},
				// The entrypoint restarts postgres once after init, so wait
				// for the second "ready" line.
				WaitingFor: wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).WithStartupTimeout(2 * time.Minute),
			},
			Started: true,
		})
		if err != nil {
			shared.err = fmt.Errorf("start %s: %w", PostgresImage, err)
			return
		}
		host, err := c.Host(ctx)
		if err != nil {
			shared.err = err
			return
		}
		port, err := c.MappedPort(ctx, "5432/tcp")
		if err != nil {
			shared.err = err
			return
		}
		shared = server{host: host, port: port.Num()}
	})
	return shared
}

func (s server) db(name string) Database {
	return Database{Host: s.host, Port: s.port, User: pgUser, Password: pgPassword, Name: name}
}

// createDB creates a database from the given template. template0 is used
// for an empty database because the image installs PostGIS into template1,
// which would clash with the migrations' own CREATE EXTENSION.
func (s server) createDB(ctx context.Context, name, template string) error {
	conn, err := pgx.Connect(ctx, s.db("postgres").URL())
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s",
		pgx.Identifier{name}.Sanitize(), pgx.Identifier{template}.Sanitize())); err != nil {
		return fmt.Errorf("create database %s: %w", name, err)
	}
	// Per-database settings are not copied from a template.
	_, err = conn.Exec(ctx, fmt.Sprintf("ALTER DATABASE %s SET search_path = %s",
		pgx.Identifier{name}.Sanitize(), SearchPath))
	return err
}

func newName(t testing.TB) string {
	base := strings.ToLower(t.Name())
	base = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '_'
	}, base)
	if len(base) > 40 {
		base = base[:40]
	}
	return fmt.Sprintf("t%d_%s", dbCounter.Add(1), base)
}

func requireServer(t testing.TB) server {
	t.Helper()
	s := startServer()
	if s.err != nil {
		t.Fatalf("postgres test container: %v", s.err)
	}
	return s
}

// EmptyDatabase returns a new database with no schema applied.
func EmptyDatabase(t testing.TB) Database {
	t.Helper()
	s := requireServer(t)
	name := newName(t)
	if err := s.createDB(context.Background(), name, "template0"); err != nil {
		t.Fatal(err)
	}
	return s.db(name)
}

// FixtureDatabase returns a new database with every migration and
// database/fixtures/synthetic.sql applied. Each call gets its own copy, so
// tests may write freely.
func FixtureDatabase(t testing.TB) Database {
	t.Helper()
	s := requireServer(t)
	ctx := context.Background()
	templateOnce.Do(func() { templateErr = buildTemplate(ctx, s) })
	if templateErr != nil {
		t.Fatalf("build fixture template: %v", templateErr)
	}
	name := newName(t)
	if err := s.createDB(ctx, name, templateDB); err != nil {
		t.Fatal(err)
	}
	return s.db(name)
}

func buildTemplate(ctx context.Context, s server) error {
	if err := s.createDB(ctx, templateDB, "template0"); err != nil {
		return err
	}
	conn, err := pgx.Connect(ctx, s.db(templateDB).URL())
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	files, err := MigrationFiles()
	if err != nil {
		return err
	}
	files = append(files, filepath.Join(RepoRoot(), "database", "fixtures", "synthetic.sql"))
	return ExecFiles(ctx, conn, files...)
}
