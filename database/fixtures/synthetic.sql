-- Synthetic civil-os data for the local compose stack and integration tests.
--
-- Everything here is invented: the "jurisdiction" is a 4 x 5 grid of 30 m
-- parcels in open ocean (around 140.00 W, 40.00 S), names are obviously
-- fictional, and every public ID follows a fixed pattern so tests can
-- reference rows directly (see internal/testutil/fixtures.go):
--
--   <prefix>-0000-4000-8000-<n, 12 digits>
--   10000000 land use types       11000000 land uses
--   20000000 addresses            30000000 parties
--   40000000 zoning               41000000 affordance types   42000000 parcel affordances
--   50000000 improvement types    51000000 improvement conditions
--   52000000 improvement qualities 53000000 improvements
--   60000000 valuations           70000000 neighborhood defs  71000000 neighborhoods
--   80000000 countries            81000000 administrative areas
--   a0000000 parcels (1..20; feature_id = 1000 + n)
--
-- Shape of the data (tests rely on it):
--   * Parcels 1-19 are active; parcel 20 is voided.
--   * Parcel n: land use ((n-1) % 3) + 1, land area 400 + 10n m2,
--     frontage 15 + 0.5n m, depth 25 + n m, properties {"fixture": true, "n": n}.
--   * Parcel 1 also has a historical attributes row valid 2020-01-01..2024-01-01
--     with land area 350 m2; its current row starts 2024-01-01.
--   * Parcels 1-15 each have one primary improvement (improvement n); parcel 3
--     also has a secondary improvement (improvement 16). Voided parcel 20 has
--     primary improvement 17, so improvement reads must hide it too.
--   * Zoning: parcels 1-12 are R-1, 13-20 are C-1.
--   * Neighborhoods (legal definition 1): parcels 1-10 North, 11-20 South.
--   * Valuation 1 (2025 roll): parcel market value 100000 + 5000n, assessed 90%.
--   * Address 21 is attached to no parcel, for update tests.
--
-- Run after all migrations. Not for production databases.

BEGIN;

-- Mirror the production database-level search_path (set by cluster
-- infrastructure), which the reader's unqualified table names rely on.
DO $$
BEGIN
  EXECUTE format('ALTER DATABASE %I SET search_path = core, geo, extensions, public', current_database());
END
$$;
SET LOCAL search_path = core, geo, extensions, public;

CREATE TEMP TABLE fx(n int) ON COMMIT DROP;
INSERT INTO fx SELECT generate_series(1, 21);

CREATE FUNCTION pg_temp.fid(prefix text, n int) RETURNS uuid
  LANGUAGE sql IMMUTABLE AS $$ SELECT (prefix || '-0000-4000-8000-' || lpad(n::text, 12, '0'))::uuid $$;

-- Reference data ---------------------------------------------------------------

INSERT INTO core.countries (public_id, code, name, trace_id)
VALUES (pg_temp.fid('80000000', 1), 'ZZ', 'Testland', 'fixture');

INSERT INTO core.administrative_areas (public_id, code, name, trace_id)
VALUES (pg_temp.fid('81000000', 1), 'ZZ-FX', 'Fixture Province', 'fixture');

INSERT INTO core.land_use_types (public_id, code, name, description, trace_id) VALUES
  (pg_temp.fid('10000000', 1), 'RES', 'Residential', 'Fixture residential uses', 'fixture'),
  (pg_temp.fid('10000000', 2), 'COM', 'Commercial',  'Fixture commercial uses',  'fixture'),
  (pg_temp.fid('10000000', 3), 'VAC', 'Vacant',      'Fixture vacant land',      'fixture');

INSERT INTO core.land_uses (public_id, land_use_type_id, code, name, description, trace_id)
SELECT pg_temp.fid('11000000', v.k), t.land_use_type_id, v.code, v.name, v.name || ' (fixture)', 'fixture'
FROM (VALUES (1, 'R100', 'Single family'), (2, 'C200', 'Retail'), (3, 'V000', 'Vacant lot')) v(k, code, name)
JOIN core.land_use_types t ON t.public_id = pg_temp.fid('10000000', v.k);

INSERT INTO core.improvement_types (public_id, code, name, description, trace_id) VALUES
  (pg_temp.fid('50000000', 1), 'SFD', 'Single family dwelling', 'Fixture dwelling',        'fixture'),
  (pg_temp.fid('50000000', 2), 'RTL', 'Retail building',        'Fixture retail building', 'fixture');

INSERT INTO core.improvement_conditions (public_id, trace_id)
SELECT pg_temp.fid('51000000', k), 'fixture' FROM generate_series(1, 3) k;

INSERT INTO core.improvement_condition_attributes (improvement_condition_id, name, depreciation_modifier, legal_valid_range, trace_id)
SELECT c.improvement_condition_id, v.name, v.modifier, tstzrange('2020-01-01 00:00:00+00', NULL), 'fixture'
FROM (VALUES (1, 'Good', 0.9000), (2, 'Average', 0.8000), (3, 'Poor', 0.6000)) v(k, name, modifier)
JOIN core.improvement_conditions c ON c.public_id = pg_temp.fid('51000000', v.k);

INSERT INTO core.improvement_qualities (public_id, trace_id)
VALUES (pg_temp.fid('52000000', 1), 'fixture');

-- Affordance type 1 must be the zoning type (check: zoning_id IS NOT NULL OR affordance_type_id != 1).
INSERT INTO core.affordance_types (public_id, name, trace_id) VALUES
  (pg_temp.fid('41000000', 1), 'Base zoning', 'fixture'),
  (pg_temp.fid('41000000', 2), 'Overlay',     'fixture');

INSERT INTO core.zoning (public_id, trace_id) VALUES
  (pg_temp.fid('40000000', 1), 'fixture'),
  (pg_temp.fid('40000000', 2), 'fixture');

INSERT INTO core.zoning_attributes (zoning_id, name, code, max_far, min_lot_size_sq_m, max_height_m, max_dwelling_units_per_hectare, max_lot_coverage_pct, legal_valid_range, trace_id)
SELECT z.zoning_id, v.name, v.code, v.far, v.lot, v.height, v.du, v.cov, tstzrange('2020-01-01 00:00:00+00', NULL), 'fixture'
FROM (VALUES (1, 'Fixture Residential', 'R-1', 0.50, 300.00, 10.00, 25.00, 0.4000),
             (2, 'Fixture Commercial',  'C-1', 2.00, 200.00, 20.00, NULL,  0.8000))
     v(k, name, code, far, lot, height, du, cov)
JOIN core.zoning z ON z.public_id = pg_temp.fid('40000000', v.k);

INSERT INTO core.valuations (public_id, legacy_id, valuation_date, trace_id)
VALUES (pg_temp.fid('60000000', 1), 'FX-2025', '2025-01-01 00:00:00+00', 'fixture');

INSERT INTO core.neighborhood_definitions (public_id, name, is_legal, trace_id)
VALUES (pg_temp.fid('70000000', 1), 'Fixture districts', true, 'fixture');

INSERT INTO core.neighborhoods (public_id, name, trace_id) VALUES
  (pg_temp.fid('71000000', 1), 'North Fixture', 'fixture'),
  (pg_temp.fid('71000000', 2), 'South Fixture', 'fixture');

-- Addresses and parties ----------------------------------------------------------

INSERT INTO core.addresses (public_id, legacy_id, trace_id)
SELECT pg_temp.fid('20000000', n), 'FX-ADDR-' || n, 'fixture' FROM fx;

INSERT INTO core.address_attributes (address_id, country_id, administrative_area_id, locality, postal_code, address_line_1, formatted_address, coordinates, legal_valid_range, trace_id)
SELECT a.address_id, c.country_id, aa.administrative_area_id, 'Testville', 'ZZ000',
       fx.n || ' Fixture Way', fx.n || ' Fixture Way, Testville ZZ000',
       extensions.ST_SetSRID(extensions.ST_MakePoint(-140.0 + ((fx.n - 1) % 4) * 0.0004 + 0.0002, -40.0 + ((fx.n - 1) / 4) * 0.0004 + 0.0002), 4326),
       tstzrange('2020-01-01 00:00:00+00', NULL), 'fixture'
FROM fx
JOIN core.addresses a ON a.public_id = pg_temp.fid('20000000', fx.n)
CROSS JOIN core.countries c
CROSS JOIN core.administrative_areas aa;

INSERT INTO core.parties (public_id, trace_id)
SELECT pg_temp.fid('30000000', k), 'fixture' FROM generate_series(1, 10) k;

INSERT INTO core.party_attributes (party_id, name, address_id, legal_valid_range, trace_id)
SELECT p.party_id, 'Fixture Owner ' || k, a.address_id, tstzrange('2020-01-01 00:00:00+00', NULL), 'fixture'
FROM generate_series(1, 10) k
JOIN core.parties p ON p.public_id = pg_temp.fid('30000000', k)
JOIN core.addresses a ON a.public_id = pg_temp.fid('20000000', k);

-- Parcels -------------------------------------------------------------------------

INSERT INTO core.parcels (public_id, legacy_id, trace_id)
SELECT pg_temp.fid('a0000000', n), 'FX-PARCEL-' || n, 'fixture' FROM fx WHERE n <= 20;

UPDATE core.parcels SET is_voided = true, voided_at = '2025-06-01 00:00:00+00'
WHERE public_id = pg_temp.fid('a0000000', 20);

-- 4 columns x 5 rows of 0.0004-degree squares (about 34 m x 44 m at 40 S).
INSERT INTO geo.parcel_geometry (parcel_id, feature_id, geom_web, legal_valid_range, trace_id)
SELECT p.parcel_id, 1000 + fx.n,
       extensions.ST_Multi(extensions.ST_MakeEnvelope(
         -140.0 + ((fx.n - 1) % 4) * 0.0004,       -40.0 + ((fx.n - 1) / 4) * 0.0004,
         -140.0 + ((fx.n - 1) % 4) * 0.0004 + 0.0004, -40.0 + ((fx.n - 1) / 4) * 0.0004 + 0.0004, 4326)),
       tstzrange('2020-01-01 00:00:00+00', NULL), 'fixture'
FROM fx JOIN core.parcels p ON p.public_id = pg_temp.fid('a0000000', fx.n);

INSERT INTO core.parcel_attributes (parcel_id, address_id, land_use_id, land_area_sq_m, frontage_m, depth_m, properties, legal_valid_range, trace_id)
SELECT p.parcel_id, a.address_id, lu.land_use_id,
       400 + 10 * fx.n, 15 + 0.5 * fx.n, 25 + fx.n,
       jsonb_build_object('fixture', true, 'n', fx.n),
       CASE WHEN fx.n = 1 THEN tstzrange('2024-01-01 00:00:00+00', NULL)
            ELSE tstzrange('2020-01-01 00:00:00+00', NULL) END,
       'fixture'
FROM fx
JOIN core.parcels p ON p.public_id = pg_temp.fid('a0000000', fx.n)
JOIN core.addresses a ON a.public_id = pg_temp.fid('20000000', fx.n)
JOIN core.land_uses lu ON lu.public_id = pg_temp.fid('11000000', ((fx.n - 1) % 3) + 1);

-- Historical attributes for parcel 1 (legal_as_of before 2024 sees 350 m2).
INSERT INTO core.parcel_attributes (parcel_id, address_id, land_use_id, land_area_sq_m, frontage_m, depth_m, properties, legal_valid_range, trace_id)
SELECT p.parcel_id, a.address_id, lu.land_use_id, 350, 15.5, 26, '{"fixture": true, "n": 1, "historical": true}',
       tstzrange('2020-01-01 00:00:00+00', '2024-01-01 00:00:00+00'), 'fixture'
FROM core.parcels p, core.addresses a, core.land_uses lu
WHERE p.public_id = pg_temp.fid('a0000000', 1)
  AND a.public_id = pg_temp.fid('20000000', 1)
  AND lu.public_id = pg_temp.fid('11000000', 1);

-- Parcel 2 is split 50/50 between owners 2 and 3; every other parcel has one owner.
INSERT INTO core.parcel_parties (parcel_id, party_id, ownership_share, legal_valid_range, trace_id)
SELECT p.parcel_id, pt.party_id, v.share, tstzrange('2020-01-01 00:00:00+00', NULL), 'fixture'
FROM (SELECT n, ((n - 1) % 10) + 1 AS owner, CASE WHEN n = 2 THEN 0.5000 ELSE 1.0000 END AS share FROM fx WHERE n <= 20
      UNION ALL SELECT 2, 3, 0.5000) v
JOIN core.parcels p ON p.public_id = pg_temp.fid('a0000000', v.n)
JOIN core.parties pt ON pt.public_id = pg_temp.fid('30000000', v.owner);

INSERT INTO core.parcel_affordances (public_id, parcel_id, zoning_id, affordance_type_id, source, max_far, min_lot_size_sq_m, max_height_m, max_dwelling_units_per_hectare, max_lot_coverage_pct, legal_valid_range, trace_id)
SELECT pg_temp.fid('42000000', fx.n), p.parcel_id, z.zoning_id, t.affordance_type_id, 'fixture zoning',
       za.max_far, za.min_lot_size_sq_m, za.max_height_m, za.max_dwelling_units_per_hectare, za.max_lot_coverage_pct,
       tstzrange('2020-01-01 00:00:00+00', NULL), 'fixture'
FROM fx
JOIN core.parcels p ON p.public_id = pg_temp.fid('a0000000', fx.n)
JOIN core.zoning z ON z.public_id = pg_temp.fid('40000000', CASE WHEN fx.n <= 12 THEN 1 ELSE 2 END)
JOIN core.zoning_attributes za ON za.zoning_id = z.zoning_id
JOIN core.affordance_types t ON t.public_id = pg_temp.fid('41000000', 1);

INSERT INTO core.parcel_neighborhood_definitions (parcel_id, neighborhood_id, neighborhood_definition_id, is_legal, legal_valid_range, trace_id)
SELECT p.parcel_id, nb.neighborhood_id, d.neighborhood_definition_id, true, tstzrange('2020-01-01 00:00:00+00', NULL), 'fixture'
FROM fx
JOIN core.parcels p ON p.public_id = pg_temp.fid('a0000000', fx.n)
JOIN core.neighborhoods nb ON nb.public_id = pg_temp.fid('71000000', CASE WHEN fx.n <= 10 THEN 1 ELSE 2 END)
CROSS JOIN core.neighborhood_definitions d;

INSERT INTO core.parcel_valuations (parcel_id, valuation_id, market_value, assessed_value, legal_valid_range, trace_id)
SELECT p.parcel_id, v.valuation_id, 100000 + 5000 * fx.n, (100000 + 5000 * fx.n) * 0.9, tstzrange('2020-01-01 00:00:00+00', NULL), 'fixture'
FROM fx
JOIN core.parcels p ON p.public_id = pg_temp.fid('a0000000', fx.n)
CROSS JOIN core.valuations v;

-- Improvements ----------------------------------------------------------------------
-- Improvement k (1..15) sits on parcel k; improvement 16 is a secondary building on parcel 3;
-- improvement 17 is the primary building on voided parcel 20.

INSERT INTO core.improvements (public_id, legacy_id, trace_id)
SELECT pg_temp.fid('53000000', k), 'FX-IMP-' || k, 'fixture' FROM generate_series(1, 17) k;

INSERT INTO core.improvement_attributes (improvement_id, address_id, improvement_type_id, improvement_condition_id, improvement_quality_id, area_sq_m, bathrooms, bedrooms, year_built, effective_year_built, units, legal_valid_range, trace_id)
SELECT i.improvement_id, a.address_id, it.improvement_type_id, c.improvement_condition_id, q.improvement_quality_id,
       CASE WHEN v.k = 16 THEN 30 ELSE 120 + 5 * v.k END,
       CASE WHEN v.k = 16 THEN 0 ELSE 2 END,
       CASE WHEN v.k = 16 THEN 0 ELSE 3 END,
       1980 + v.k, 1990 + v.k, 1,
       tstzrange('2020-01-01 00:00:00+00', NULL), 'fixture'
FROM (SELECT k, CASE k WHEN 16 THEN 3 WHEN 17 THEN 20 ELSE k END AS parcel_n FROM generate_series(1, 17) k) v
JOIN core.improvements i ON i.public_id = pg_temp.fid('53000000', v.k)
JOIN core.addresses a ON a.public_id = pg_temp.fid('20000000', v.parcel_n)
JOIN core.improvement_types it ON it.public_id = pg_temp.fid('50000000', CASE WHEN v.parcel_n <= 12 THEN 1 ELSE 2 END)
JOIN core.improvement_conditions c ON c.public_id = pg_temp.fid('51000000', ((v.k - 1) % 3) + 1)
CROSS JOIN core.improvement_qualities q;

INSERT INTO core.parcel_improvements (parcel_id, improvement_id, is_primary, legal_valid_range, trace_id)
SELECT p.parcel_id, i.improvement_id, v.k <> 16, tstzrange('2020-01-01 00:00:00+00', NULL), 'fixture'
FROM (SELECT k, CASE k WHEN 16 THEN 3 WHEN 17 THEN 20 ELSE k END AS parcel_n FROM generate_series(1, 17) k) v
JOIN core.parcels p ON p.public_id = pg_temp.fid('a0000000', v.parcel_n)
JOIN core.improvements i ON i.public_id = pg_temp.fid('53000000', v.k);

INSERT INTO core.improvement_valuations (improvement_id, valuation_id, market_value, assessed_value, legal_valid_range, trace_id)
SELECT i.improvement_id, v.valuation_id, 80000 + 2000 * k, (80000 + 2000 * k) * 0.9, tstzrange('2020-01-01 00:00:00+00', NULL), 'fixture'
FROM generate_series(1, 17) k
JOIN core.improvements i ON i.public_id = pg_temp.fid('53000000', k)
CROSS JOIN core.valuations v;

COMMIT;
