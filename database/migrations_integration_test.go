//go:build integration

package database_test

import (
	"context"
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

	for _, fn := range []string{"core.get_parcel_tiles", "core.get_primary_improvements"} {
		var ok bool
		if err := conn.QueryRow(ctx, "SELECT to_regproc($1) IS NOT NULL", fn).Scan(&ok); err != nil || !ok {
			t.Errorf("function %s missing after migrations (err=%v)", fn, err)
		}
	}
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
		(SELECT count(*) FROM core.parcel_improvements WHERE is_primary)`).Scan(&parcels, &active, &primary)
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
