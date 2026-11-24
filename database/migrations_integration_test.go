//go:build integration

package database_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/civil-labs/civil-os/internal/testutil"
)

// TestMigrationsApplyToEmptyDatabase applies every migration, in order, to a
// database created from template0.
func TestMigrationsApplyToEmptyDatabase(t *testing.T) {
	ctx := context.Background()
	db := testutil.EmptyDatabase(t)
	conn, err := pgx.Connect(ctx, db.URL())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)

	files, err := testutil.MigrationFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no migrations found")
	}
	if err := testutil.ExecFiles(ctx, conn, files...); err != nil {
		t.Fatal(err)
	}

	for _, fn := range []string{"core.get_parcel_tiles", "core.get_primary_improvements", "core.search_normalize"} {
		var ok bool
		if err := conn.QueryRow(ctx, "SELECT to_regproc($1) IS NOT NULL", fn).Scan(&ok); err != nil || !ok {
			t.Errorf("function %s missing after migrations (err=%v)", fn, err)
		}
	}

	// Address search (docs/parcel-search/spec.md): pg_trgm lives in the
	// extensions schema, and the trigram index is GIN over the normalised
	// address.
	var trgmSchema string
	if err := conn.QueryRow(ctx, `SELECT n.nspname FROM pg_extension e
		JOIN pg_namespace n ON n.oid = e.extnamespace WHERE e.extname = 'pg_trgm'`).Scan(&trgmSchema); err != nil {
		t.Errorf("pg_trgm not installed: %v", err)
	} else if trgmSchema != "extensions" {
		t.Errorf("pg_trgm is in schema %q, want extensions", trgmSchema)
	}

	var indexdef string
	if err := conn.QueryRow(ctx, `SELECT indexdef FROM pg_indexes
		WHERE schemaname = 'core' AND tablename = 'address_attributes'
		  AND indexname = 'idx_address_attributes_formatted_address_trgm'`).Scan(&indexdef); err != nil {
		t.Errorf("address search index missing: %v", err)
	} else {
		for _, want := range []string{"USING gin", "search_normalize(formatted_address)", "gin_trgm_ops"} {
			if !strings.Contains(indexdef, want) {
				t.Errorf("address search index %q does not contain %q", indexdef, want)
			}
		}
	}
}

// TestSearchNormalize checks core.search_normalize, which the address search
// applies to both the query and the indexed address.
func TestSearchNormalize(t *testing.T) {
	ctx := context.Background()
	pool := testutil.FixtureDatabase(t).Pool(t)

	for _, tc := range []struct {
		in, want *string
	}{
		{ptr("12 Fixture Way"), ptr("12 fixture way")},
		{ptr("12, FIXTURE-way"), ptr("12 fixture way")},
		{ptr("  --12   Fixture__Way!! "), ptr("12 fixture way")},
		{ptr("#4B"), ptr("4b")},
		{ptr("--"), ptr("")},
		{nil, nil}, // STRICT: NULL in, NULL out
	} {
		var got *string
		if err := pool.QueryRow(ctx, "SELECT core.search_normalize($1::text)", tc.in).Scan(&got); err != nil {
			t.Fatalf("search_normalize(%v): %v", show(tc.in), err)
		}
		if show(got) != show(tc.want) {
			t.Errorf("search_normalize(%s) = %s, want %s", show(tc.in), show(got), show(tc.want))
		}
	}

	// The fixtures are too small for the planner to choose the index on its
	// own, so seqscans are disabled for this one transaction.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SET LOCAL enable_seqscan = off"); err != nil {
		t.Fatal(err)
	}
	rows, err := tx.Query(ctx, `EXPLAIN SELECT address_id FROM core.address_attributes
		WHERE core.search_normalize(formatted_address) LIKE '%fixture%'`)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(plan, "\n"); !strings.Contains(joined, "idx_address_attributes_formatted_address_trgm") {
		t.Errorf("LIKE on the normalised address does not use the trigram index:\n%s", joined)
	}
}

func ptr(s string) *string { return &s }

func show(s *string) string {
	if s == nil {
		return "NULL"
	}
	return "\"" + *s + "\""
}

// TestFixturesLoad checks the synthetic dataset loads and has the shape that
// internal/testutil/fixtures.go documents.
func TestFixturesLoad(t *testing.T) {
	ctx := context.Background()
	pool := testutil.FixtureDatabase(t).Pool(t)

	var parcels, active, primary int
	err := pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM core.parcels),
		(SELECT count(*) FROM core.parcels WHERE NOT is_voided),
		(SELECT count(*) FROM core.parcel_improvements pi JOIN core.parcels p USING (parcel_id) WHERE pi.is_primary AND NOT p.is_voided)`).Scan(&parcels, &active, &primary)
	if err != nil {
		t.Fatal(err)
	}
	if parcels != testutil.ParcelCount || active != testutil.ActiveParcelCount || primary != testutil.ImprovedParcels {
		t.Errorf("got %d parcels, %d active, %d primary improvements; want %d, %d, %d",
			parcels, active, primary, testutil.ParcelCount, testutil.ActiveParcelCount, testutil.ImprovedParcels)
	}

	// search_path is set per database, so unqualified names resolve.
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM parcel_attributes").Scan(&n); err != nil {
		t.Fatalf("unqualified table name: %v", err)
	}

	tile := testutil.FixtureTile
	var size int
	if err := pool.QueryRow(ctx, "SELECT length(core.get_parcel_tiles($1, $2, $3))", tile.Z, tile.X, tile.Y).Scan(&size); err != nil {
		t.Fatal(err)
	}
	if size == 0 {
		t.Error("fixture tile is empty")
	}
}

// TestBathroomsRenameKeepsValues applies the migrations up to the full and
// half bathrooms change over a row with bathrooms set, then the rest, and
// checks the count survives as full_bathrooms (docs/full-half-bathrooms/spec.md).
// A migration that dropped and re-added the column would lose it.
func TestBathroomsRenameKeepsValues(t *testing.T) {
	ctx := context.Background()
	db := testutil.EmptyDatabase(t)
	conn, err := pgx.Connect(ctx, db.URL())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)

	files, err := testutil.MigrationFiles()
	if err != nil {
		t.Fatal(err)
	}
	rename := -1
	for i, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(sql), `RENAME COLUMN "bathrooms" TO "full_bathrooms"`) {
			rename = i
			break
		}
	}
	if rename < 0 {
		t.Fatal("no migration renames bathrooms to full_bathrooms")
	}
	if err := testutil.ExecFiles(ctx, conn, files[:rename]...); err != nil {
		t.Fatal(err)
	}

	// Only the column matters here, so skip the foreign keys rather than
	// building the improvement, address and type rows they point to.
	if _, err := conn.Exec(ctx, `SET session_replication_role = replica`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO core.improvement_attributes
		(improvement_id, address_id, improvement_type_id, bathrooms, legal_valid_range, trace_id)
		VALUES (1, 1, 1, 3, tstzrange('2020-01-01 00:00:00+00', NULL), 'test')`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `SET session_replication_role = DEFAULT`); err != nil {
		t.Fatal(err)
	}

	if err := testutil.ExecFiles(ctx, conn, files[rename:]...); err != nil {
		t.Fatal(err)
	}
	var full, half *int32
	if err := conn.QueryRow(ctx, `SELECT full_bathrooms, half_bathrooms FROM core.improvement_attributes`).Scan(&full, &half); err != nil {
		t.Fatal(err)
	}
	if full == nil || *full != 3 || half != nil {
		t.Errorf("after the migration: full_bathrooms = %v, half_bathrooms = %v; want 3 and null", deref(full), deref(half))
	}
}

func deref(p *int32) any {
	if p == nil {
		return nil
	}
	return *p
}
