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
--   90000000 real property transfers
--   91000000 transfer code types  92000000 transfer codes
--   a0000000 parcels (1..21; feature_id = 1000 + n)
--
-- Shape of the data (tests rely on it):
--   * Parcels 1-19 and 21 are active; parcel 20 is voided. Parcel 21 has no
--     geometry, so parcel reads never return it (address 2, land use 1,
--     owned by party 2 at 1.0; no improvement, affordance, neighborhood or
--     valuation).
--   * Parcel n: land use ((n-1) % 3) + 1, land area 400 + 10n m2,
--     frontage 15 + 0.5n m, depth 25 + n m, properties {"fixture": true, "n": n}.
--   * Parcel 1 also has a historical attributes row valid 2020-01-01..2024-01-01
--     with land area 350 m2; its current row starts 2024-01-01.
--   * Parcels 1-15 each have one primary improvement (improvement n); parcel 3
--     also has a secondary improvement (improvement 16). Voided parcel 20 has
--     primary improvement 17, so improvement reads must hide it too.
--   * Improvements (docs/improvement-read-api/spec.md):
--       qualities  1 'Fixture Standard' (1.0), used by improvements 1-17;
--                  2 'Fixture Premium' (1.2), used by none
--       improvement 4   condition 3, 100 m2, 1 full bath, 2 bedrooms, effective
--                       year built 1984 until 2023-06-01; current values from then
--       improvement 18  a non-primary shed at address 12: 20 m2, year built 2001,
--                       no condition, quality, counts or valuation; on parcel 11
--                       until 2024-01-01, on parcel 12 from then
--       improvement_parties: 2 -> parties 2 and 3 (0.5 each); 3 -> party 3;
--                       16 -> party 5 and voided party 11 (0.5 each); 6 -> party 10
--                       until 2023-09-01, party 6 from then; 17 -> party 10.
--                       Every other improvement has none.
--   * Zoning: parcels 1-12 are R-1, 13-20 are C-1.
--   * Neighborhoods (docs/neighborhoods-api-fixes/spec.md):
--       definition 1 'Fixture districts' (legal): parcels 1-10 North (1) and
--         11-20 South (2) from 2020-01-01, except parcel 10: South until
--         2023-01-01, North from then
--       definition 2 'Fixture market areas' (not legal): parcels 1-5 ->
--         'Fixture Market Area' (3), null range; parcel 6 -> 3 for
--         2020-01-01..2023-01-01; parcel 7 -> North (1), null range
--       neighborhood 4 'Unused Fixture' has no assignments
--   * Valuation 1 (2025 roll): parcel market value 100000 + 5000n, assessed 90%.
--   * Address 21 is attached to no parcel, for update tests.
--   * History (docs/address-party-read-api/spec.md):
--       party 8    'Fixture Owner 8 Formerly', mailing address 18, until 2023-01-01;
--                  'Fixture Owner 8', address 8, from then
--       address 5  '5 Old Fixture Way', no coordinates, until 2023-01-01
--       parcel 9   west half of its square, feature 1109, until 2024-01-01;
--                  full square, feature 1009, from then
--   * Party 11 is voided (mailing address 11), but still has a parcel_parties
--     row on parcel 4 at 0.5, so reads must hide it.
--   * Real property transfers 1-7 (docs/real-property-transfers-api/spec.md):
--       1  2021-03-01  parcel 5: party 9 -> 4; codes 1 and 3
--       2  2022-06-15  parcel 2: party 1 -> 2 and 3 (0.5 each)
--       3  2023-09-01  parcels 6 and 7, improvements 6 and 7: party 10 -> 6 (n = 6), 7 (n = 7); code 2
--       4  2024-02-01  voided parcel 20 and improvement 17 only: never visible
--       5  2024-05-01  parcels 19 and voided 20: parties 1 and 2 (0.5 each) -> 9 (19); 1 -> 10 (20)
--       6  2025-01-10  parcel 5: party 4 -> 5; no amount, no recording
--       7  2022-01-15  parcel 8: party 3 as grantor, no grantee recorded
--     Transfers other than 6 have deed book FX-B<n>, page <n> and URI
--     https://recordings.example.invalid/FX-<n>. The latest grantee of each
--     parcel is its current owner (except parcel 8), but parcel_parties
--     ranges all start 2020-01-01: ownership history isn't modelled, so the
--     dates deliberately don't match the transfers.
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

INSERT INTO core.improvement_qualities (public_id, trace_id) VALUES
  (pg_temp.fid('52000000', 1), 'fixture'),
  (pg_temp.fid('52000000', 2), 'fixture');

-- Quality 2 is used by no improvement.
INSERT INTO core.improvement_quality_attributes (improvement_quality_id, name, quality_adjustment_modifier, legal_valid_range, trace_id)
SELECT q.improvement_quality_id, v.name, v.modifier, tstzrange('2020-01-01 00:00:00+00', NULL), 'fixture'
FROM (VALUES (1, 'Fixture Standard', 1.0), (2, 'Fixture Premium', 1.2)) v(k, name, modifier)
JOIN core.improvement_qualities q ON q.public_id = pg_temp.fid('52000000', v.k);

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

INSERT INTO core.neighborhood_definitions (public_id, name, is_legal, trace_id) VALUES
  (pg_temp.fid('70000000', 1), 'Fixture districts', true, 'fixture'),
  (pg_temp.fid('70000000', 2), 'Fixture market areas', false, 'fixture');

INSERT INTO core.neighborhoods (public_id, name, trace_id) VALUES
  (pg_temp.fid('71000000', 1), 'North Fixture', 'fixture'),
  (pg_temp.fid('71000000', 2), 'South Fixture', 'fixture'),
  (pg_temp.fid('71000000', 3), 'Fixture Market Area', 'fixture'),
  (pg_temp.fid('71000000', 4), 'Unused Fixture', 'fixture');

-- Addresses and parties ----------------------------------------------------------

INSERT INTO core.addresses (public_id, legacy_id, trace_id)
SELECT pg_temp.fid('20000000', n), 'FX-ADDR-' || n, 'fixture' FROM fx;

INSERT INTO core.address_attributes (address_id, country_id, administrative_area_id, locality, postal_code, address_line_1, formatted_address, coordinates, legal_valid_range, trace_id)
SELECT a.address_id, c.country_id, aa.administrative_area_id, 'Testville', 'ZZ000',
       fx.n || ' Fixture Way', fx.n || ' Fixture Way, Testville ZZ000',
       extensions.ST_SetSRID(extensions.ST_MakePoint(-140.0 + ((fx.n - 1) % 4) * 0.0004 + 0.0002, -40.0 + ((fx.n - 1) / 4) * 0.0004 + 0.0002), 4326),
       CASE WHEN fx.n = 5 THEN tstzrange('2023-01-01 00:00:00+00', NULL)
            ELSE tstzrange('2020-01-01 00:00:00+00', NULL) END,
       'fixture'
FROM fx
JOIN core.addresses a ON a.public_id = pg_temp.fid('20000000', fx.n)
CROSS JOIN core.countries c
CROSS JOIN core.administrative_areas aa;

-- Address 5's earlier version: another line and no coordinates.
INSERT INTO core.address_attributes (address_id, country_id, administrative_area_id, locality, postal_code, address_line_1, formatted_address, coordinates, legal_valid_range, trace_id)
SELECT a.address_id, c.country_id, aa.administrative_area_id, 'Testville', 'ZZ000',
       '5 Old Fixture Way', '5 Old Fixture Way, Testville ZZ000', NULL,
       tstzrange('2020-01-01 00:00:00+00', '2023-01-01 00:00:00+00'), 'fixture'
FROM core.addresses a, core.countries c, core.administrative_areas aa
WHERE a.public_id = pg_temp.fid('20000000', 5);

INSERT INTO core.parties (public_id, trace_id)
SELECT pg_temp.fid('30000000', k), 'fixture' FROM generate_series(1, 11) k;

UPDATE core.parties SET is_voided = true, voided_at = '2025-06-01 00:00:00+00'
WHERE public_id = pg_temp.fid('30000000', 11);

INSERT INTO core.party_attributes (party_id, name, address_id, legal_valid_range, trace_id)
SELECT p.party_id, 'Fixture Owner ' || k, a.address_id,
       CASE WHEN k = 8 THEN tstzrange('2023-01-01 00:00:00+00', NULL)
            ELSE tstzrange('2020-01-01 00:00:00+00', NULL) END,
       'fixture'
FROM generate_series(1, 11) k
JOIN core.parties p ON p.public_id = pg_temp.fid('30000000', k)
JOIN core.addresses a ON a.public_id = pg_temp.fid('20000000', k);

-- Party 8's earlier name and mailing address.
INSERT INTO core.party_attributes (party_id, name, address_id, legal_valid_range, trace_id)
SELECT p.party_id, 'Fixture Owner 8 Formerly', a.address_id,
       tstzrange('2020-01-01 00:00:00+00', '2023-01-01 00:00:00+00'), 'fixture'
FROM core.parties p, core.addresses a
WHERE p.public_id = pg_temp.fid('30000000', 8) AND a.public_id = pg_temp.fid('20000000', 18);

-- Parcels -------------------------------------------------------------------------

INSERT INTO core.parcels (public_id, legacy_id, trace_id)
SELECT pg_temp.fid('a0000000', n), 'FX-PARCEL-' || n, 'fixture' FROM fx WHERE n <= 21;

UPDATE core.parcels SET is_voided = true, voided_at = '2025-06-01 00:00:00+00'
WHERE public_id = pg_temp.fid('a0000000', 20);

-- 4 columns x 5 rows of 0.0004-degree squares (about 34 m x 44 m at 40 S).
INSERT INTO geo.parcel_geometry (parcel_id, feature_id, geom_web, legal_valid_range, trace_id)
SELECT p.parcel_id, 1000 + fx.n,
       extensions.ST_Multi(extensions.ST_MakeEnvelope(
         -140.0 + ((fx.n - 1) % 4) * 0.0004,       -40.0 + ((fx.n - 1) / 4) * 0.0004,
         -140.0 + ((fx.n - 1) % 4) * 0.0004 + 0.0004, -40.0 + ((fx.n - 1) / 4) * 0.0004 + 0.0004, 4326)),
       CASE WHEN fx.n = 9 THEN tstzrange('2024-01-01 00:00:00+00', NULL)
            ELSE tstzrange('2020-01-01 00:00:00+00', NULL) END,
       'fixture'
FROM fx JOIN core.parcels p ON p.public_id = pg_temp.fid('a0000000', fx.n)
WHERE fx.n <= 20; -- parcel 21 has no geometry

-- Parcel 9's earlier geometry: the west half of its square, as feature 1109.
INSERT INTO geo.parcel_geometry (parcel_id, feature_id, geom_web, legal_valid_range, trace_id)
SELECT p.parcel_id, 1109,
       extensions.ST_Multi(extensions.ST_MakeEnvelope(-140.0, -39.9992, -139.9998, -39.9988, 4326)),
       tstzrange('2020-01-01 00:00:00+00', '2024-01-01 00:00:00+00'), 'fixture'
FROM core.parcels p WHERE p.public_id = pg_temp.fid('a0000000', 9);

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
JOIN core.land_uses lu ON lu.public_id = pg_temp.fid('11000000', ((fx.n - 1) % 3) + 1)
WHERE fx.n <= 20;

-- Parcel 21: address 2, land use 1, no geometry.
INSERT INTO core.parcel_attributes (parcel_id, address_id, land_use_id, land_area_sq_m, frontage_m, depth_m, properties, legal_valid_range, trace_id)
SELECT p.parcel_id, a.address_id, lu.land_use_id, 610, 25.5, 46, '{"fixture": true, "n": 21}',
       tstzrange('2020-01-01 00:00:00+00', NULL), 'fixture'
FROM core.parcels p, core.addresses a, core.land_uses lu
WHERE p.public_id = pg_temp.fid('a0000000', 21)
  AND a.public_id = pg_temp.fid('20000000', 2)
  AND lu.public_id = pg_temp.fid('11000000', 1);

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
      UNION ALL SELECT 2, 3, 0.5000
      UNION ALL SELECT 21, 2, 1.0000   -- parcel 21 (no geometry)
      UNION ALL SELECT 4, 11, 0.5000   -- voided party 11
     ) v
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
JOIN core.affordance_types t ON t.public_id = pg_temp.fid('41000000', 1)
WHERE fx.n <= 20;

-- Legal definition 1. Parcel 10's North row starts 2023-01-01; before that it was South.
INSERT INTO core.parcel_neighborhood_definitions (parcel_id, neighborhood_id, neighborhood_definition_id, is_legal, legal_valid_range, trace_id)
SELECT p.parcel_id, nb.neighborhood_id, d.neighborhood_definition_id, true,
       CASE WHEN fx.n = 10 THEN tstzrange('2023-01-01 00:00:00+00', NULL)
            ELSE tstzrange('2020-01-01 00:00:00+00', NULL) END,
       'fixture'
FROM fx
JOIN core.parcels p ON p.public_id = pg_temp.fid('a0000000', fx.n)
JOIN core.neighborhoods nb ON nb.public_id = pg_temp.fid('71000000', CASE WHEN fx.n <= 10 THEN 1 ELSE 2 END)
JOIN core.neighborhood_definitions d ON d.public_id = pg_temp.fid('70000000', 1)
WHERE fx.n <= 20;

INSERT INTO core.parcel_neighborhood_definitions (parcel_id, neighborhood_id, neighborhood_definition_id, is_legal, legal_valid_range, trace_id)
SELECT p.parcel_id, nb.neighborhood_id, d.neighborhood_definition_id, true,
       tstzrange('2020-01-01 00:00:00+00', '2023-01-01 00:00:00+00'), 'fixture'
FROM core.parcels p
JOIN core.neighborhoods nb ON nb.public_id = pg_temp.fid('71000000', 2)
JOIN core.neighborhood_definitions d ON d.public_id = pg_temp.fid('70000000', 1)
WHERE p.public_id = pg_temp.fid('a0000000', 10);

-- Non-legal definition 2: parcels 1-5 and 7 with a null range, parcel 6 dated.
INSERT INTO core.parcel_neighborhood_definitions (parcel_id, neighborhood_id, neighborhood_definition_id, is_legal, legal_valid_range, trace_id)
SELECT p.parcel_id, nb.neighborhood_id, d.neighborhood_definition_id, false,
       CASE WHEN fx.n = 6 THEN tstzrange('2020-01-01 00:00:00+00', '2023-01-01 00:00:00+00') END,
       'fixture'
FROM fx
JOIN core.parcels p ON p.public_id = pg_temp.fid('a0000000', fx.n)
JOIN core.neighborhoods nb ON nb.public_id = pg_temp.fid('71000000', CASE WHEN fx.n = 7 THEN 1 ELSE 3 END)
JOIN core.neighborhood_definitions d ON d.public_id = pg_temp.fid('70000000', 2)
WHERE fx.n <= 7;

INSERT INTO core.parcel_valuations (parcel_id, valuation_id, market_value, assessed_value, legal_valid_range, trace_id)
SELECT p.parcel_id, v.valuation_id, 100000 + 5000 * fx.n, (100000 + 5000 * fx.n) * 0.9, tstzrange('2020-01-01 00:00:00+00', NULL), 'fixture'
FROM fx
JOIN core.parcels p ON p.public_id = pg_temp.fid('a0000000', fx.n)
CROSS JOIN core.valuations v
WHERE fx.n <= 20;

-- Improvements ----------------------------------------------------------------------
-- Improvement k (1..15) sits on parcel k; improvement 16 is a secondary building on parcel 3;
-- improvement 17 is the primary building on voided parcel 20; improvement 18 is
-- a shed that moves from parcel 11 to parcel 12 (its own inserts below).

INSERT INTO core.improvements (public_id, legacy_id, trace_id)
SELECT pg_temp.fid('53000000', k), 'FX-IMP-' || k, 'fixture' FROM generate_series(1, 18) k;

INSERT INTO core.improvement_attributes (improvement_id, address_id, improvement_type_id, improvement_condition_id, improvement_quality_id, area_sq_m, full_bathrooms, half_bathrooms, bedrooms, year_built, effective_year_built, units, legal_valid_range, trace_id)
SELECT i.improvement_id, a.address_id, it.improvement_type_id, c.improvement_condition_id, q.improvement_quality_id,
       CASE WHEN v.k = 16 THEN 30 ELSE 120 + 5 * v.k END,
       CASE WHEN v.k = 16 THEN 0 ELSE 2 END,
       -- Parcel 1's improvement has a half bath (2.5 baths in all); the others
       -- leave half_bathrooms unrecorded (docs/full-half-bathrooms/spec.md).
       CASE WHEN v.k = 1 THEN 1 END,
       CASE WHEN v.k = 16 THEN 0 ELSE 3 END,
       1980 + v.k, 1990 + v.k, 1,
       CASE WHEN v.k = 4 THEN tstzrange('2023-06-01 00:00:00+00', NULL)
            ELSE tstzrange('2020-01-01 00:00:00+00', NULL) END,
       'fixture'
FROM (SELECT k, CASE k WHEN 16 THEN 3 WHEN 17 THEN 20 ELSE k END AS parcel_n FROM generate_series(1, 17) k) v
JOIN core.improvements i ON i.public_id = pg_temp.fid('53000000', v.k)
JOIN core.addresses a ON a.public_id = pg_temp.fid('20000000', v.parcel_n)
JOIN core.improvement_types it ON it.public_id = pg_temp.fid('50000000', CASE WHEN v.parcel_n <= 12 THEN 1 ELSE 2 END)
JOIN core.improvement_conditions c ON c.public_id = pg_temp.fid('51000000', ((v.k - 1) % 3) + 1)
JOIN core.improvement_qualities q ON q.public_id = pg_temp.fid('52000000', 1);

-- Improvement 4's earlier version, before a renovation on 2023-06-01.
INSERT INTO core.improvement_attributes (improvement_id, address_id, improvement_type_id, improvement_condition_id, improvement_quality_id, area_sq_m, full_bathrooms, half_bathrooms, bedrooms, year_built, effective_year_built, units, legal_valid_range, trace_id)
SELECT i.improvement_id, a.address_id, it.improvement_type_id, c.improvement_condition_id, q.improvement_quality_id,
       100, 1, NULL, 2, 1984, 1984, 1,
       tstzrange('2020-01-01 00:00:00+00', '2023-06-01 00:00:00+00'), 'fixture'
FROM core.improvements i, core.addresses a, core.improvement_types it, core.improvement_conditions c, core.improvement_qualities q
WHERE i.public_id = pg_temp.fid('53000000', 4)
  AND a.public_id = pg_temp.fid('20000000', 4)
  AND it.public_id = pg_temp.fid('50000000', 1)
  AND c.public_id = pg_temp.fid('51000000', 3)
  AND q.public_id = pg_temp.fid('52000000', 1);

-- Improvement 18: a shed at address 12 with only an area and a year built.
INSERT INTO core.improvement_attributes (improvement_id, address_id, improvement_type_id, area_sq_m, year_built, legal_valid_range, trace_id)
SELECT i.improvement_id, a.address_id, it.improvement_type_id, 20, 2001,
       tstzrange('2020-01-01 00:00:00+00', NULL), 'fixture'
FROM core.improvements i, core.addresses a, core.improvement_types it
WHERE i.public_id = pg_temp.fid('53000000', 18)
  AND a.public_id = pg_temp.fid('20000000', 12)
  AND it.public_id = pg_temp.fid('50000000', 1);

INSERT INTO core.parcel_improvements (parcel_id, improvement_id, is_primary, legal_valid_range, trace_id)
SELECT p.parcel_id, i.improvement_id, v.k <> 16, tstzrange('2020-01-01 00:00:00+00', NULL), 'fixture'
FROM (SELECT k, CASE k WHEN 16 THEN 3 WHEN 17 THEN 20 ELSE k END AS parcel_n FROM generate_series(1, 17) k) v
JOIN core.parcels p ON p.public_id = pg_temp.fid('a0000000', v.parcel_n)
JOIN core.improvements i ON i.public_id = pg_temp.fid('53000000', v.k);

-- Improvement 18 is never primary: on parcel 11 until 2024-01-01, then on parcel 12.
INSERT INTO core.parcel_improvements (parcel_id, improvement_id, is_primary, legal_valid_range, trace_id)
SELECT p.parcel_id, i.improvement_id, false, v.r, 'fixture'
FROM (VALUES (11, tstzrange('2020-01-01 00:00:00+00', '2024-01-01 00:00:00+00')),
             (12, tstzrange('2024-01-01 00:00:00+00', NULL))) v(parcel_n, r)
JOIN core.parcels p ON p.public_id = pg_temp.fid('a0000000', v.parcel_n)
JOIN core.improvements i ON i.public_id = pg_temp.fid('53000000', 18);

-- Owners recorded on improvements themselves (improvement_parties). Improvement
-- 16's owners differ from parcel 3's; voided party 11 must be hidden; improvement
-- 6 passes from party 10 to party 6 with transfer 3; improvement 17 is hidden.
INSERT INTO core.improvement_parties (improvement_id, party_id, ownership_share, legal_valid_range, trace_id)
SELECT i.improvement_id, pt.party_id, v.share, v.r, 'fixture'
FROM (VALUES (2, 2, 0.5000, tstzrange('2020-01-01 00:00:00+00', NULL)),
             (2, 3, 0.5000, tstzrange('2020-01-01 00:00:00+00', NULL)),
             (3, 3, 1.0000, tstzrange('2020-01-01 00:00:00+00', NULL)),
             (16, 5, 0.5000, tstzrange('2020-01-01 00:00:00+00', NULL)),
             (16, 11, 0.5000, tstzrange('2020-01-01 00:00:00+00', NULL)),
             (6, 10, 1.0000, tstzrange('2020-01-01 00:00:00+00', '2023-09-01 00:00:00+00')),
             (6, 6, 1.0000, tstzrange('2023-09-01 00:00:00+00', NULL)),
             (17, 10, 1.0000, tstzrange('2020-01-01 00:00:00+00', NULL))) v(improvement_n, party_n, share, r)
JOIN core.improvements i ON i.public_id = pg_temp.fid('53000000', v.improvement_n)
JOIN core.parties pt ON pt.public_id = pg_temp.fid('30000000', v.party_n);

INSERT INTO core.improvement_valuations (improvement_id, valuation_id, market_value, assessed_value, legal_valid_range, trace_id)
SELECT i.improvement_id, v.valuation_id, 80000 + 2000 * k, (80000 + 2000 * k) * 0.9, tstzrange('2020-01-01 00:00:00+00', NULL), 'fixture'
FROM generate_series(1, 17) k
JOIN core.improvements i ON i.public_id = pg_temp.fid('53000000', k)
CROSS JOIN core.valuations v;

-- Real property transfers -------------------------------------------------------------

INSERT INTO core.real_property_transfer_code_types (public_id, name, description, trace_id)
VALUES (pg_temp.fid('91000000', 1), 'Fixture Qualification', 'Fixture qualification of a sale', 'fixture');

INSERT INTO core.real_property_transfer_codes (public_id, real_property_transfer_code_type_id, name, description, trace_id)
SELECT pg_temp.fid('92000000', v.n), ct.real_property_transfer_code_type_id, v.name, v.description, 'fixture'
FROM (VALUES (1, 1, 'Fixture Qualified', 'Fixture arm''s-length sale'),
             (2, 1, 'Fixture Unqualified', NULL),
             (3, NULL, 'Fixture Untyped', NULL)) v(n, type_n, name, description)
LEFT JOIN core.real_property_transfer_code_types ct ON ct.public_id = pg_temp.fid('91000000', v.type_n);

INSERT INTO core.real_property_transfers (public_id, legacy_id, transfer_timestamp, transfer_amount, deed_book, deed_page, deed_uri, trace_id)
SELECT pg_temp.fid('90000000', v.n), 'FX-RPT-' || v.n, v.ts, v.amount,
       CASE WHEN v.n <> 6 THEN 'FX-B' || v.n END,
       CASE WHEN v.n <> 6 THEN v.n::text END,
       CASE WHEN v.n <> 6 THEN 'https://recordings.example.invalid/FX-' || v.n END,
       'fixture'
FROM (VALUES (1, timestamptz '2021-03-01 00:00:00+00', 250000.0000),
             (2, timestamptz '2022-06-15 00:00:00+00', 310000.0000),
             (3, timestamptz '2023-09-01 00:00:00+00', 520000.0000),
             (4, timestamptz '2024-02-01 00:00:00+00', 90000.0000),
             (5, timestamptz '2024-05-01 00:00:00+00', 180000.0000),
             (6, timestamptz '2025-01-10 00:00:00+00', NULL),
             (7, timestamptz '2022-01-15 00:00:00+00', 140000.0000)) v(n, ts, amount);

-- (transfer, parcel, party, is_grantor, share)
INSERT INTO core.real_property_transfer_party_parcels (real_property_transfer_id, parcel_id, party_id, is_grantor, transferred_share, trace_id)
SELECT t.real_property_transfer_id, p.parcel_id, pt.party_id, v.is_grantor, v.share, 'fixture'
FROM (VALUES (1, 5, 9, true, 1.0000), (1, 5, 4, false, 1.0000),
             (2, 2, 1, true, 1.0000), (2, 2, 2, false, 0.5000), (2, 2, 3, false, 0.5000),
             (3, 6, 10, true, 1.0000), (3, 6, 6, false, 1.0000),
             (3, 7, 10, true, 1.0000), (3, 7, 7, false, 1.0000),
             (4, 20, 1, true, 1.0000), (4, 20, 10, false, 1.0000),
             (5, 19, 1, true, 0.5000), (5, 19, 2, true, 0.5000), (5, 19, 9, false, 1.0000),
             (5, 20, 1, true, 1.0000), (5, 20, 10, false, 1.0000),
             (6, 5, 4, true, 1.0000), (6, 5, 5, false, 1.0000),
             (7, 8, 3, true, 1.0000)) v(transfer_n, parcel_n, party_n, is_grantor, share)
JOIN core.real_property_transfers t ON t.public_id = pg_temp.fid('90000000', v.transfer_n)
JOIN core.parcels p ON p.public_id = pg_temp.fid('a0000000', v.parcel_n)
JOIN core.parties pt ON pt.public_id = pg_temp.fid('30000000', v.party_n);

-- (transfer, improvement, party, is_grantor, share)
INSERT INTO core.real_property_transfer_party_improvements (real_property_transfer_id, improvement_id, party_id, is_grantor, transferred_share, trace_id)
SELECT t.real_property_transfer_id, i.improvement_id, pt.party_id, v.is_grantor, v.share, 'fixture'
FROM (VALUES (3, 6, 10, true, 1.0000), (3, 6, 6, false, 1.0000),
             (3, 7, 10, true, 1.0000), (3, 7, 7, false, 1.0000),
             (4, 17, 1, true, 1.0000), (4, 17, 10, false, 1.0000)) v(transfer_n, improvement_n, party_n, is_grantor, share)
JOIN core.real_property_transfers t ON t.public_id = pg_temp.fid('90000000', v.transfer_n)
JOIN core.improvements i ON i.public_id = pg_temp.fid('53000000', v.improvement_n)
JOIN core.parties pt ON pt.public_id = pg_temp.fid('30000000', v.party_n);

INSERT INTO core.real_property_transfer_code_assignments (real_property_transfer_id, real_property_transfer_code_id, trace_id)
SELECT t.real_property_transfer_id, c.real_property_transfer_code_id, 'fixture'
FROM (VALUES (1, 1), (1, 3), (3, 2)) v(transfer_n, code_n)
JOIN core.real_property_transfers t ON t.public_id = pg_temp.fid('90000000', v.transfer_n)
JOIN core.real_property_transfer_codes c ON c.public_id = pg_temp.fid('92000000', v.code_n);

COMMIT;
