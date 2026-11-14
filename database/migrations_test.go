package database_test

import (
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/civil-labs/civil-os/internal/testutil"
)

// TestAtlasSum checks database/migrations/atlas.sum against the migration
// files without needing Atlas (CI has no Atlas license): same file list, same
// order, and the same cumulative hashes Atlas computes. A failure means a
// migration was added, removed or edited without `mise run db:diff`.
func TestAtlasSum(t *testing.T) {
	dir := filepath.Join(testutil.RepoRoot(), "database", "migrations")

	f, err := os.Open(filepath.Join(dir, "atlas.sum"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var total string
	type entry struct{ name, hash string }
	var want []entry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if total == "" {
			total = line
			continue
		}
		name, hash, ok := strings.Cut(line, " ")
		if !ok {
			t.Fatalf("malformed atlas.sum line %q", line)
		}
		want = append(want, entry{name, hash})
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}

	files, err := testutil.MigrationFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(want) {
		t.Fatalf("atlas.sum lists %d migrations, directory has %d", len(want), len(files))
	}

	// Atlas hashes each file name and content into one running SHA-256 and
	// records the running sum after each file. The first line is a SHA-256
	// over every file name followed by its base64 hash.
	h := sha256.New()
	sum := sha256.New()
	for i, path := range files {
		name := filepath.Base(path)
		if name != want[i].name {
			t.Fatalf("migration %d: atlas.sum has %s, directory has %s", i, want[i].name, name)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		h.Write([]byte(name))
		h.Write(content)
		fileHash := base64.StdEncoding.EncodeToString(h.Sum(nil))
		sum.Write([]byte(name))
		sum.Write([]byte(fileHash))
		if got := "h1:" + fileHash; got != want[i].hash {
			t.Errorf("%s: hash %s, atlas.sum has %s (file edited after atlas migrate diff?)", name, got, want[i].hash)
		}
	}
	if got := "h1:" + base64.StdEncoding.EncodeToString(sum.Sum(nil)); got != total {
		t.Errorf("total hash %s, atlas.sum has %s", got, total)
	}
}
