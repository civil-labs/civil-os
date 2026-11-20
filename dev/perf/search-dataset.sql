-- Performance dataset for address search (docs/parcel-search/spec.md,
-- "Performance check"; docs/parcel-search/plan.md, step 2.9).
--
-- Adds 500,000 visible parcels, each with its own address, to a database that
-- already has the synthetic fixtures (database/fixtures/synthetic.sql). Every
-- value is derived from the row number with hashint4, so the dataset is the
-- same on every run.
--
-- Addresses are '<house> <street> <suffix>, <locality> ZZ<5 digits>'. Street
-- and locality names are made by joining invented syllables (20 x 20 streets,
-- 6 x 5 localities), so no real place name appears.
--
-- Run by services/database-reader/search_perf_test.go (`mise run
-- perf:search`). Not for production or compose databases.

BEGIN;
SET LOCAL search_path = core, geo, extensions, public;

-- pg_temp.pick(i, salt, n) is a deterministic index in 1..n.
CREATE FUNCTION pg_temp.pick(i int, salt int, n int) RETURNS int
  LANGUAGE sql IMMUTABLE AS $$ SELECT 1 + ((hashint4(i * 31 + salt)::bigint & 2147483647) % n)::int $$;

CREATE TEMP TABLE perf_rows ON COMMIT DROP AS
WITH words AS (
  SELECT
    ARRAY['Zav', 'Quil', 'Morv', 'Tesk', 'Plon', 'Vard', 'Krel', 'Oss', 'Brin', 'Gav',
          'Hult', 'Jor', 'Lisk', 'Nemb', 'Prav', 'Rusk', 'Sorv', 'Tamb', 'Ulv', 'Wenk'] AS head,
    ARRAY['ack', 'ell', 'ion', 'ory', 'esk', 'ane', 'ith', 'urn', 'alm', 'ost',
          'ery', 'inge', 'obb', 'ume', 'arl', 'eth', 'ix', 'onde', 'uff', 'ay'] AS tail,
    ARRAY['Way', 'Road', 'Lane', 'Court', 'Row', 'Loop'] AS suffix,
    ARRAY['Zav', 'Quil', 'Morv', 'Tesk', 'Plon', 'Vard'] AS town_head,
    ARRAY['ford', 'mere', 'dale', 'wick', 'holt'] AS town_tail
)
SELECT i,
       ('b0000000-0000-4000-8000-' || lpad(i::text, 12, '0'))::uuid AS parcel_uuid,
       ('b1000000-0000-4000-8000-' || lpad(i::text, 12, '0'))::uuid AS address_uuid,
       pg_temp.pick(i, 1, 9999) AS house,
       head[pg_temp.pick(i, 2, 20)] || tail[pg_temp.pick(i, 3, 20)] AS street,
       suffix[pg_temp.pick(i, 4, 6)] AS suffix,
       town_head[pg_temp.pick(i, 5, 6)] || town_tail[pg_temp.pick(i, 6, 5)] AS locality,
       'ZZ' || lpad((pg_temp.pick(i, 7, 100000) - 1)::text, 5, '0') AS postal_code
FROM words, generate_series(1, 500000) AS i;

INSERT INTO core.addresses (public_id, legacy_id, trace_id)
SELECT address_uuid, 'PERF-ADDR-' || i, 'perf' FROM perf_rows;

INSERT INTO core.address_attributes (address_id, country_id, administrative_area_id, locality, postal_code,
       address_line_1, formatted_address, legal_valid_range, trace_id)
SELECT a.address_id, c.country_id, aa.administrative_area_id, r.locality, r.postal_code,
       r.house || ' ' || r.street || ' ' || r.suffix,
       r.house || ' ' || r.street || ' ' || r.suffix || ', ' || r.locality || ' ' || r.postal_code,
       tstzrange('2020-01-01 00:00:00+00', NULL), 'perf'
FROM perf_rows r
JOIN core.addresses a ON a.public_id = r.address_uuid
CROSS JOIN core.countries c
CROSS JOIN core.administrative_areas aa;

INSERT INTO core.parcels (public_id, legacy_id, trace_id)
SELECT parcel_uuid, 'PERF-PARCEL-' || i, 'perf' FROM perf_rows;

-- A 1,000-column grid of 0.0003-degree squares (about 25 m x 33 m), away
-- from the fixtures' grid at (-140, -40).
INSERT INTO geo.parcel_geometry (parcel_id, feature_id, geom_web, legal_valid_range, trace_id)
SELECT p.parcel_id, 2000000 + r.i,
       ST_Multi(ST_MakeEnvelope(
         -139.0 + ((r.i - 1) % 1000) * 0.0003,          -45.0 + ((r.i - 1) / 1000) * 0.0003,
         -139.0 + ((r.i - 1) % 1000) * 0.0003 + 0.0003, -45.0 + ((r.i - 1) / 1000) * 0.0003 + 0.0003, 4326)),
       tstzrange('2020-01-01 00:00:00+00', NULL), 'perf'
FROM perf_rows r JOIN core.parcels p ON p.public_id = r.parcel_uuid;

INSERT INTO core.parcel_attributes (parcel_id, address_id, land_use_id, land_area_sq_m, frontage_m, depth_m,
       legal_valid_range, trace_id)
SELECT p.parcel_id, a.address_id, lu.land_use_id, 825, 25, 33,
       tstzrange('2020-01-01 00:00:00+00', NULL), 'perf'
FROM perf_rows r
JOIN core.parcels p ON p.public_id = r.parcel_uuid
JOIN core.addresses a ON a.public_id = r.address_uuid
CROSS JOIN core.land_uses lu
WHERE lu.public_id = '11000000-0000-4000-8000-000000000001';

COMMIT;

ANALYZE;
