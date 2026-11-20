//go:build perf

package main

import (
	"bufio"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	parcelsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parcels/v1"
	"github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/parcels/v1/parcelsv1connect"
	tu "github.com/civil-labs/civil-os/internal/testutil"
)

// TestSearchLatency is the address search performance check
// (docs/parcel-search/spec.md, "Performance check"; plan step 2.9). It loads
// 500,000 extra parcels into a fixture database, times the trigram index
// build, and runs a fixed query set against the in-process reader.
//
//	mise run perf:search
//	PERF_SEARCH_RUNS=20 mise run perf:search   # a quick run: 20 per kind
//
// The plans (EXPLAIN ANALYZE) are printed before the timed runs, and each
// kind's percentiles as soon as it finishes, so a slow kind can't hide them.
const (
	perfDefaultRuns = 200
	perfWarmup      = 20
	perfBudgetP95   = 200 * time.Millisecond
	perfBudgetP99   = 500 * time.Millisecond
	searchIndex     = "idx_address_attributes_formatted_address_trgm"
)

type perfSample struct {
	house, street, suffix, locality, formatted string
}

// perfKinds are the spec's five query kinds, each built from a sample address.
var perfKinds = []struct {
	name  string
	query func(s perfSample) string
}{
	{"house-number prefix", func(s perfSample) string { return s.house + " " + s.street[:3] }},
	{"street fragment", func(s perfSample) string { return s.street[1:min(len(s.street), 6)] }},
	{"full address", func(s perfSample) string { return s.formatted }},
	{"one-typo street", func(s perfSample) string {
		// Drop one letter from the middle of the street name.
		m := len(s.street) / 2
		return s.street[:m] + s.street[m+1:] + " " + s.suffix
	}},
	{"broad (cap)", func(s perfSample) string { return s.suffix }},
}

func TestSearchLatency(t *testing.T) {
	ctx := context.Background()
	pool := tu.FixtureDatabase(t).Pool(t)

	// Load without the index, then build it alone, so its time can be
	// reported for the production migration.
	var indexDef string
	if err := pool.QueryRow(ctx, `SELECT indexdef FROM pg_indexes WHERE indexname = $1`, searchIndex).Scan(&indexDef); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "DROP INDEX core."+searchIndex); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tu.ExecFiles(ctx, conn.Conn(), filepath.Join(tu.RepoRoot(), "dev", "perf", "search-dataset.sql")); err != nil {
		t.Fatal(err)
	}
	conn.Release()
	loadTime := time.Since(start)
	start = time.Now()
	if _, err := pool.Exec(ctx, indexDef); err != nil {
		t.Fatal(err)
	}
	indexTime := time.Since(start)
	if _, err := pool.Exec(ctx, "ANALYZE core.address_attributes"); err != nil {
		t.Fatal(err)
	}

	printEnvironment(t, pool)
	t.Logf("dataset load: %s; trigram index build (CREATE INDEX, not CONCURRENTLY): %s", loadTime.Round(time.Millisecond), indexTime.Round(time.Millisecond))

	samples := perfSamples(t, pool)
	srv := httptest.NewServer(newHandler(pool, slog.New(slog.DiscardHandler)))
	t.Cleanup(srv.Close)
	client := parcelsv1connect.NewParcelReadServiceClient(http.DefaultClient, srv.URL)

	call := func(q string) (time.Duration, int) {
		first := int32(defaultPageSize)
		start := time.Now()
		res, err := client.SearchParcelsAddresses(ctx, connect.NewRequest(&parcelsv1.SearchParcelsAddressesRequest{Query: q, First: &first}))
		d := time.Since(start)
		if err != nil {
			t.Fatalf("search %q: %v", q, err)
		}
		return d, len(res.Msg.Hits)
	}

	for _, k := range perfKinds {
		explain(t, pool, k.name, k.query(samples[0]))
	}

	runs := perfDefaultRuns
	if v := os.Getenv("PERF_SEARCH_RUNS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			t.Fatalf("PERF_SEARCH_RUNS = %q, want a positive integer", v)
		}
		runs = n
	}
	t.Logf("%d timed runs per kind", runs)

	for i := range perfWarmup {
		k := perfKinds[i%len(perfKinds)]
		call(k.query(samples[i%len(samples)]))
	}

	var all []time.Duration
	t.Logf("%-20s %8s %8s %8s %8s %10s", "kind", "p50", "p95", "p99", "max", "mean hits")
	for _, k := range perfKinds {
		var times []time.Duration
		hits := 0
		kindStart := time.Now()
		for i := range runs {
			d, n := call(k.query(samples[i%len(samples)]))
			times = append(times, d)
			hits += n
		}
		all = append(all, times...)
		p50, p95, p99, top := percentiles(times)
		t.Logf("%-20s %8s %8s %8s %8s %10.1f  (%s total)", k.name, p50, p95, p99, top, float64(hits)/float64(runs), time.Since(kindStart).Round(time.Second))
	}
	p50, p95, p99, top := percentiles(all)
	t.Logf("%-20s %8s %8s %8s %8s", "overall", p50, p95, p99, top)

	if p95 > perfBudgetP95 || p99 > perfBudgetP99 {
		t.Errorf("over budget: p95 %s (budget %s), p99 %s (budget %s)", p95, perfBudgetP95, p99, perfBudgetP99)
	}
}

// perfSamples returns 200 dataset addresses spread across the table, in a
// fixed order.
func perfSamples(t *testing.T, pool *pgxpool.Pool) []perfSample {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT split_part(aa.address_line_1, ' ', 1), split_part(aa.address_line_1, ' ', 2),
		       split_part(aa.address_line_1, ' ', 3), aa.locality, aa.formatted_address
		FROM core.address_attributes aa JOIN core.addresses a USING (address_id)
		WHERE a.legacy_id LIKE 'PERF-ADDR-%' AND a.address_id % 2500 = 0
		ORDER BY a.address_id
		LIMIT 200`)
	if err != nil {
		t.Fatal(err)
	}
	samples, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (perfSample, error) {
		var s perfSample
		return s, r.Scan(&s.house, &s.street, &s.suffix, &s.locality, &s.formatted)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) < 100 {
		t.Fatalf("got %d samples, want about 200", len(samples))
	}
	return samples
}

func percentiles(d []time.Duration) (p50, p95, p99, top time.Duration) {
	s := slices.Clone(d)
	slices.Sort(s)
	at := func(p float64) time.Duration {
		return s[min(len(s)-1, int(p*float64(len(s))))].Round(100 * time.Microsecond)
	}
	return at(0.50), at(0.95), at(0.99), s[len(s)-1].Round(100 * time.Microsecond)
}

func printEnvironment(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	cpu := "unknown"
	if f, err := os.Open("/proc/cpuinfo"); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if k, v, ok := strings.Cut(sc.Text(), ":"); ok && strings.TrimSpace(k) == "model name" {
				cpu = strings.TrimSpace(v)
				break
			}
		}
		f.Close()
	}
	var version, sharedBuffers, workMem string
	var parcels, visible, addressRows int
	err := pool.QueryRow(ctx, `SELECT version(), current_setting('shared_buffers'), current_setting('work_mem'),
		(SELECT count(*) FROM core.parcels),
		(SELECT count(*) FROM core.parcels p JOIN geo.parcel_geometry pg ON pg.parcel_id = p.parcel_id AND pg.legal_valid_range @> now()
		 WHERE NOT p.is_voided),
		(SELECT count(*) FROM core.address_attributes)`).
		Scan(&version, &sharedBuffers, &workMem, &parcels, &visible, &addressRows)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("machine: %s/%s, %d CPUs, %s", runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), cpu)
	t.Logf("database: %s (testcontainers defaults: shared_buffers %s, work_mem %s)", strings.SplitN(version, ",", 2)[0], sharedBuffers, workMem)
	t.Logf("rows: %d parcels (%d visible now), %d address_attributes", parcels, visible, addressRows)
}

// explain prints the plan of the reader's query for q, to show which
// indexes it uses.
func explain(t *testing.T, pool *pgxpool.Pool, kind, query string) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var q string
	if err := tx.QueryRow(ctx, "SELECT core.search_normalize($1::text)", query).Scan(&q); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "SELECT set_config('pg_trgm.word_similarity_threshold', $1, true)", searchSimilarityThreshold); err != nil {
		t.Fatal(err)
	}
	rows, err := tx.Query(ctx, "EXPLAIN (ANALYZE, BUFFERS) "+searchSQL,
		time.Now().UTC().Truncate(time.Microsecond), q, escapeLike(q), defaultPageSize+1, defaultSearchCap, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("EXPLAIN (ANALYZE, BUFFERS), %s, query %q:\n%s", kind, query, strings.Join(plan, "\n"))
	if !strings.Contains(strings.Join(plan, "\n"), searchIndex) {
		t.Errorf("%s: the plan doesn't use %s", kind, searchIndex)
	}
}
