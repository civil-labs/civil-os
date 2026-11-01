-- Add new schema named "extensions"
CREATE SCHEMA "extensions";
-- Set comment to schema: "extensions"
COMMENT ON SCHEMA "extensions" IS 'schema for isolated extensions';
-- Create extension "postgis"
CREATE EXTENSION "postgis" WITH SCHEMA "extensions" VERSION "3.5.6";
-- Create extension "pgcrypto"
CREATE EXTENSION "pgcrypto" WITH SCHEMA "extensions" VERSION "1.3";
-- Create extension "btree_gist"
CREATE EXTENSION "btree_gist" WITH SCHEMA "extensions" VERSION "1.7";
-- Add new schema named "core"
CREATE SCHEMA "core";
-- Set comment to schema: "core"
COMMENT ON SCHEMA "core" IS 'schema for relational data and audit';
-- Create "neighborhood_definitions_history" table
CREATE TABLE "core"."neighborhood_definitions_history" (
  "neighborhood_definition_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "neighborhood_definition_id" bigint NOT NULL,
  "public_id" uuid NOT NULL,
  "legacy_id" text NULL,
  "name" text NOT NULL,
  "is_legal" boolean NOT NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("neighborhood_definition_history_id")
);
-- Create index "idx_neighborhood_definitions_history_neighborhood_definition_id" to table: "neighborhood_definitions_history"
CREATE INDEX "idx_neighborhood_definitions_history_neighborhood_definition_id" ON "core"."neighborhood_definitions_history" ("neighborhood_definition_id");
-- Create "record_real_property_transfer_code_types_history" function
CREATE FUNCTION "core"."record_real_property_transfer_code_types_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO real_property_transfer_code_types_history (
      real_property_transfer_code_type_id,
      public_id,
      legacy_id,
      name,
      description,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.real_property_transfer_code_type_id,
      OLD.public_id,
      OLD.legacy_id,
      OLD.name,
      OLD.description,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "real_property_transfer_code_types" table
CREATE TABLE "core"."real_property_transfer_code_types" (
  "real_property_transfer_code_type_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "public_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "legacy_id" text NULL,
  "name" text NOT NULL,
  "description" text NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("real_property_transfer_code_type_id")
);
-- Create index "idx_rpt_code_types_legacy_id" to table: "real_property_transfer_code_types"
CREATE UNIQUE INDEX "idx_rpt_code_types_legacy_id" ON "core"."real_property_transfer_code_types" ("legacy_id");
-- Create index "idx_rpt_code_types_name" to table: "real_property_transfer_code_types"
CREATE UNIQUE INDEX "idx_rpt_code_types_name" ON "core"."real_property_transfer_code_types" ("name");
-- Create index "idx_rpt_code_types_public_id" to table: "real_property_transfer_code_types"
CREATE UNIQUE INDEX "idx_rpt_code_types_public_id" ON "core"."real_property_transfer_code_types" ("public_id");
-- Create trigger "record_real_property_transfer_code_types_history"
CREATE TRIGGER "record_real_property_transfer_code_types_history" BEFORE DELETE OR UPDATE ON "core"."real_property_transfer_code_types" FOR EACH ROW EXECUTE FUNCTION "core"."record_real_property_transfer_code_types_history"();
-- Set comment to schema: "public"
COMMENT ON SCHEMA "public" IS 'default postgres schema (locked down)';
-- Add new schema named "geo"
CREATE SCHEMA "geo";
-- Set comment to schema: "geo"
COMMENT ON SCHEMA "geo" IS 'schema for heavy spatial data';
-- Create "parcels" table
CREATE TABLE "core"."parcels" (
  "parcel_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "public_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "legacy_id" text NULL,
  "is_voided" boolean NOT NULL DEFAULT false,
  "voided_at" timestamptz NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("parcel_id"),
  CONSTRAINT "chk_voided_logic" CHECK (((is_voided = false) AND (voided_at IS NULL)) OR ((is_voided = true) AND (voided_at IS NOT NULL)))
);
-- Create index "idx_parcels_legacy_id" to table: "parcels"
CREATE UNIQUE INDEX "idx_parcels_legacy_id" ON "core"."parcels" ("legacy_id");
-- Create index "idx_parcels_public_id" to table: "parcels"
CREATE UNIQUE INDEX "idx_parcels_public_id" ON "core"."parcels" ("public_id");
-- Create "generate_secure_mvt_id" function
CREATE FUNCTION "geo"."generate_secure_mvt_id" () RETURNS bigint LANGUAGE plpgsql PARALLEL SAFE AS $$
DECLARE
    b bytea := gen_random_bytes(7);
    res bigint := 0;
BEGIN
    res := (get_byte(b, 0)::bigint << 48) |
           (get_byte(b, 1)::bigint << 40) |
           (get_byte(b, 2)::bigint << 32) |
           (get_byte(b, 3)::bigint << 24) |
           (get_byte(b, 4)::bigint << 16) |
           (get_byte(b, 5)::bigint << 8) |
           (get_byte(b, 6)::bigint);
               
    RETURN GREATEST(1, res & 9007199254740991);
END;
$$;
-- Create "parcel_geometry" table
CREATE TABLE "geo"."parcel_geometry" (
  "parcel_geometry_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "parcel_id" bigint NOT NULL,
  "feature_id" bigint NOT NULL DEFAULT geo.generate_secure_mvt_id(),
  "geom_web" extensions.geometry(MultiPolygon,4326) NOT NULL,
  "geom_legal" extensions.geometry(MultiPolygon) NULL,
  "local_srid" integer NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("parcel_geometry_id"),
  CONSTRAINT "no_overlapping_feature_id" EXCLUDE USING GIST ("feature_id" WITH =, "legal_valid_range" WITH &&),
  CONSTRAINT "no_overlapping_parcel_geometry" EXCLUDE USING GIST ("parcel_id" WITH =, "legal_valid_range" WITH &&),
  CONSTRAINT "fk_parcel_id" FOREIGN KEY ("parcel_id") REFERENCES "core"."parcels" ("parcel_id") ON UPDATE NO ACTION ON DELETE RESTRICT
);
-- Create index "idx_geom_web_parcel_geometry" to table: "parcel_geometry"
CREATE INDEX "idx_geom_web_parcel_geometry" ON "geo"."parcel_geometry" USING GIST ("geom_web");
-- Create index "idx_parcel_geometry_feature_id" to table: "parcel_geometry"
CREATE INDEX "idx_parcel_geometry_feature_id" ON "geo"."parcel_geometry" ("feature_id");
-- Create index "idx_parcel_geometry_parcel_id" to table: "parcel_geometry"
CREATE INDEX "idx_parcel_geometry_parcel_id" ON "geo"."parcel_geometry" ("parcel_id");
-- Create "get_parcel_tiles" function
CREATE FUNCTION "core"."get_parcel_tiles" ("z" integer, "x" integer, "y" integer) RETURNS bytea LANGUAGE plpgsql STRICT STABLE PARALLEL SAFE AS $$
DECLARE
  mvt bytea;
  bounds extensions.geometry;
BEGIN
   -- Guard against invalid zoom levels causing ST_TileEnvelope to overflow
  IF z < 0 OR z > 24 THEN
      RETURN NULL;
  END IF;
      
  bounds := ST_TileEnvelope(z, x, y);
      
  WITH tile AS (
    SELECT 
        pg.feature_id AS id,
        -- Clip the geometry to the tile boundary for performance
        ST_AsMVTGeom(ST_Transform(pg.geom_web, 3857), bounds, 4096, 256, true) AS geom
    FROM 
        core.parcels p
    JOIN
        geo.parcel_geometry pg ON p.parcel_id = pg.parcel_id
    WHERE 
        p.is_voided = false AND
        pg.legal_valid_range @> NOW() AND
        ST_Intersects(pg.geom_web, ST_Transform(bounds, 4326))
  )
  SELECT ST_AsMVT(tile, 'parcels', 4096, 'geom', 'id')
  INTO mvt
  FROM tile;
  RETURN mvt;
END;
$$;
-- Create "improvements" table
CREATE TABLE "core"."improvements" (
  "improvement_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "public_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "legacy_id" text NULL,
  "is_voided" boolean NOT NULL DEFAULT false,
  "voided_at" timestamptz NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("improvement_id"),
  CONSTRAINT "chk_voided_logic" CHECK (((is_voided = false) AND (voided_at IS NULL)) OR ((is_voided = true) AND (voided_at IS NOT NULL)))
);
-- Create index "idx_improvements_legacy_id" to table: "improvements"
CREATE UNIQUE INDEX "idx_improvements_legacy_id" ON "core"."improvements" ("legacy_id");
-- Create index "idx_improvements_public_id" to table: "improvements"
CREATE UNIQUE INDEX "idx_improvements_public_id" ON "core"."improvements" ("public_id");
-- Create "parcel_improvements" table
CREATE TABLE "core"."parcel_improvements" (
  "parcel_improvement_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "parcel_id" bigint NOT NULL,
  "improvement_id" bigint NOT NULL,
  "is_primary" boolean NOT NULL DEFAULT false,
  "legal_valid_range" tstzrange NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("parcel_improvement_id"),
  CONSTRAINT "no_overlapping_improvements" EXCLUDE USING GIST ("improvement_id" WITH =, "legal_valid_range" WITH &&),
  CONSTRAINT "no_overlapping_primary_improvements" EXCLUDE USING GIST ("parcel_id" WITH =, "legal_valid_range" WITH &&) WHERE (is_primary = true),
  CONSTRAINT "fk_improvement_id" FOREIGN KEY ("improvement_id") REFERENCES "core"."improvements" ("improvement_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_parcel_id" FOREIGN KEY ("parcel_id") REFERENCES "core"."parcels" ("parcel_id") ON UPDATE NO ACTION ON DELETE RESTRICT
);
-- Create index "idx_parcel_improvements_improvement_id" to table: "parcel_improvements"
CREATE INDEX "idx_parcel_improvements_improvement_id" ON "core"."parcel_improvements" ("improvement_id");
-- Create index "idx_parcel_improvements_parcel_id" to table: "parcel_improvements"
CREATE INDEX "idx_parcel_improvements_parcel_id" ON "core"."parcel_improvements" ("parcel_id");
-- Create "get_primary_improvements" function
CREATE FUNCTION "core"."get_primary_improvements" ("parcel_ids" bigint[], "target_time" timestamptz) RETURNS TABLE ("parcel_id" bigint, "improvement_id" bigint) LANGUAGE plpgsql STABLE PARALLEL SAFE AS $$
BEGIN
  RETURN QUERY
  SELECT 
    sub.parcel_id, 
    sub.improvement_id
  FROM (
    SELECT 
      pi.parcel_id,
      pi.improvement_id,
      ROW_NUMBER() OVER (
        PARTITION BY pi.parcel_id 
        ORDER BY 
          pi.is_primary DESC, 
          COALESCE(ia.area_sq_m, 0.0) DESC, 
          pi.improvement_id ASC
      ) as rn
    FROM 
      core.parcel_improvements pi
    LEFT JOIN 
      geo.improvement_attributes ia ON pi.improvement_id = ia.improvement_id 
      AND ia.legal_valid_range @> target_time
    WHERE 
      pi.parcel_id = ANY(parcel_ids)
      AND pi.legal_valid_range @> target_time
  ) AS sub
  WHERE 
    sub.rn = 1;
END;
$$;
-- Create "prevent_history_tampering" function
CREATE FUNCTION "core"."prevent_history_tampering" () RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'TAMPER ALERT: History tables are immutable, append-only ledgers. UPDATE, DELETE, and TRUNCATE operations are strictly forbidden.';
  RETURN NULL;
END;
$$;
-- Create "record_address_attributes_history" function
CREATE FUNCTION "core"."record_address_attributes_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO address_attributes_history (
      address_attribute_id,
      address_id,
      country_id,
      administrative_area_id,
      locality,
      sublocality,
      postal_code,
      address_line_1,
      address_line_2,
      address_line_3,
      formatted_address,
      coordinates,
      legal_valid_range,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.address_attribute_id,
      OLD.address_id,
      OLD.country_id,
      OLD.administrative_area_id,
      OLD.locality,
      OLD.sublocality,
      OLD.postal_code,
      OLD.address_line_1,
      OLD.address_line_2,
      OLD.address_line_3,
      OLD.formatted_address,
      OLD.coordinates,
      OLD.legal_valid_range,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_addresses_history" function
CREATE FUNCTION "core"."record_addresses_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO addresses_history (
      address_id,
      public_id,
      legacy_id,
      is_voided,
      voided_at,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.address_id,
      OLD.public_id,
      OLD.legacy_id,
      OLD.is_voided,
      OLD.voided_at,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_improvement_attributes_history" function
CREATE FUNCTION "core"."record_improvement_attributes_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO improvement_attributes_history (
      improvement_attribute_id,
      improvement_id,
      address_id,
      improvement_type_id,
      improvement_condition_id,
      improvement_quality_id,
      area_sq_m,
      bathrooms,
      bedrooms,
      year_built,
      effective_year_built,
      depreciation_modifier_override,
      quality_adjustment_modifier_override,
      units,
      properties,
      legal_valid_range,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.improvement_attribute_id,
      OLD.improvement_id,
      OLD.address_id,
      OLD.improvement_type_id,
      OLD.improvement_condition_id,
      OLD.improvement_quality_id,
      OLD.area_sq_m,
      OLD.bathrooms,
      OLD.bedrooms,
      OLD.year_built,
      OLD.effective_year_built,
      OLD.depreciation_modifier_override,
      OLD.quality_adjustment_modifier_override,
      OLD.units,
      OLD.properties,
      OLD.legal_valid_range,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_improvement_condition_attributes_history" function
CREATE FUNCTION "core"."record_improvement_condition_attributes_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO improvement_condition_attributes_history (
      improvement_condition_attribute_id,
      improvement_condition_id,
      name,
      depreciation_modifier,
      legal_valid_range,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.improvement_condition_attribute_id,
      OLD.improvement_condition_id,
      OLD.name,
      OLD.depreciation_modifier,
      OLD.legal_valid_range,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_improvement_conditions_history" function
CREATE FUNCTION "core"."record_improvement_conditions_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO improvement_conditions_history (
      improvement_condition_id,
      public_id,
      legacy_id,
      is_voided,
      voided_at,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.improvement_condition_id,
      OLD.public_id,
      OLD.legacy_id,
      OLD.is_voided,
      OLD.voided_at,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_improvement_geometry_history" function
CREATE FUNCTION "core"."record_improvement_geometry_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO improvement_geometry_history (
      improvement_geometry_id,
      improvement_id,
      geom_web,
      geom_legal,
      local_horizontal_srid,
      local_vertical_datum,
      legal_valid_range,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.improvement_geometry_id,
      OLD.improvement_id,
      OLD.geom_web,
      OLD.geom_legal,
      OLD.local_horizontal_srid,
      OLD.local_vertical_datum,
      OLD.legal_valid_range,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_improvement_parties_history" function
CREATE FUNCTION "core"."record_improvement_parties_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO improvement_parties_history (
      improvement_party_id,
      improvement_id,
      party_id,
      ownership_share,
      legal_valid_range,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.improvement_party_id,
      OLD.improvement_id,
      OLD.party_id,
      OLD.ownership_share,
      OLD.legal_valid_range,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_improvement_qualities_history" function
CREATE FUNCTION "core"."record_improvement_qualities_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO improvement_qualities_history (
      improvement_quality_id,
      public_id,
      legacy_id,
      is_voided,
      voided_at,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.improvement_quality_id,
      OLD.public_id,
      OLD.legacy_id,
      OLD.is_voided,
      OLD.voided_at,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_improvement_quality_attributes_history" function
CREATE FUNCTION "core"."record_improvement_quality_attributes_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO improvement_quality_attributes_history (
      improvement_quality_attribute_id,
      improvement_quality_id,
      name,
      quality_adjustment_modifier,
      legal_valid_range,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.improvement_quality_attribute_id,
      OLD.improvement_quality_id,
      OLD.name,
      OLD.quality_adjustment_modifier,
      OLD.legal_valid_range,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_improvement_valuations_history" function
CREATE FUNCTION "core"."record_improvement_valuations_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO improvement_valuations_history (
      improvement_valuation_id,
      improvement_id,
      valuation_id,
      market_value,
      assessed_value,
      legal_valid_range,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.improvement_valuation_id,
      OLD.improvement_id,
      OLD.valuation_id,
      OLD.market_value,
      OLD.assessed_value,
      OLD.legal_valid_range,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_improvements_history" function
CREATE FUNCTION "core"."record_improvements_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO improvements_history (
      improvement_id,
      public_id,
      legacy_id,
      is_voided,
      voided_at,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.improvement_id,
      OLD.public_id,
      OLD.legacy_id,
      OLD.is_voided,
      OLD.voided_at,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_neighborhood_definitions_history" function
CREATE FUNCTION "core"."record_neighborhood_definitions_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO neighborhood_definitions_history (
      neighborhood_definition_id,
      public_id,
      legacy_id,
      name,
      is_legal,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.neighborhood_definition_id,
      OLD.public_id,
      OLD.legacy_id,
      OLD.name,
      OLD.is_legal,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_neighborhoods_history" function
CREATE FUNCTION "core"."record_neighborhoods_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO neighborhoods_history (
      neighborhood_id,
      public_id,
      legacy_id,
      name,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.neighborhood_id,
      OLD.public_id,
      OLD.legacy_id,
      OLD.name,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_parcel_affordances_history" function
CREATE FUNCTION "core"."record_parcel_affordances_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO parcel_affordances_history (
      parcel_affordance_id,
      public_id,
      parcel_id,
      zoning_id,
      affordance_type_id,
      source,
      precedence_rank,
      max_far,
      min_lot_size_sq_m,
      max_height_m,
      max_dwelling_units_per_hectare,
      max_lot_coverage_pct,
      legal_valid_range,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.parcel_affordance_id,
      OLD.public_id,
      OLD.parcel_id,
      OLD.zoning_id,
      OLD.affordance_type_id,
      OLD.source,
      OLD.precedence_rank,
      OLD.max_far,
      OLD.min_lot_size_sq_m,
      OLD.max_height_m,
      OLD.max_dwelling_units_per_hectare,
      OLD.max_lot_coverage_pct,
      OLD.legal_valid_range,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_parcel_attributes_history" function
CREATE FUNCTION "core"."record_parcel_attributes_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO parcel_attributes_history (
      parcel_attribute_id,
      parcel_id,
      address_id,
      land_use_id,
      land_area_sq_m,
      frontage_m,
      depth_m,
      properties,
      legal_valid_range,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.parcel_attribute_id,
      OLD.parcel_id,
      OLD.address_id,
      OLD.land_use_id,
      OLD.land_area_sq_m,
      OLD.frontage_m,
      OLD.depth_m,
      OLD.properties,
      OLD.legal_valid_range,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_parcel_geometry_history" function
CREATE FUNCTION "core"."record_parcel_geometry_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO parcel_geometry_history (
      parcel_geometry_id,
      parcel_id,
      feature_id,
      geom_web,
      geom_legal,
      local_srid,
      legal_valid_range,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.parcel_geometry_id,
      OLD.parcel_id,
      OLD.feature_id,
      OLD.geom_web,
      OLD.geom_legal,
      OLD.local_srid,
      OLD.legal_valid_range,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_parcel_improvements_history" function
CREATE FUNCTION "core"."record_parcel_improvements_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO parcel_improvements_history (
      parcel_improvement_id,
      parcel_id,
      improvement_id,
      is_primary,
      legal_valid_range,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.parcel_improvement_id,
      OLD.parcel_id,
      OLD.improvement_id,
      OLD.is_primary,
      OLD.legal_valid_range,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_parcel_neighborhood_definitions_history" function
CREATE FUNCTION "core"."record_parcel_neighborhood_definitions_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO parcel_neighborhood_definitions_history (
      parcel_neighborhood_definition_id,
      parcel_id,
      neighborhood_id,
      neighborhood_definition_id,
      is_legal,
      legal_valid_range,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.parcel_neighborhood_definition_id,
      OLD.parcel_id,
      OLD.neighborhood_id,
      OLD.neighborhood_definition_id,
      OLD.is_legal,
      OLD.legal_valid_range,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_parcel_parties_history" function
CREATE FUNCTION "core"."record_parcel_parties_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO parcel_parties_history (
      parcel_party_id,
      parcel_id,
      party_id,
      ownership_share,
      legal_valid_range,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.parcel_party_id,
      OLD.parcel_id,
      OLD.party_id,
      OLD.ownership_share,
      OLD.legal_valid_range,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_parcel_valuations_history" function
CREATE FUNCTION "core"."record_parcel_valuations_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO parcel_valuations_history (
      parcel_valuation_id,
      parcel_id,
      valuation_id,
      market_value,
      assessed_value,
      legal_valid_range,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.parcel_valuation_id,
      OLD.parcel_id,
      OLD.valuation_id,
      OLD.market_value,
      OLD.assessed_value,
      OLD.legal_valid_range,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_parcels_history" function
CREATE FUNCTION "core"."record_parcels_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO parcels_history (
      parcel_id,
      public_id,
      legacy_id,
      is_voided,
      voided_at,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.parcel_id,
      OLD.public_id,
      OLD.legacy_id,
      OLD.is_voided,
      OLD.voided_at,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_parties_history" function
CREATE FUNCTION "core"."record_parties_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO parties_history (
      party_id,
      public_id,
      legacy_id,
      is_voided,
      voided_at,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.party_id,
      OLD.public_id,
      OLD.legacy_id,
      OLD.is_voided,
      OLD.voided_at,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_party_attributes_history" function
CREATE FUNCTION "core"."record_party_attributes_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO party_attributes_history (
      party_attribute_id,
      party_id,
      name,
      address_id,
      legal_valid_range,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.party_attribute_id,
      OLD.party_id,
      OLD.name,
      OLD.address_id,
      OLD.legal_valid_range,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_real_property_transfer_code_assignments_history" function
CREATE FUNCTION "core"."record_real_property_transfer_code_assignments_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO real_property_transfer_code_assignments_history (
      real_property_transfer_code_assignment_id,
      real_property_transfer_id,
      real_property_transfer_code_id,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.real_property_transfer_code_assignment_id,
      OLD.real_property_transfer_id,
      OLD.real_property_transfer_code_id,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_real_property_transfer_codes_history" function
CREATE FUNCTION "core"."record_real_property_transfer_codes_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO real_property_transfer_codes_history (
      real_property_transfer_code_id,
      public_id,
      legacy_id,
      real_property_transfer_code_type_id,
      name,
      description,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.real_property_transfer_code_id,
      OLD.public_id,
      OLD.legacy_id,
      OLD.real_property_transfer_code_type_id,
      OLD.name,
      OLD.description,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_real_property_transfer_party_improvements_history" function
CREATE FUNCTION "core"."record_real_property_transfer_party_improvements_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO real_property_transfer_party_improvements_history (
      real_property_transfer_party_improvement_id,
      improvement_id,
      real_property_transfer_id,
      party_id,
      is_grantor,
      transferred_share,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.real_property_transfer_party_improvement_id,
      OLD.improvement_id,
      OLD.real_property_transfer_id,
      OLD.party_id,
      OLD.is_grantor,
      OLD.transferred_share,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_real_property_transfer_party_parcels_history" function
CREATE FUNCTION "core"."record_real_property_transfer_party_parcels_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO real_property_transfer_party_parcels_history (
      real_property_transfer_party_parcel_id,
      parcel_id,
      real_property_transfer_id,
      party_id,
      is_grantor,
      transferred_share,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.real_property_transfer_party_parcel_id,
      OLD.parcel_id,
      OLD.real_property_transfer_id,
      OLD.party_id,
      OLD.is_grantor,
      OLD.transferred_share,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_real_property_transfers_history" function
CREATE FUNCTION "core"."record_real_property_transfers_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO real_property_transfers_history (
      real_property_transfer_id,
      public_id,
      legacy_id,
      transfer_timestamp,
      transfer_amount,
      deed_book,
      deed_page,
      deed_uri,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.real_property_transfer_id,
      OLD.public_id,
      OLD.legacy_id,
      OLD.transfer_timestamp,
      OLD.transfer_amount,
      OLD.deed_book,
      OLD.deed_page,
      OLD.deed_uri,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_system_settings_history" function
CREATE FUNCTION "core"."record_system_settings_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO system_settings_history (
      system_setting_id,
      base_currency,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.system_setting_id,
      OLD.base_currency,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the 
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_valuations_history" function
CREATE FUNCTION "core"."record_valuations_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO valuations_history (
      valuation_id,
      public_id,
      legacy_id,
      valuation_date,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.valuation_id,
      OLD.public_id,
      OLD.legacy_id,
      OLD.valuation_date,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_zoning_attributes_history" function
CREATE FUNCTION "core"."record_zoning_attributes_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO zoning_attributes_history (
      zoning_attribute_id,
      zoning_id,
      name,
      code,
      max_far,
      min_lot_size_sq_m,
      max_height_m,
      max_dwelling_units_per_hectare,
      max_lot_coverage_pct,
      legal_valid_range,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.zoning_attribute_id,
      OLD.zoning_id,
      OLD.name,
      OLD.code,
      OLD.max_far,
      OLD.min_lot_size_sq_m,
      OLD.max_height_m,
      OLD.max_dwelling_units_per_hectare,
      OLD.max_lot_coverage_pct,
      OLD.legal_valid_range,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "record_zoning_history" function
CREATE FUNCTION "core"."record_zoning_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO zoning_history (
      zoning_id,
      public_id,
      legacy_id,
      is_voided,
      voided_at,
      system_valid_range,
      trace_id
    ) VALUES (
      OLD.zoning_id,
      OLD.public_id,
      OLD.legacy_id,
      OLD.is_voided,
      OLD.voided_at,
      tstzrange(OLD.system_updated_at, current_transaction_time, '[)'),
      OLD.trace_id
    );
  END IF;
          
  -- Safely route the return pointer
  IF (TG_OP = 'UPDATE') THEN
      -- Ensures the record's system log is updated for the proper time
      NEW.system_updated_at = current_transaction_time;
      RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
      RETURN OLD;
  END IF;
        
  RETURN NULL;
END;
$$;
-- Create "parcel_geometry_history" table
CREATE TABLE "geo"."parcel_geometry_history" (
  "parcel_geometry_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "parcel_geometry_id" bigint NOT NULL,
  "parcel_id" bigint NOT NULL,
  "feature_id" bigint NOT NULL DEFAULT geo.generate_secure_mvt_id(),
  "geom_web" extensions.geometry(MultiPolygon,4326) NOT NULL,
  "geom_legal" extensions.geometry(MultiPolygon) NULL,
  "local_srid" integer NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("parcel_geometry_history_id")
);
-- Create index "idx_parcel_geometry_history_feature_id" to table: "parcel_geometry_history"
CREATE INDEX "idx_parcel_geometry_history_feature_id" ON "geo"."parcel_geometry_history" ("parcel_id");
-- Create index "idx_parcel_geometry_history_parcel_geometry_id" to table: "parcel_geometry_history"
CREATE INDEX "idx_parcel_geometry_history_parcel_geometry_id" ON "geo"."parcel_geometry_history" ("parcel_geometry_id");
-- Create index "idx_parcel_geometry_history_parcel_id" to table: "parcel_geometry_history"
CREATE INDEX "idx_parcel_geometry_history_parcel_id" ON "geo"."parcel_geometry_history" ("parcel_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "geo"."parcel_geometry_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create "addresses" table
CREATE TABLE "core"."addresses" (
  "address_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "public_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "legacy_id" text NULL,
  "is_voided" boolean NOT NULL DEFAULT false,
  "voided_at" timestamptz NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("address_id"),
  CONSTRAINT "chk_voided_logic" CHECK (((is_voided = false) AND (voided_at IS NULL)) OR ((is_voided = true) AND (voided_at IS NOT NULL)))
);
-- Create index "idx_addresses_legacy_id" to table: "addresses"
CREATE UNIQUE INDEX "idx_addresses_legacy_id" ON "core"."addresses" ("legacy_id");
-- Create index "idx_addresses_public_id" to table: "addresses"
CREATE UNIQUE INDEX "idx_addresses_public_id" ON "core"."addresses" ("public_id");
-- Create "administrative_areas" table
CREATE TABLE "core"."administrative_areas" (
  "administrative_area_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "public_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "code" character varying(10) NOT NULL,
  "name" character varying(100) NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("administrative_area_id")
);
-- Create index "idx_administrative_areas_public_id" to table: "administrative_areas"
CREATE UNIQUE INDEX "idx_administrative_areas_public_id" ON "core"."administrative_areas" ("public_id");
-- Create "countries" table
CREATE TABLE "core"."countries" (
  "country_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "public_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "code" character varying(2) NOT NULL,
  "name" character varying(100) NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("country_id")
);
-- Create index "idx_countries_public_id" to table: "countries"
CREATE UNIQUE INDEX "idx_countries_public_id" ON "core"."countries" ("public_id");
-- Create "address_attributes" table
CREATE TABLE "core"."address_attributes" (
  "address_attribute_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "address_id" bigint NOT NULL,
  "country_id" bigint NULL,
  "administrative_area_id" bigint NULL,
  "locality" text NULL,
  "sublocality" text NULL,
  "postal_code" text NULL,
  "address_line_1" text NULL,
  "address_line_2" text NULL,
  "address_line_3" text NULL,
  "formatted_address" text NOT NULL,
  "coordinates" extensions.geometry(Point,4326) NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("address_attribute_id"),
  CONSTRAINT "no_overlapping_address_attributes" EXCLUDE USING GIST ("address_id" WITH =, "legal_valid_range" WITH &&),
  CONSTRAINT "fk_address_id" FOREIGN KEY ("address_id") REFERENCES "core"."addresses" ("address_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_administrative_area_id" FOREIGN KEY ("administrative_area_id") REFERENCES "core"."administrative_areas" ("administrative_area_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_country_id" FOREIGN KEY ("country_id") REFERENCES "core"."countries" ("country_id") ON UPDATE NO ACTION ON DELETE RESTRICT
);
-- Create index "idx_address_attributes_address_id" to table: "address_attributes"
CREATE INDEX "idx_address_attributes_address_id" ON "core"."address_attributes" ("address_id");
-- Create trigger "record_address_attributes_history"
CREATE TRIGGER "record_address_attributes_history" BEFORE DELETE OR UPDATE ON "core"."address_attributes" FOR EACH ROW EXECUTE FUNCTION "core"."record_address_attributes_history"();
-- Create "address_attributes_history" table
CREATE TABLE "core"."address_attributes_history" (
  "address_attribute_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "address_attribute_id" bigint NOT NULL,
  "address_id" bigint NOT NULL,
  "country_id" bigint NULL,
  "administrative_area_id" bigint NULL,
  "locality" text NULL,
  "sublocality" text NULL,
  "postal_code" text NULL,
  "address_line_1" text NULL,
  "address_line_2" text NULL,
  "address_line_3" text NULL,
  "formatted_address" text NOT NULL,
  "coordinates" extensions.geometry(Point,4326) NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("address_attribute_history_id")
);
-- Create index "idx_address_attributes_history_address_attribute_id" to table: "address_attributes_history"
CREATE INDEX "idx_address_attributes_history_address_attribute_id" ON "core"."address_attributes_history" ("address_attribute_id");
-- Create index "idx_address_attributes_history_address_id" to table: "address_attributes_history"
CREATE INDEX "idx_address_attributes_history_address_id" ON "core"."address_attributes_history" ("address_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."address_attributes_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create trigger "record_addresses_history"
CREATE TRIGGER "record_addresses_history" BEFORE DELETE OR UPDATE ON "core"."addresses" FOR EACH ROW EXECUTE FUNCTION "core"."record_addresses_history"();
-- Create "addresses_history" table
CREATE TABLE "core"."addresses_history" (
  "address_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "address_id" bigint NOT NULL,
  "public_id" uuid NOT NULL,
  "legacy_id" text NULL,
  "is_voided" boolean NOT NULL,
  "voided_at" timestamptz NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("address_history_id")
);
-- Create index "idx_addresses_history_address_id" to table: "addresses_history"
CREATE INDEX "idx_addresses_history_address_id" ON "core"."addresses_history" ("address_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."addresses_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create trigger "record_parcel_geometry_history"
CREATE TRIGGER "record_parcel_geometry_history" BEFORE DELETE OR UPDATE ON "geo"."parcel_geometry" FOR EACH ROW EXECUTE FUNCTION "core"."record_parcel_geometry_history"();
-- Create "improvement_geometry_history" table
CREATE TABLE "geo"."improvement_geometry_history" (
  "improvement_geometry_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "improvement_geometry_id" bigint NOT NULL,
  "improvement_id" bigint NOT NULL,
  "geom_web" extensions.geometry(MultiPolygonZ,4326) NOT NULL,
  "geom_legal" extensions.geometry(MultiPolygonZ) NULL,
  "local_horizontal_srid" integer NULL,
  "local_vertical_datum" text NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("improvement_geometry_history_id")
);
-- Create index "idx_improvement_geometry_history_improvement_geometry_id" to table: "improvement_geometry_history"
CREATE INDEX "idx_improvement_geometry_history_improvement_geometry_id" ON "geo"."improvement_geometry_history" ("improvement_geometry_id");
-- Create index "idx_improvement_geometry_history_improvement_id" to table: "improvement_geometry_history"
CREATE INDEX "idx_improvement_geometry_history_improvement_id" ON "geo"."improvement_geometry_history" ("improvement_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "geo"."improvement_geometry_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create "improvement_conditions" table
CREATE TABLE "core"."improvement_conditions" (
  "improvement_condition_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "public_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "legacy_id" text NULL,
  "is_voided" boolean NOT NULL DEFAULT false,
  "voided_at" timestamptz NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("improvement_condition_id"),
  CONSTRAINT "chk_voided_logic" CHECK (((is_voided = false) AND (voided_at IS NULL)) OR ((is_voided = true) AND (voided_at IS NOT NULL)))
);
-- Create index "idx_improvement_conditions_legacy_id" to table: "improvement_conditions"
CREATE UNIQUE INDEX "idx_improvement_conditions_legacy_id" ON "core"."improvement_conditions" ("legacy_id");
-- Create index "idx_improvement_conditions_public_id" to table: "improvement_conditions"
CREATE UNIQUE INDEX "idx_improvement_conditions_public_id" ON "core"."improvement_conditions" ("public_id");
-- Create "improvement_qualities" table
CREATE TABLE "core"."improvement_qualities" (
  "improvement_quality_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "public_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "legacy_id" text NULL,
  "is_voided" boolean NOT NULL DEFAULT false,
  "voided_at" timestamptz NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("improvement_quality_id"),
  CONSTRAINT "chk_voided_logic" CHECK (((is_voided = false) AND (voided_at IS NULL)) OR ((is_voided = true) AND (voided_at IS NOT NULL)))
);
-- Create index "idx_improvement_qualities_legacy_id" to table: "improvement_qualities"
CREATE UNIQUE INDEX "idx_improvement_qualities_legacy_id" ON "core"."improvement_qualities" ("legacy_id");
-- Create index "idx_improvement_qualities_public_id" to table: "improvement_qualities"
CREATE UNIQUE INDEX "idx_improvement_qualities_public_id" ON "core"."improvement_qualities" ("public_id");
-- Create "improvement_types" table
CREATE TABLE "core"."improvement_types" (
  "improvement_type_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "public_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "legacy_id" text NULL,
  "code" text NULL,
  "name" text NOT NULL,
  "description" text NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("improvement_type_id")
);
-- Create index "idx_improvement_types_legacy_id" to table: "improvement_types"
CREATE UNIQUE INDEX "idx_improvement_types_legacy_id" ON "core"."improvement_types" ("legacy_id");
-- Create index "idx_improvement_types_name" to table: "improvement_types"
CREATE UNIQUE INDEX "idx_improvement_types_name" ON "core"."improvement_types" ("name");
-- Create index "idx_improvement_types_public_id" to table: "improvement_types"
CREATE UNIQUE INDEX "idx_improvement_types_public_id" ON "core"."improvement_types" ("public_id");
-- Create "improvement_attributes" table
CREATE TABLE "core"."improvement_attributes" (
  "improvement_attribute_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "improvement_id" bigint NOT NULL,
  "address_id" bigint NOT NULL,
  "improvement_type_id" bigint NOT NULL,
  "improvement_condition_id" bigint NULL,
  "improvement_quality_id" bigint NULL,
  "area_sq_m" double precision NULL,
  "bathrooms" integer NULL,
  "bedrooms" integer NULL,
  "year_built" integer NULL,
  "effective_year_built" integer NULL,
  "depreciation_modifier_override" numeric(5,4) NULL,
  "quality_adjustment_modifier_override" double precision NULL,
  "units" integer NULL,
  "properties" jsonb NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("improvement_attribute_id"),
  CONSTRAINT "no_overlapping_improvement_attributes" EXCLUDE USING GIST ("improvement_id" WITH =, "legal_valid_range" WITH &&),
  CONSTRAINT "fk_address_id" FOREIGN KEY ("address_id") REFERENCES "core"."addresses" ("address_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_improvement_condition_id" FOREIGN KEY ("improvement_condition_id") REFERENCES "core"."improvement_conditions" ("improvement_condition_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_improvement_id" FOREIGN KEY ("improvement_id") REFERENCES "core"."improvements" ("improvement_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_improvement_quality_id" FOREIGN KEY ("improvement_quality_id") REFERENCES "core"."improvement_qualities" ("improvement_quality_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_improvement_type_id" FOREIGN KEY ("improvement_type_id") REFERENCES "core"."improvement_types" ("improvement_type_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "properties_size_limit" CHECK (pg_column_size(properties) <= 5000)
);
-- Create index "idx_improvement_attributes_address_id" to table: "improvement_attributes"
CREATE INDEX "idx_improvement_attributes_address_id" ON "core"."improvement_attributes" ("address_id");
-- Create index "idx_improvement_attributes_improvement_condition_id" to table: "improvement_attributes"
CREATE INDEX "idx_improvement_attributes_improvement_condition_id" ON "core"."improvement_attributes" ("improvement_condition_id");
-- Create index "idx_improvement_attributes_improvement_id" to table: "improvement_attributes"
CREATE INDEX "idx_improvement_attributes_improvement_id" ON "core"."improvement_attributes" ("improvement_id");
-- Create index "idx_improvement_attributes_improvement_quality_id" to table: "improvement_attributes"
CREATE INDEX "idx_improvement_attributes_improvement_quality_id" ON "core"."improvement_attributes" ("improvement_quality_id");
-- Create index "idx_improvement_attributes_improvement_type_id" to table: "improvement_attributes"
CREATE INDEX "idx_improvement_attributes_improvement_type_id" ON "core"."improvement_attributes" ("improvement_type_id");
-- Create trigger "record_improvement_attributes_history"
CREATE TRIGGER "record_improvement_attributes_history" BEFORE DELETE OR UPDATE ON "core"."improvement_attributes" FOR EACH ROW EXECUTE FUNCTION "core"."record_improvement_attributes_history"();
-- Create "improvement_attributes_history" table
CREATE TABLE "core"."improvement_attributes_history" (
  "improvement_attribute_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "improvement_attribute_id" bigint NOT NULL,
  "improvement_id" bigint NOT NULL,
  "address_id" bigint NOT NULL,
  "improvement_type_id" bigint NOT NULL,
  "improvement_condition_id" bigint NULL,
  "area_sq_m" double precision NULL,
  "bathrooms" integer NULL,
  "bedrooms" integer NULL,
  "year_built" integer NULL,
  "effective_year_built" integer NULL,
  "depreciation_modifier_override" numeric(5,4) NULL,
  "quality_adjustment_modifier_override" double precision NULL,
  "units" integer NULL,
  "properties" jsonb NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("improvement_attribute_history_id")
);
-- Create index "idx_improvement_attributes_history_improvement_attribute_id" to table: "improvement_attributes_history"
CREATE INDEX "idx_improvement_attributes_history_improvement_attribute_id" ON "core"."improvement_attributes_history" ("improvement_attribute_id");
-- Create index "idx_improvement_attributes_history_improvement_id" to table: "improvement_attributes_history"
CREATE INDEX "idx_improvement_attributes_history_improvement_id" ON "core"."improvement_attributes_history" ("improvement_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."improvement_attributes_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create "improvement_geometry" table
CREATE TABLE "geo"."improvement_geometry" (
  "improvement_geometry_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "improvement_id" bigint NOT NULL,
  "geom_web" extensions.geometry(MultiPolygonZ,4326) NOT NULL,
  "geom_legal" extensions.geometry(MultiPolygonZ) NULL,
  "local_horizontal_srid" integer NULL,
  "local_vertical_datum" text NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("improvement_geometry_id"),
  CONSTRAINT "no_overlapping_improvement_geometry" EXCLUDE USING GIST ("improvement_id" WITH =, "legal_valid_range" WITH &&),
  CONSTRAINT "fk_improvement_id" FOREIGN KEY ("improvement_id") REFERENCES "core"."improvements" ("improvement_id") ON UPDATE NO ACTION ON DELETE RESTRICT
);
-- Create index "idx_geom_web_improvement_geometry" to table: "improvement_geometry"
CREATE INDEX "idx_geom_web_improvement_geometry" ON "geo"."improvement_geometry" USING GIST ("geom_web");
-- Create index "idx_improvement_geometry_improvement_id" to table: "improvement_geometry"
CREATE INDEX "idx_improvement_geometry_improvement_id" ON "geo"."improvement_geometry" ("improvement_id");
-- Create trigger "record_improvement_geometry_history"
CREATE TRIGGER "record_improvement_geometry_history" BEFORE DELETE OR UPDATE ON "geo"."improvement_geometry" FOR EACH ROW EXECUTE FUNCTION "core"."record_improvement_geometry_history"();
-- Create "improvement_condition_attributes" table
CREATE TABLE "core"."improvement_condition_attributes" (
  "improvement_condition_attribute_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "improvement_condition_id" bigint NOT NULL,
  "name" text NOT NULL,
  "depreciation_modifier" numeric(5,4) NOT NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("improvement_condition_attribute_id"),
  CONSTRAINT "no_overlapping_improvement_conditions" EXCLUDE USING GIST ("improvement_condition_id" WITH =, "legal_valid_range" WITH &&),
  CONSTRAINT "fk_improvement_condition_id" FOREIGN KEY ("improvement_condition_id") REFERENCES "core"."improvement_conditions" ("improvement_condition_id") ON UPDATE NO ACTION ON DELETE RESTRICT
);
-- Create trigger "record_improvement_condition_attributes_history"
CREATE TRIGGER "record_improvement_condition_attributes_history" BEFORE DELETE OR UPDATE ON "core"."improvement_condition_attributes" FOR EACH ROW EXECUTE FUNCTION "core"."record_improvement_condition_attributes_history"();
-- Create "improvement_condition_attributes_history" table
CREATE TABLE "core"."improvement_condition_attributes_history" (
  "improvement_condition_attribute_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "improvement_condition_attribute_id" bigint NOT NULL,
  "improvement_condition_id" bigint NOT NULL,
  "name" text NOT NULL,
  "depreciation_modifier" numeric(5,4) NOT NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("improvement_condition_attribute_history_id")
);
-- Create index "idx_impr_cond_attr_history_impr_cond_attr_id" to table: "improvement_condition_attributes_history"
CREATE INDEX "idx_impr_cond_attr_history_impr_cond_attr_id" ON "core"."improvement_condition_attributes_history" ("improvement_condition_attribute_id");
-- Create index "idx_impr_cond_attr_history_impr_cond_id" to table: "improvement_condition_attributes_history"
CREATE INDEX "idx_impr_cond_attr_history_impr_cond_id" ON "core"."improvement_condition_attributes_history" ("improvement_condition_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."improvement_condition_attributes_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."neighborhood_definitions_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create trigger "record_improvement_conditions_history"
CREATE TRIGGER "record_improvement_conditions_history" BEFORE DELETE OR UPDATE ON "core"."improvement_conditions" FOR EACH ROW EXECUTE FUNCTION "core"."record_improvement_conditions_history"();
-- Create "improvement_conditions_history" table
CREATE TABLE "core"."improvement_conditions_history" (
  "improvement_condition_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "improvement_condition_id" bigint NOT NULL,
  "public_id" uuid NOT NULL,
  "legacy_id" text NULL,
  "is_voided" boolean NOT NULL,
  "voided_at" timestamptz NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("improvement_condition_history_id")
);
-- Create index "idx_improvement_condition_history_improvement_condition_id" to table: "improvement_conditions_history"
CREATE INDEX "idx_improvement_condition_history_improvement_condition_id" ON "core"."improvement_conditions_history" ("improvement_condition_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."improvement_conditions_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create "parties" table
CREATE TABLE "core"."parties" (
  "party_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "public_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "legacy_id" text NULL,
  "is_voided" boolean NOT NULL DEFAULT false,
  "voided_at" timestamptz NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("party_id"),
  CONSTRAINT "chk_voided_logic" CHECK (((is_voided = false) AND (voided_at IS NULL)) OR ((is_voided = true) AND (voided_at IS NOT NULL)))
);
-- Create index "idx_parties_legacy_id" to table: "parties"
CREATE UNIQUE INDEX "idx_parties_legacy_id" ON "core"."parties" ("legacy_id");
-- Create index "idx_parties_public_id" to table: "parties"
CREATE UNIQUE INDEX "idx_parties_public_id" ON "core"."parties" ("public_id");
-- Create "improvement_parties" table
CREATE TABLE "core"."improvement_parties" (
  "improvement_party_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "improvement_id" bigint NOT NULL,
  "party_id" bigint NOT NULL,
  "ownership_share" numeric(5,4) NOT NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("improvement_party_id"),
  CONSTRAINT "no_overlapping_improvement_parties" EXCLUDE USING GIST ("improvement_id" WITH =, "party_id" WITH =, "legal_valid_range" WITH &&),
  CONSTRAINT "fk_improvement_id" FOREIGN KEY ("improvement_id") REFERENCES "core"."improvements" ("improvement_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_party_id" FOREIGN KEY ("party_id") REFERENCES "core"."parties" ("party_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "valid_improvement_ownership" CHECK ((ownership_share > (0)::numeric) AND (ownership_share <= 1.0000))
);
-- Create index "idx_improvement_parties_improvement_id" to table: "improvement_parties"
CREATE INDEX "idx_improvement_parties_improvement_id" ON "core"."improvement_parties" ("improvement_id");
-- Create index "idx_improvement_parties_party_id" to table: "improvement_parties"
CREATE INDEX "idx_improvement_parties_party_id" ON "core"."improvement_parties" ("party_id");
-- Create trigger "record_improvement_parties_history"
CREATE TRIGGER "record_improvement_parties_history" BEFORE DELETE OR UPDATE ON "core"."improvement_parties" FOR EACH ROW EXECUTE FUNCTION "core"."record_improvement_parties_history"();
-- Create "improvement_parties_history" table
CREATE TABLE "core"."improvement_parties_history" (
  "improvement_party_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "improvement_party_id" bigint NOT NULL,
  "improvement_id" bigint NOT NULL,
  "party_id" bigint NOT NULL,
  "ownership_share" numeric(5,4) NOT NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("improvement_party_history_id")
);
-- Create index "idx_improvement_parties_history_improvement_id" to table: "improvement_parties_history"
CREATE INDEX "idx_improvement_parties_history_improvement_id" ON "core"."improvement_parties_history" ("improvement_id");
-- Create index "idx_improvement_parties_history_improvement_party_id" to table: "improvement_parties_history"
CREATE INDEX "idx_improvement_parties_history_improvement_party_id" ON "core"."improvement_parties_history" ("improvement_party_id");
-- Create index "idx_improvement_parties_history_party_id" to table: "improvement_parties_history"
CREATE INDEX "idx_improvement_parties_history_party_id" ON "core"."improvement_parties_history" ("party_id");
-- Create "zoning_history" table
CREATE TABLE "core"."zoning_history" (
  "zoning_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "zoning_id" bigint NOT NULL,
  "public_id" uuid NOT NULL,
  "legacy_id" text NULL,
  "is_voided" boolean NOT NULL,
  "voided_at" timestamptz NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("zoning_history_id")
);
-- Create index "idx_zoning_history_zoning_id" to table: "zoning_history"
CREATE INDEX "idx_zoning_history_zoning_id" ON "core"."zoning_history" ("zoning_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."zoning_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create trigger "record_improvement_qualities_history"
CREATE TRIGGER "record_improvement_qualities_history" BEFORE DELETE OR UPDATE ON "core"."improvement_qualities" FOR EACH ROW EXECUTE FUNCTION "core"."record_improvement_qualities_history"();
-- Create "improvement_qualities_history" table
CREATE TABLE "core"."improvement_qualities_history" (
  "improvement_quality_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "improvement_quality_id" bigint NOT NULL,
  "public_id" uuid NOT NULL,
  "legacy_id" text NULL,
  "is_voided" boolean NOT NULL,
  "voided_at" timestamptz NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("improvement_quality_history_id")
);
-- Create index "idx_improvement_qualities_history_improvement_quality_id" to table: "improvement_qualities_history"
CREATE INDEX "idx_improvement_qualities_history_improvement_quality_id" ON "core"."improvement_qualities_history" ("improvement_quality_id");
-- Create "improvement_quality_attributes" table
CREATE TABLE "core"."improvement_quality_attributes" (
  "improvement_quality_attribute_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "improvement_quality_id" bigint NOT NULL,
  "name" text NOT NULL,
  "quality_adjustment_modifier" double precision NOT NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("improvement_quality_attribute_id"),
  CONSTRAINT "no_overlapping_improvement_qualities" EXCLUDE USING GIST ("improvement_quality_id" WITH =, "legal_valid_range" WITH &&),
  CONSTRAINT "fk_improvement_quality_id" FOREIGN KEY ("improvement_quality_id") REFERENCES "core"."improvement_qualities" ("improvement_quality_id") ON UPDATE NO ACTION ON DELETE RESTRICT
);
-- Create trigger "record_improvement_quality_attributes_history"
CREATE TRIGGER "record_improvement_quality_attributes_history" BEFORE DELETE OR UPDATE ON "core"."improvement_quality_attributes" FOR EACH ROW EXECUTE FUNCTION "core"."record_improvement_quality_attributes_history"();
-- Create "improvement_quality_attributes_history" table
CREATE TABLE "core"."improvement_quality_attributes_history" (
  "improvement_quality_attribute_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "improvement_quality_attribute_id" bigint NOT NULL,
  "improvement_quality_id" bigint NOT NULL,
  "name" text NOT NULL,
  "quality_adjustment_modifier" double precision NOT NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("improvement_quality_attribute_history_id")
);
-- Create index "idx_impr_qual_attr_history_impr_qual_attr_id" to table: "improvement_quality_attributes_history"
CREATE INDEX "idx_impr_qual_attr_history_impr_qual_attr_id" ON "core"."improvement_quality_attributes_history" ("improvement_quality_attribute_id");
-- Create index "idx_impr_qual_attr_history_impr_qual_id" to table: "improvement_quality_attributes_history"
CREATE INDEX "idx_impr_qual_attr_history_impr_qual_id" ON "core"."improvement_quality_attributes_history" ("improvement_quality_id");
-- Create "zoning_attributes_history" table
CREATE TABLE "core"."zoning_attributes_history" (
  "zoning_attribute_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "zoning_attribute_id" bigint NOT NULL,
  "zoning_id" bigint NOT NULL,
  "name" text NOT NULL,
  "code" text NOT NULL,
  "max_far" numeric(6,2) NULL,
  "min_lot_size_sq_m" numeric(6,2) NULL,
  "max_height_m" numeric(6,2) NULL,
  "max_dwelling_units_per_hectare" numeric(6,2) NULL,
  "max_lot_coverage_pct" numeric(5,4) NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("zoning_attribute_history_id")
);
-- Create index "idx_zoning_attributes_history_zoning_attribute_id" to table: "zoning_attributes_history"
CREATE INDEX "idx_zoning_attributes_history_zoning_attribute_id" ON "core"."zoning_attributes_history" ("zoning_attribute_id");
-- Create index "idx_zoning_attributes_history_zoning_id" to table: "zoning_attributes_history"
CREATE INDEX "idx_zoning_attributes_history_zoning_id" ON "core"."zoning_attributes_history" ("zoning_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."zoning_attributes_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create "valuations" table
CREATE TABLE "core"."valuations" (
  "valuation_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "public_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "legacy_id" text NULL,
  "valuation_date" timestamptz NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("valuation_id")
);
-- Create index "idx_valuations_legacy_id" to table: "valuations"
CREATE UNIQUE INDEX "idx_valuations_legacy_id" ON "core"."valuations" ("legacy_id");
-- Create index "idx_valuations_public_id" to table: "valuations"
CREATE UNIQUE INDEX "idx_valuations_public_id" ON "core"."valuations" ("public_id");
-- Create "improvement_valuations" table
CREATE TABLE "core"."improvement_valuations" (
  "improvement_valuation_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "improvement_id" bigint NOT NULL,
  "valuation_id" bigint NOT NULL,
  "market_value" numeric(19,4) NOT NULL,
  "assessed_value" numeric(19,4) NOT NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("improvement_valuation_id"),
  CONSTRAINT "no_overlapping_improvement_valuations" EXCLUDE USING GIST ("improvement_id" WITH =, "legal_valid_range" WITH &&),
  CONSTRAINT "fk_improvement_id" FOREIGN KEY ("improvement_id") REFERENCES "core"."improvements" ("improvement_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_valuation_id" FOREIGN KEY ("valuation_id") REFERENCES "core"."valuations" ("valuation_id") ON UPDATE NO ACTION ON DELETE RESTRICT
);
-- Create index "idx_improvement_valuations_improvement_id" to table: "improvement_valuations"
CREATE INDEX "idx_improvement_valuations_improvement_id" ON "core"."improvement_valuations" ("improvement_id");
-- Create index "idx_improvement_valuations_improvement_id_valuation_id" to table: "improvement_valuations"
CREATE UNIQUE INDEX "idx_improvement_valuations_improvement_id_valuation_id" ON "core"."improvement_valuations" ("improvement_id", "valuation_id");
-- Create index "idx_improvement_valuations_valuation_id" to table: "improvement_valuations"
CREATE INDEX "idx_improvement_valuations_valuation_id" ON "core"."improvement_valuations" ("valuation_id");
-- Create trigger "record_improvement_valuations_history"
CREATE TRIGGER "record_improvement_valuations_history" BEFORE DELETE OR UPDATE ON "core"."improvement_valuations" FOR EACH ROW EXECUTE FUNCTION "core"."record_improvement_valuations_history"();
-- Create "improvement_valuations_history" table
CREATE TABLE "core"."improvement_valuations_history" (
  "improvement_valuation_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "improvement_valuation_id" bigint NOT NULL,
  "improvement_id" bigint NOT NULL,
  "valuation_id" bigint NOT NULL,
  "market_value" numeric(19,4) NOT NULL,
  "assessed_value" numeric(19,4) NOT NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("improvement_valuation_history_id")
);
-- Create index "idx_improvement_valuations_history_improvement_id" to table: "improvement_valuations_history"
CREATE INDEX "idx_improvement_valuations_history_improvement_id" ON "core"."improvement_valuations_history" ("improvement_id");
-- Create index "idx_improvement_valuations_history_improvement_valuation_id" to table: "improvement_valuations_history"
CREATE INDEX "idx_improvement_valuations_history_improvement_valuation_id" ON "core"."improvement_valuations_history" ("improvement_valuation_id");
-- Create index "idx_improvement_valuations_history_valuation_id" to table: "improvement_valuations_history"
CREATE INDEX "idx_improvement_valuations_history_valuation_id" ON "core"."improvement_valuations_history" ("valuation_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."improvement_valuations_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create "zoning" table
CREATE TABLE "core"."zoning" (
  "zoning_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "public_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "legacy_id" text NULL,
  "is_voided" boolean NOT NULL DEFAULT false,
  "voided_at" timestamptz NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("zoning_id"),
  CONSTRAINT "chk_voided_logic" CHECK (((is_voided = false) AND (voided_at IS NULL)) OR ((is_voided = true) AND (voided_at IS NOT NULL)))
);
-- Create index "idx_zoning_legacy_id" to table: "zoning"
CREATE UNIQUE INDEX "idx_zoning_legacy_id" ON "core"."zoning" ("legacy_id");
-- Create index "idx_zoning_public_id" to table: "zoning"
CREATE UNIQUE INDEX "idx_zoning_public_id" ON "core"."zoning" ("public_id");
-- Create "zoning_attributes" table
CREATE TABLE "core"."zoning_attributes" (
  "zoning_attribute_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "zoning_id" bigint NOT NULL,
  "name" text NOT NULL,
  "code" text NOT NULL,
  "max_far" numeric(6,2) NULL,
  "min_lot_size_sq_m" numeric(6,2) NULL,
  "max_height_m" numeric(6,2) NULL,
  "max_dwelling_units_per_hectare" numeric(6,2) NULL,
  "max_lot_coverage_pct" numeric(5,4) NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("zoning_attribute_id"),
  CONSTRAINT "no_overlapping_zoning_attributes" EXCLUDE USING GIST ("zoning_id" WITH =, "legal_valid_range" WITH &&),
  CONSTRAINT "fk_zoning_id" FOREIGN KEY ("zoning_id") REFERENCES "core"."zoning" ("zoning_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "valid_max_lot_coverage" CHECK ((max_lot_coverage_pct > (0)::numeric) AND (max_lot_coverage_pct <= 1.0000))
);
-- Create index "idx_zoning_attributes_zoning_id" to table: "zoning_attributes"
CREATE INDEX "idx_zoning_attributes_zoning_id" ON "core"."zoning_attributes" ("zoning_id");
-- Create trigger "record_zoning_attributes_history"
CREATE TRIGGER "record_zoning_attributes_history" BEFORE DELETE OR UPDATE ON "core"."zoning_attributes" FOR EACH ROW EXECUTE FUNCTION "core"."record_zoning_attributes_history"();
-- Create trigger "record_improvements_history"
CREATE TRIGGER "record_improvements_history" BEFORE DELETE OR UPDATE ON "core"."improvements" FOR EACH ROW EXECUTE FUNCTION "core"."record_improvements_history"();
-- Create "improvements_history" table
CREATE TABLE "core"."improvements_history" (
  "improvement_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "improvement_id" bigint NOT NULL,
  "public_id" uuid NOT NULL,
  "legacy_id" text NULL,
  "is_voided" boolean NOT NULL,
  "voided_at" timestamptz NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("improvement_history_id")
);
-- Create index "idx_improvements_history_improvement_id" to table: "improvements_history"
CREATE INDEX "idx_improvements_history_improvement_id" ON "core"."improvements_history" ("improvement_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."improvements_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create trigger "record_zoning_history"
CREATE TRIGGER "record_zoning_history" BEFORE DELETE OR UPDATE ON "core"."zoning" FOR EACH ROW EXECUTE FUNCTION "core"."record_zoning_history"();
-- Create "valuations_history" table
CREATE TABLE "core"."valuations_history" (
  "valuation_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "valuation_id" bigint NOT NULL,
  "public_id" uuid NOT NULL,
  "legacy_id" text NULL,
  "valuation_date" timestamptz NOT NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("valuation_history_id")
);
-- Create index "idx_valuations_history_valuation_id" to table: "valuations_history"
CREATE INDEX "idx_valuations_history_valuation_id" ON "core"."valuations_history" ("valuation_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."valuations_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create "neighborhood_definitions" table
CREATE TABLE "core"."neighborhood_definitions" (
  "neighborhood_definition_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "public_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "legacy_id" text NULL,
  "name" text NOT NULL,
  "is_legal" boolean NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("neighborhood_definition_id")
);
-- Create index "idx_neighborhood_definitions_id_is_legal" to table: "neighborhood_definitions"
CREATE UNIQUE INDEX "idx_neighborhood_definitions_id_is_legal" ON "core"."neighborhood_definitions" ("neighborhood_definition_id", "is_legal");
-- Create index "idx_neighborhood_definitions_legacy_id" to table: "neighborhood_definitions"
CREATE UNIQUE INDEX "idx_neighborhood_definitions_legacy_id" ON "core"."neighborhood_definitions" ("legacy_id");
-- Create index "idx_neighborhood_definitions_name" to table: "neighborhood_definitions"
CREATE UNIQUE INDEX "idx_neighborhood_definitions_name" ON "core"."neighborhood_definitions" ("name");
-- Create index "idx_neighborhood_definitions_public_id" to table: "neighborhood_definitions"
CREATE UNIQUE INDEX "idx_neighborhood_definitions_public_id" ON "core"."neighborhood_definitions" ("public_id");
-- Create trigger "record_neighborhood_definitions_history"
CREATE TRIGGER "record_neighborhood_definitions_history" BEFORE DELETE OR UPDATE ON "core"."neighborhood_definitions" FOR EACH ROW EXECUTE FUNCTION "core"."record_neighborhood_definitions_history"();
-- Create trigger "record_valuations_history"
CREATE TRIGGER "record_valuations_history" BEFORE DELETE OR UPDATE ON "core"."valuations" FOR EACH ROW EXECUTE FUNCTION "core"."record_valuations_history"();
-- Create "system_settings_history" table
CREATE TABLE "core"."system_settings_history" (
  "system_setting_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "system_setting_id" integer NOT NULL,
  "base_currency" character(3) NOT NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("system_setting_history_id")
);
-- Create "neighborhoods" table
CREATE TABLE "core"."neighborhoods" (
  "neighborhood_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "public_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "legacy_id" text NULL,
  "name" text NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("neighborhood_id")
);
-- Create index "idx_neighborhoods_legacy_id" to table: "neighborhoods"
CREATE UNIQUE INDEX "idx_neighborhoods_legacy_id" ON "core"."neighborhoods" ("legacy_id");
-- Create index "idx_neighborhoods_name" to table: "neighborhoods"
CREATE UNIQUE INDEX "idx_neighborhoods_name" ON "core"."neighborhoods" ("name");
-- Create index "idx_neighborhoods_public_id" to table: "neighborhoods"
CREATE UNIQUE INDEX "idx_neighborhoods_public_id" ON "core"."neighborhoods" ("public_id");
-- Create trigger "record_neighborhoods_history"
CREATE TRIGGER "record_neighborhoods_history" BEFORE DELETE OR UPDATE ON "core"."neighborhoods" FOR EACH ROW EXECUTE FUNCTION "core"."record_neighborhoods_history"();
-- Create "neighborhoods_history" table
CREATE TABLE "core"."neighborhoods_history" (
  "neighborhood_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "neighborhood_id" bigint NOT NULL,
  "public_id" uuid NOT NULL,
  "legacy_id" text NULL,
  "name" text NOT NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("neighborhood_history_id")
);
-- Create index "idx_neighborhoods_history_neighborhood_id" to table: "neighborhoods_history"
CREATE INDEX "idx_neighborhoods_history_neighborhood_id" ON "core"."neighborhoods_history" ("neighborhood_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."neighborhoods_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create "system_settings" table
CREATE TABLE "core"."system_settings" (
  "system_setting_id" integer NOT NULL DEFAULT 1,
  "base_currency" character(3) NOT NULL DEFAULT 'USD',
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("system_setting_id"),
  CONSTRAINT "enforce_single_row" CHECK (system_setting_id = 1)
);
-- Create trigger "record_system_settings_history"
CREATE TRIGGER "record_system_settings_history" BEFORE DELETE OR UPDATE ON "core"."system_settings" FOR EACH ROW EXECUTE FUNCTION "core"."record_system_settings_history"();
-- Create "affordance_types" table
CREATE TABLE "core"."affordance_types" (
  "affordance_type_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "public_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "name" text NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("affordance_type_id")
);
-- Create index "idx_affordance_types_public_id" to table: "affordance_types"
CREATE UNIQUE INDEX "idx_affordance_types_public_id" ON "core"."affordance_types" ("public_id");
-- Create "parcel_affordances" table
CREATE TABLE "core"."parcel_affordances" (
  "parcel_affordance_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "public_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "parcel_id" bigint NOT NULL,
  "zoning_id" bigint NULL,
  "affordance_type_id" bigint NOT NULL,
  "source" text NULL,
  "precedence_rank" integer NOT NULL DEFAULT 100,
  "max_far" numeric(6,2) NULL,
  "min_lot_size_sq_m" numeric(6,2) NULL,
  "max_height_m" numeric(6,2) NULL,
  "max_dwelling_units_per_hectare" numeric(6,2) NULL,
  "max_lot_coverage_pct" numeric(5,4) NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("parcel_affordance_id"),
  CONSTRAINT "no_overlapping_zoning_affordance" EXCLUDE USING GIST ("parcel_id" WITH =, "zoning_id" WITH =, "legal_valid_range" WITH &&) WHERE (zoning_id IS NOT NULL),
  CONSTRAINT "fk_affordance_type_id" FOREIGN KEY ("affordance_type_id") REFERENCES "core"."affordance_types" ("affordance_type_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_parcel_id" FOREIGN KEY ("parcel_id") REFERENCES "core"."parcels" ("parcel_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_zoning_id" FOREIGN KEY ("zoning_id") REFERENCES "core"."zoning" ("zoning_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "chk_require_origin" CHECK ((zoning_id IS NOT NULL) OR (affordance_type_id <> 1)),
  CONSTRAINT "valid_max_lot_coverage" CHECK ((max_lot_coverage_pct > (0)::numeric) AND (max_lot_coverage_pct <= 1.0000))
);
-- Create index "idx_parcel_affordances_parcel_id" to table: "parcel_affordances"
CREATE INDEX "idx_parcel_affordances_parcel_id" ON "core"."parcel_affordances" ("parcel_id");
-- Create index "idx_parcel_affordances_public_id" to table: "parcel_affordances"
CREATE UNIQUE INDEX "idx_parcel_affordances_public_id" ON "core"."parcel_affordances" ("public_id");
-- Create trigger "record_parcel_affordances_history"
CREATE TRIGGER "record_parcel_affordances_history" BEFORE DELETE OR UPDATE ON "core"."parcel_affordances" FOR EACH ROW EXECUTE FUNCTION "core"."record_parcel_affordances_history"();
-- Create "parcel_affordances_history" table
CREATE TABLE "core"."parcel_affordances_history" (
  "parcel_affordance_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "parcel_affordance_id" bigint NOT NULL,
  "public_id" uuid NOT NULL,
  "parcel_id" bigint NOT NULL,
  "zoning_id" bigint NULL,
  "affordance_type_id" bigint NOT NULL,
  "source" text NULL,
  "precedence_rank" integer NOT NULL,
  "max_far" numeric(6,2) NULL,
  "min_lot_size_sq_m" numeric(6,2) NULL,
  "max_height_m" numeric(6,2) NULL,
  "max_dwelling_units_per_hectare" numeric(6,2) NULL,
  "max_lot_coverage_pct" numeric(5,4) NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("parcel_affordance_history_id")
);
-- Create index "idx_parcel_affordances_history_parcel_affordance_id" to table: "parcel_affordances_history"
CREATE INDEX "idx_parcel_affordances_history_parcel_affordance_id" ON "core"."parcel_affordances_history" ("parcel_affordance_id");
-- Create index "idx_parcel_affordances_history_parcel_id" to table: "parcel_affordances_history"
CREATE INDEX "idx_parcel_affordances_history_parcel_id" ON "core"."parcel_affordances_history" ("parcel_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."parcel_affordances_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create "land_use_types" table
CREATE TABLE "core"."land_use_types" (
  "land_use_type_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "public_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "legacy_id" text NULL,
  "code" text NULL,
  "name" text NOT NULL,
  "description" text NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("land_use_type_id")
);
-- Create index "idx_land_use_types_legacy_id" to table: "land_use_types"
CREATE UNIQUE INDEX "idx_land_use_types_legacy_id" ON "core"."land_use_types" ("legacy_id");
-- Create index "idx_land_use_types_name" to table: "land_use_types"
CREATE UNIQUE INDEX "idx_land_use_types_name" ON "core"."land_use_types" ("name");
-- Create index "idx_land_use_types_public_id" to table: "land_use_types"
CREATE UNIQUE INDEX "idx_land_use_types_public_id" ON "core"."land_use_types" ("public_id");
-- Create "land_uses" table
CREATE TABLE "core"."land_uses" (
  "land_use_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "public_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "legacy_id" text NULL,
  "land_use_type_id" bigint NULL,
  "code" text NULL,
  "name" text NOT NULL,
  "description" text NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("land_use_id"),
  CONSTRAINT "fk_land_use_type_id" FOREIGN KEY ("land_use_type_id") REFERENCES "core"."land_use_types" ("land_use_type_id") ON UPDATE NO ACTION ON DELETE RESTRICT
);
-- Create index "idx_land_uses_legacy_id" to table: "land_uses"
CREATE UNIQUE INDEX "idx_land_uses_legacy_id" ON "core"."land_uses" ("legacy_id");
-- Create index "idx_land_uses_name" to table: "land_uses"
CREATE UNIQUE INDEX "idx_land_uses_name" ON "core"."land_uses" ("name");
-- Create index "idx_land_uses_public_id" to table: "land_uses"
CREATE UNIQUE INDEX "idx_land_uses_public_id" ON "core"."land_uses" ("public_id");
-- Create "parcel_attributes" table
CREATE TABLE "core"."parcel_attributes" (
  "parcel_attribute_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "parcel_id" bigint NOT NULL,
  "address_id" bigint NOT NULL,
  "land_use_id" bigint NOT NULL,
  "land_area_sq_m" double precision NULL,
  "frontage_m" double precision NULL,
  "depth_m" double precision NULL,
  "properties" jsonb NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("parcel_attribute_id"),
  CONSTRAINT "no_overlapping_parcel_attributes" EXCLUDE USING GIST ("parcel_id" WITH =, "legal_valid_range" WITH &&),
  CONSTRAINT "fk_address_id" FOREIGN KEY ("address_id") REFERENCES "core"."addresses" ("address_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_land_use_id" FOREIGN KEY ("land_use_id") REFERENCES "core"."land_uses" ("land_use_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_parcel_id" FOREIGN KEY ("parcel_id") REFERENCES "core"."parcels" ("parcel_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "properties_size_limit" CHECK (pg_column_size(properties) <= 5000)
);
-- Create index "idx_parcel_attributes_address_id" to table: "parcel_attributes"
CREATE INDEX "idx_parcel_attributes_address_id" ON "core"."parcel_attributes" ("address_id");
-- Create index "idx_parcel_attributes_land_use_id" to table: "parcel_attributes"
CREATE INDEX "idx_parcel_attributes_land_use_id" ON "core"."parcel_attributes" ("land_use_id");
-- Create index "idx_parcel_attributes_parcel_id" to table: "parcel_attributes"
CREATE INDEX "idx_parcel_attributes_parcel_id" ON "core"."parcel_attributes" ("parcel_id");
-- Create trigger "record_parcel_attributes_history"
CREATE TRIGGER "record_parcel_attributes_history" BEFORE DELETE OR UPDATE ON "core"."parcel_attributes" FOR EACH ROW EXECUTE FUNCTION "core"."record_parcel_attributes_history"();
-- Create "parcel_attributes_history" table
CREATE TABLE "core"."parcel_attributes_history" (
  "parcel_attribute_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "parcel_attribute_id" bigint NOT NULL,
  "parcel_id" bigint NOT NULL,
  "address_id" bigint NOT NULL,
  "land_area_sq_m" double precision NULL,
  "frontage_m" double precision NULL,
  "depth_m" double precision NULL,
  "land_use_id" bigint NOT NULL,
  "properties" jsonb NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("parcel_attribute_history_id")
);
-- Create index "idx_parcel_attributes_history_parcel_attribute_id" to table: "parcel_attributes_history"
CREATE INDEX "idx_parcel_attributes_history_parcel_attribute_id" ON "core"."parcel_attributes_history" ("parcel_attribute_id");
-- Create index "idx_parcel_attributes_history_parcel_id" to table: "parcel_attributes_history"
CREATE INDEX "idx_parcel_attributes_history_parcel_id" ON "core"."parcel_attributes_history" ("parcel_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."parcel_attributes_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create "real_property_transfers_history" table
CREATE TABLE "core"."real_property_transfers_history" (
  "real_property_transfer_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "real_property_transfer_id" bigint NOT NULL,
  "public_id" uuid NOT NULL,
  "legacy_id" text NULL,
  "transfer_timestamp" timestamptz NOT NULL,
  "transfer_amount" numeric(19,4) NULL,
  "deed_book" text NULL,
  "deed_page" text NULL,
  "deed_uri" text NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("real_property_transfer_history_id")
);
-- Create index "idx_real_property_transfers_history_real_property_transfer_id" to table: "real_property_transfers_history"
CREATE INDEX "idx_real_property_transfers_history_real_property_transfer_id" ON "core"."real_property_transfers_history" ("real_property_transfer_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."real_property_transfers_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create trigger "record_parcel_improvements_history"
CREATE TRIGGER "record_parcel_improvements_history" BEFORE DELETE OR UPDATE ON "core"."parcel_improvements" FOR EACH ROW EXECUTE FUNCTION "core"."record_parcel_improvements_history"();
-- Create "parcel_improvements_history" table
CREATE TABLE "core"."parcel_improvements_history" (
  "parcel_improvement_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "parcel_improvement_id" bigint NOT NULL,
  "parcel_id" bigint NOT NULL,
  "improvement_id" bigint NOT NULL,
  "is_primary" boolean NOT NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("parcel_improvement_history_id")
);
-- Create index "idx_parcel_improvements_history_improvement_id" to table: "parcel_improvements_history"
CREATE INDEX "idx_parcel_improvements_history_improvement_id" ON "core"."parcel_improvements_history" ("improvement_id");
-- Create index "idx_parcel_improvements_history_parcel_id" to table: "parcel_improvements_history"
CREATE INDEX "idx_parcel_improvements_history_parcel_id" ON "core"."parcel_improvements_history" ("parcel_id");
-- Create index "idx_parcel_improvements_history_parcel_improvement_id" to table: "parcel_improvements_history"
CREATE INDEX "idx_parcel_improvements_history_parcel_improvement_id" ON "core"."parcel_improvements_history" ("parcel_improvement_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."parcel_improvements_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create "parcel_neighborhood_definitions" table
CREATE TABLE "core"."parcel_neighborhood_definitions" (
  "parcel_neighborhood_definition_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "parcel_id" bigint NOT NULL,
  "neighborhood_id" bigint NOT NULL,
  "neighborhood_definition_id" bigint NOT NULL,
  "is_legal" boolean NOT NULL,
  "legal_valid_range" tstzrange NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("parcel_neighborhood_definition_id"),
  CONSTRAINT "no_overlapping_legal_parcel_neighborhood_definitions" EXCLUDE USING GIST ("parcel_id" WITH =, "neighborhood_definition_id" WITH =, "legal_valid_range" WITH &&) WHERE (is_legal = true),
  CONSTRAINT "fk_neighborhood_definition_id" FOREIGN KEY ("neighborhood_definition_id", "is_legal") REFERENCES "core"."neighborhood_definitions" ("neighborhood_definition_id", "is_legal") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_neighborhood_id" FOREIGN KEY ("neighborhood_id") REFERENCES "core"."neighborhoods" ("neighborhood_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_parcel_id" FOREIGN KEY ("parcel_id") REFERENCES "core"."parcels" ("parcel_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "chk_legal_must_have_time" CHECK ((is_legal = false) OR ((is_legal = true) AND (legal_valid_range IS NOT NULL)))
);
-- Create index "idx_parcel_neighborhood_definitions_definition_id" to table: "parcel_neighborhood_definitions"
CREATE INDEX "idx_parcel_neighborhood_definitions_definition_id" ON "core"."parcel_neighborhood_definitions" ("neighborhood_definition_id");
-- Create index "idx_parcel_neighborhood_definitions_neighborhood_id" to table: "parcel_neighborhood_definitions"
CREATE INDEX "idx_parcel_neighborhood_definitions_neighborhood_id" ON "core"."parcel_neighborhood_definitions" ("neighborhood_id");
-- Create index "idx_parcel_neighborhood_definitions_parcel_id" to table: "parcel_neighborhood_definitions"
CREATE INDEX "idx_parcel_neighborhood_definitions_parcel_id" ON "core"."parcel_neighborhood_definitions" ("parcel_id");
-- Create index "idx_unique_non_legal_parcel_neighborhood" to table: "parcel_neighborhood_definitions"
CREATE UNIQUE INDEX "idx_unique_non_legal_parcel_neighborhood" ON "core"."parcel_neighborhood_definitions" ("parcel_id", "neighborhood_definition_id") WHERE (is_legal = false);
-- Create trigger "record_parcel_neighborhood_definitions_history"
CREATE TRIGGER "record_parcel_neighborhood_definitions_history" BEFORE DELETE OR UPDATE ON "core"."parcel_neighborhood_definitions" FOR EACH ROW EXECUTE FUNCTION "core"."record_parcel_neighborhood_definitions_history"();
-- Create "parcel_neighborhood_definitions_history" table
CREATE TABLE "core"."parcel_neighborhood_definitions_history" (
  "parcel_neighborhood_definition_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "parcel_neighborhood_definition_id" bigint NOT NULL,
  "parcel_id" bigint NOT NULL,
  "neighborhood_id" bigint NOT NULL,
  "neighborhood_definition_id" bigint NOT NULL,
  "is_legal" boolean NOT NULL,
  "legal_valid_range" tstzrange NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("parcel_neighborhood_definition_history_id")
);
-- Create index "idx_parcel_neighborhood_definitions_history_neighborhood_defini" to table: "parcel_neighborhood_definitions_history"
CREATE INDEX "idx_parcel_neighborhood_definitions_history_neighborhood_defini" ON "core"."parcel_neighborhood_definitions_history" ("neighborhood_definition_id");
-- Create index "idx_parcel_neighborhood_definitions_history_neighborhood_id" to table: "parcel_neighborhood_definitions_history"
CREATE INDEX "idx_parcel_neighborhood_definitions_history_neighborhood_id" ON "core"."parcel_neighborhood_definitions_history" ("neighborhood_id");
-- Create index "idx_parcel_neighborhood_definitions_history_parcel_id" to table: "parcel_neighborhood_definitions_history"
CREATE INDEX "idx_parcel_neighborhood_definitions_history_parcel_id" ON "core"."parcel_neighborhood_definitions_history" ("parcel_id");
-- Create index "idx_parcel_neighborhood_definitions_history_parcel_neighborhood" to table: "parcel_neighborhood_definitions_history"
CREATE INDEX "idx_parcel_neighborhood_definitions_history_parcel_neighborhood" ON "core"."parcel_neighborhood_definitions_history" ("parcel_neighborhood_definition_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."parcel_neighborhood_definitions_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create "real_property_transfers" table
CREATE TABLE "core"."real_property_transfers" (
  "real_property_transfer_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "public_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "legacy_id" text NULL,
  "transfer_timestamp" timestamptz NOT NULL,
  "transfer_amount" numeric(19,4) NULL,
  "deed_book" text NULL,
  "deed_page" text NULL,
  "deed_uri" text NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("real_property_transfer_id")
);
-- Create index "idx_real_property_transfers_legacy_id" to table: "real_property_transfers"
CREATE UNIQUE INDEX "idx_real_property_transfers_legacy_id" ON "core"."real_property_transfers" ("legacy_id");
-- Create index "idx_real_property_transfers_public_id" to table: "real_property_transfers"
CREATE UNIQUE INDEX "idx_real_property_transfers_public_id" ON "core"."real_property_transfers" ("public_id");
-- Create trigger "record_real_property_transfers_history"
CREATE TRIGGER "record_real_property_transfers_history" BEFORE DELETE OR UPDATE ON "core"."real_property_transfers" FOR EACH ROW EXECUTE FUNCTION "core"."record_real_property_transfers_history"();
-- Create "parcel_parties" table
CREATE TABLE "core"."parcel_parties" (
  "parcel_party_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "parcel_id" bigint NOT NULL,
  "party_id" bigint NOT NULL,
  "ownership_share" numeric(5,4) NOT NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("parcel_party_id"),
  CONSTRAINT "no_overlapping_parcel_parties" EXCLUDE USING GIST ("parcel_id" WITH =, "party_id" WITH =, "legal_valid_range" WITH &&),
  CONSTRAINT "fk_parcel_id" FOREIGN KEY ("parcel_id") REFERENCES "core"."parcels" ("parcel_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_party_id" FOREIGN KEY ("party_id") REFERENCES "core"."parties" ("party_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "valid_parcel_ownership" CHECK ((ownership_share > (0)::numeric) AND (ownership_share <= 1.0000))
);
-- Create index "idx_parcel_parties_parcel_id" to table: "parcel_parties"
CREATE INDEX "idx_parcel_parties_parcel_id" ON "core"."parcel_parties" ("parcel_id");
-- Create index "idx_parcel_parties_party_id" to table: "parcel_parties"
CREATE INDEX "idx_parcel_parties_party_id" ON "core"."parcel_parties" ("party_id");
-- Create trigger "record_parcel_parties_history"
CREATE TRIGGER "record_parcel_parties_history" BEFORE DELETE OR UPDATE ON "core"."parcel_parties" FOR EACH ROW EXECUTE FUNCTION "core"."record_parcel_parties_history"();
-- Create "parcel_parties_history" table
CREATE TABLE "core"."parcel_parties_history" (
  "parcel_party_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "parcel_party_id" bigint NOT NULL,
  "parcel_id" bigint NOT NULL,
  "party_id" bigint NOT NULL,
  "ownership_share" numeric(5,4) NOT NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("parcel_party_history_id")
);
-- Create index "idx_parcel_parties_history_parcel_id" to table: "parcel_parties_history"
CREATE INDEX "idx_parcel_parties_history_parcel_id" ON "core"."parcel_parties_history" ("parcel_id");
-- Create index "idx_parcel_parties_history_parcel_party_id" to table: "parcel_parties_history"
CREATE INDEX "idx_parcel_parties_history_parcel_party_id" ON "core"."parcel_parties_history" ("parcel_party_id");
-- Create index "idx_parcel_parties_history_party_id" to table: "parcel_parties_history"
CREATE INDEX "idx_parcel_parties_history_party_id" ON "core"."parcel_parties_history" ("party_id");
-- Create "real_property_transfer_party_parcels_history" table
CREATE TABLE "core"."real_property_transfer_party_parcels_history" (
  "real_property_transfer_party_parcel_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "real_property_transfer_party_parcel_id" bigint NOT NULL,
  "parcel_id" bigint NOT NULL,
  "real_property_transfer_id" bigint NOT NULL,
  "party_id" bigint NOT NULL,
  "is_grantor" boolean NOT NULL,
  "transferred_share" numeric(5,4) NOT NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("real_property_transfer_party_parcel_history_id")
);
-- Create index "idx_rpt_party_parcels_hist_id" to table: "real_property_transfer_party_parcels_history"
CREATE INDEX "idx_rpt_party_parcels_hist_id" ON "core"."real_property_transfer_party_parcels_history" ("real_property_transfer_party_parcel_id");
-- Create index "idx_rpt_party_parcels_hist_parcel_id" to table: "real_property_transfer_party_parcels_history"
CREATE INDEX "idx_rpt_party_parcels_hist_parcel_id" ON "core"."real_property_transfer_party_parcels_history" ("parcel_id");
-- Create index "idx_rpt_party_parcels_hist_party_id" to table: "real_property_transfer_party_parcels_history"
CREATE INDEX "idx_rpt_party_parcels_hist_party_id" ON "core"."real_property_transfer_party_parcels_history" ("party_id");
-- Create index "idx_rpt_party_parcels_hist_transfer_id" to table: "real_property_transfer_party_parcels_history"
CREATE INDEX "idx_rpt_party_parcels_hist_transfer_id" ON "core"."real_property_transfer_party_parcels_history" ("real_property_transfer_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."real_property_transfer_party_parcels_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create "parcel_valuations" table
CREATE TABLE "core"."parcel_valuations" (
  "parcel_valuation_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "parcel_id" bigint NOT NULL,
  "valuation_id" bigint NOT NULL,
  "market_value" numeric(19,4) NOT NULL,
  "assessed_value" numeric(19,4) NOT NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("parcel_valuation_id"),
  CONSTRAINT "no_overlapping_parcel_valuations" EXCLUDE USING GIST ("parcel_id" WITH =, "legal_valid_range" WITH &&),
  CONSTRAINT "fk_parcel_id" FOREIGN KEY ("parcel_id") REFERENCES "core"."parcels" ("parcel_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_valuation_id" FOREIGN KEY ("valuation_id") REFERENCES "core"."valuations" ("valuation_id") ON UPDATE NO ACTION ON DELETE RESTRICT
);
-- Create index "idx_parcel_valuations_parcel_id" to table: "parcel_valuations"
CREATE INDEX "idx_parcel_valuations_parcel_id" ON "core"."parcel_valuations" ("parcel_id");
-- Create index "idx_parcel_valuations_parcel_id_valuation_id" to table: "parcel_valuations"
CREATE UNIQUE INDEX "idx_parcel_valuations_parcel_id_valuation_id" ON "core"."parcel_valuations" ("parcel_id", "valuation_id");
-- Create index "idx_parcel_valuations_valuation_id" to table: "parcel_valuations"
CREATE INDEX "idx_parcel_valuations_valuation_id" ON "core"."parcel_valuations" ("valuation_id");
-- Create trigger "record_parcel_valuations_history"
CREATE TRIGGER "record_parcel_valuations_history" BEFORE DELETE OR UPDATE ON "core"."parcel_valuations" FOR EACH ROW EXECUTE FUNCTION "core"."record_parcel_valuations_history"();
-- Create "parcel_valuations_history" table
CREATE TABLE "core"."parcel_valuations_history" (
  "parcel_valuation_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "parcel_valuation_id" bigint NOT NULL,
  "parcel_id" bigint NOT NULL,
  "valuation_id" bigint NOT NULL,
  "market_value" numeric(19,4) NOT NULL,
  "assessed_value" numeric(19,4) NOT NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("parcel_valuation_history_id")
);
-- Create index "idx_parcel_valuations_history_parcel_id" to table: "parcel_valuations_history"
CREATE INDEX "idx_parcel_valuations_history_parcel_id" ON "core"."parcel_valuations_history" ("parcel_id");
-- Create index "idx_parcel_valuations_history_parcel_valuation_id" to table: "parcel_valuations_history"
CREATE INDEX "idx_parcel_valuations_history_parcel_valuation_id" ON "core"."parcel_valuations_history" ("parcel_valuation_id");
-- Create index "idx_parcel_valuations_history_valuation_id" to table: "parcel_valuations_history"
CREATE INDEX "idx_parcel_valuations_history_valuation_id" ON "core"."parcel_valuations_history" ("valuation_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."parcel_valuations_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create trigger "record_parcels_history"
CREATE TRIGGER "record_parcels_history" BEFORE DELETE OR UPDATE ON "core"."parcels" FOR EACH ROW EXECUTE FUNCTION "core"."record_parcels_history"();
-- Create "parcels_history" table
CREATE TABLE "core"."parcels_history" (
  "parcel_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "parcel_id" bigint NOT NULL,
  "public_id" uuid NOT NULL,
  "legacy_id" text NULL,
  "is_voided" boolean NOT NULL,
  "voided_at" timestamptz NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("parcel_history_id")
);
-- Create index "idx_parcels_history_parcel_id" to table: "parcels_history"
CREATE INDEX "idx_parcels_history_parcel_id" ON "core"."parcels_history" ("parcel_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."parcels_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create "real_property_transfer_party_parcels" table
CREATE TABLE "core"."real_property_transfer_party_parcels" (
  "real_property_transfer_party_parcel_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "parcel_id" bigint NOT NULL,
  "real_property_transfer_id" bigint NOT NULL,
  "party_id" bigint NOT NULL,
  "is_grantor" boolean NOT NULL,
  "transferred_share" numeric(5,4) NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("real_property_transfer_party_parcel_id"),
  CONSTRAINT "fk_parcel_id" FOREIGN KEY ("parcel_id") REFERENCES "core"."parcels" ("parcel_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_party_id" FOREIGN KEY ("party_id") REFERENCES "core"."parties" ("party_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_real_property_transfer_id" FOREIGN KEY ("real_property_transfer_id") REFERENCES "core"."real_property_transfers" ("real_property_transfer_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "valid_share_transfer" CHECK ((transferred_share > (0)::numeric) AND (transferred_share <= 1.0000))
);
-- Create index "idx_rpt_party_parcels_parcel_id" to table: "real_property_transfer_party_parcels"
CREATE INDEX "idx_rpt_party_parcels_parcel_id" ON "core"."real_property_transfer_party_parcels" ("parcel_id");
-- Create index "idx_rpt_party_parcels_party_id" to table: "real_property_transfer_party_parcels"
CREATE INDEX "idx_rpt_party_parcels_party_id" ON "core"."real_property_transfer_party_parcels" ("party_id");
-- Create index "idx_rpt_party_parcels_rpt_id" to table: "real_property_transfer_party_parcels"
CREATE INDEX "idx_rpt_party_parcels_rpt_id" ON "core"."real_property_transfer_party_parcels" ("real_property_transfer_id");
-- Create index "idx_rpt_party_parcels_unique_party" to table: "real_property_transfer_party_parcels"
CREATE UNIQUE INDEX "idx_rpt_party_parcels_unique_party" ON "core"."real_property_transfer_party_parcels" ("parcel_id", "real_property_transfer_id", "party_id");
-- Create trigger "record_real_property_transfer_party_parcels_history"
CREATE TRIGGER "record_real_property_transfer_party_parcels_history" BEFORE DELETE OR UPDATE ON "core"."real_property_transfer_party_parcels" FOR EACH ROW EXECUTE FUNCTION "core"."record_real_property_transfer_party_parcels_history"();
-- Create trigger "record_parties_history"
CREATE TRIGGER "record_parties_history" BEFORE DELETE OR UPDATE ON "core"."parties" FOR EACH ROW EXECUTE FUNCTION "core"."record_parties_history"();
-- Create "parties_history" table
CREATE TABLE "core"."parties_history" (
  "party_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "party_id" bigint NOT NULL,
  "public_id" uuid NOT NULL,
  "legacy_id" text NULL,
  "is_voided" boolean NOT NULL,
  "voided_at" timestamptz NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("party_history_id")
);
-- Create index "idx_parties_history_party_id" to table: "parties_history"
CREATE INDEX "idx_parties_history_party_id" ON "core"."parties_history" ("party_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."parties_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create "real_property_transfer_party_improvements_history" table
CREATE TABLE "core"."real_property_transfer_party_improvements_history" (
  "real_property_transfer_party_improvement_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "real_property_transfer_party_improvement_id" bigint NOT NULL,
  "improvement_id" bigint NOT NULL,
  "real_property_transfer_id" bigint NOT NULL,
  "party_id" bigint NOT NULL,
  "is_grantor" boolean NOT NULL,
  "transferred_share" numeric(5,4) NOT NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("real_property_transfer_party_improvement_history_id")
);
-- Create index "idx_rpt_party_improvements_hist_id" to table: "real_property_transfer_party_improvements_history"
CREATE INDEX "idx_rpt_party_improvements_hist_id" ON "core"."real_property_transfer_party_improvements_history" ("real_property_transfer_party_improvement_id");
-- Create index "idx_rpt_party_improvements_hist_improvement_id" to table: "real_property_transfer_party_improvements_history"
CREATE INDEX "idx_rpt_party_improvements_hist_improvement_id" ON "core"."real_property_transfer_party_improvements_history" ("improvement_id");
-- Create index "idx_rpt_party_improvements_hist_party_id" to table: "real_property_transfer_party_improvements_history"
CREATE INDEX "idx_rpt_party_improvements_hist_party_id" ON "core"."real_property_transfer_party_improvements_history" ("party_id");
-- Create index "idx_rpt_party_improvements_hist_transfer_id" to table: "real_property_transfer_party_improvements_history"
CREATE INDEX "idx_rpt_party_improvements_hist_transfer_id" ON "core"."real_property_transfer_party_improvements_history" ("real_property_transfer_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."real_property_transfer_party_improvements_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create "party_attributes" table
CREATE TABLE "core"."party_attributes" (
  "party_attribute_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "party_id" bigint NOT NULL,
  "name" text NOT NULL,
  "address_id" bigint NOT NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("party_attribute_id"),
  CONSTRAINT "no_overlapping_party_attributes" EXCLUDE USING GIST ("party_id" WITH =, "legal_valid_range" WITH &&),
  CONSTRAINT "fk_address_id" FOREIGN KEY ("address_id") REFERENCES "core"."addresses" ("address_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_party_id" FOREIGN KEY ("party_id") REFERENCES "core"."parties" ("party_id") ON UPDATE NO ACTION ON DELETE RESTRICT
);
-- Create index "idx_party_attributes_address_id" to table: "party_attributes"
CREATE INDEX "idx_party_attributes_address_id" ON "core"."party_attributes" ("address_id");
-- Create index "idx_party_attributes_party_id" to table: "party_attributes"
CREATE INDEX "idx_party_attributes_party_id" ON "core"."party_attributes" ("party_id");
-- Create trigger "record_party_attributes_history"
CREATE TRIGGER "record_party_attributes_history" BEFORE DELETE OR UPDATE ON "core"."party_attributes" FOR EACH ROW EXECUTE FUNCTION "core"."record_party_attributes_history"();
-- Create "party_attributes_history" table
CREATE TABLE "core"."party_attributes_history" (
  "party_attribute_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "party_attribute_id" bigint NOT NULL,
  "party_id" bigint NOT NULL,
  "name" text NOT NULL,
  "address_id" bigint NOT NULL,
  "legal_valid_range" tstzrange NOT NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("party_attribute_history_id")
);
-- Create index "idx_party_attributes_history_party_attribute_id" to table: "party_attributes_history"
CREATE INDEX "idx_party_attributes_history_party_attribute_id" ON "core"."party_attributes_history" ("party_attribute_id");
-- Create index "idx_party_attributes_history_party_id" to table: "party_attributes_history"
CREATE INDEX "idx_party_attributes_history_party_id" ON "core"."party_attributes_history" ("party_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."party_attributes_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create "real_property_transfer_codes" table
CREATE TABLE "core"."real_property_transfer_codes" (
  "real_property_transfer_code_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "public_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "legacy_id" text NULL,
  "real_property_transfer_code_type_id" bigint NULL,
  "name" text NOT NULL,
  "description" text NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("real_property_transfer_code_id"),
  CONSTRAINT "fk_rpt_code_type_id" FOREIGN KEY ("real_property_transfer_code_type_id") REFERENCES "core"."real_property_transfer_code_types" ("real_property_transfer_code_type_id") ON UPDATE NO ACTION ON DELETE RESTRICT
);
-- Create index "idx_rpt_codes_legacy_id" to table: "real_property_transfer_codes"
CREATE UNIQUE INDEX "idx_rpt_codes_legacy_id" ON "core"."real_property_transfer_codes" ("legacy_id");
-- Create index "idx_rpt_codes_public_id" to table: "real_property_transfer_codes"
CREATE UNIQUE INDEX "idx_rpt_codes_public_id" ON "core"."real_property_transfer_codes" ("public_id");
-- Create index "idx_rpt_codes_unique_name" to table: "real_property_transfer_codes"
CREATE UNIQUE INDEX "idx_rpt_codes_unique_name" ON "core"."real_property_transfer_codes" ("real_property_transfer_code_type_id", "name");
-- Create "real_property_transfer_code_assignments" table
CREATE TABLE "core"."real_property_transfer_code_assignments" (
  "real_property_transfer_code_assignment_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "real_property_transfer_id" bigint NOT NULL,
  "real_property_transfer_code_id" bigint NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("real_property_transfer_code_assignment_id"),
  CONSTRAINT "fk_real_property_transfer_code_id" FOREIGN KEY ("real_property_transfer_code_id") REFERENCES "core"."real_property_transfer_codes" ("real_property_transfer_code_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_real_property_transfer_id" FOREIGN KEY ("real_property_transfer_id") REFERENCES "core"."real_property_transfers" ("real_property_transfer_id") ON UPDATE NO ACTION ON DELETE RESTRICT
);
-- Create index "idx_rpt_code_assignments_id_code_id" to table: "real_property_transfer_code_assignments"
CREATE UNIQUE INDEX "idx_rpt_code_assignments_id_code_id" ON "core"."real_property_transfer_code_assignments" ("real_property_transfer_id", "real_property_transfer_code_id");
-- Create trigger "record_real_property_transfer_code_assignments_history"
CREATE TRIGGER "record_real_property_transfer_code_assignments_history" BEFORE DELETE OR UPDATE ON "core"."real_property_transfer_code_assignments" FOR EACH ROW EXECUTE FUNCTION "core"."record_real_property_transfer_code_assignments_history"();
-- Create "real_property_transfer_code_assignments_history" table
CREATE TABLE "core"."real_property_transfer_code_assignments_history" (
  "real_property_transfer_code_assignment_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "real_property_transfer_code_assignment_id" bigint NOT NULL,
  "real_property_transfer_id" bigint NOT NULL,
  "real_property_transfer_code_id" bigint NOT NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("real_property_transfer_code_assignment_history_id")
);
-- Create index "idx_rpt_code_assignments_hist_id" to table: "real_property_transfer_code_assignments_history"
CREATE INDEX "idx_rpt_code_assignments_hist_id" ON "core"."real_property_transfer_code_assignments_history" ("real_property_transfer_code_assignment_id");
-- Create index "idx_rpt_code_assignments_hist_rpt_code_id" to table: "real_property_transfer_code_assignments_history"
CREATE INDEX "idx_rpt_code_assignments_hist_rpt_code_id" ON "core"."real_property_transfer_code_assignments_history" ("real_property_transfer_code_id");
-- Create index "idx_rpt_code_assignments_hist_rpt_id" to table: "real_property_transfer_code_assignments_history"
CREATE INDEX "idx_rpt_code_assignments_hist_rpt_id" ON "core"."real_property_transfer_code_assignments_history" ("real_property_transfer_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."real_property_transfer_code_assignments_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create "real_property_transfer_party_improvements" table
CREATE TABLE "core"."real_property_transfer_party_improvements" (
  "real_property_transfer_party_improvement_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "improvement_id" bigint NOT NULL,
  "real_property_transfer_id" bigint NOT NULL,
  "party_id" bigint NOT NULL,
  "is_grantor" boolean NOT NULL,
  "transferred_share" numeric(5,4) NOT NULL,
  "system_updated_at" timestamptz NOT NULL DEFAULT now(),
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("real_property_transfer_party_improvement_id"),
  CONSTRAINT "fk_improvement_id" FOREIGN KEY ("improvement_id") REFERENCES "core"."improvements" ("improvement_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_party_id" FOREIGN KEY ("party_id") REFERENCES "core"."parties" ("party_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_real_property_transfer_id" FOREIGN KEY ("real_property_transfer_id") REFERENCES "core"."real_property_transfers" ("real_property_transfer_id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "valid_share_transfer" CHECK ((transferred_share > (0)::numeric) AND (transferred_share <= 1.0000))
);
-- Create index "idx_rpt_party_improvements_improvement_id" to table: "real_property_transfer_party_improvements"
CREATE INDEX "idx_rpt_party_improvements_improvement_id" ON "core"."real_property_transfer_party_improvements" ("improvement_id");
-- Create index "idx_rpt_party_improvements_party_id" to table: "real_property_transfer_party_improvements"
CREATE INDEX "idx_rpt_party_improvements_party_id" ON "core"."real_property_transfer_party_improvements" ("party_id");
-- Create index "idx_rpt_party_improvements_rpt_id" to table: "real_property_transfer_party_improvements"
CREATE INDEX "idx_rpt_party_improvements_rpt_id" ON "core"."real_property_transfer_party_improvements" ("real_property_transfer_id");
-- Create index "idx_rpt_party_improvements_unique_party" to table: "real_property_transfer_party_improvements"
CREATE UNIQUE INDEX "idx_rpt_party_improvements_unique_party" ON "core"."real_property_transfer_party_improvements" ("improvement_id", "real_property_transfer_id", "party_id");
-- Create trigger "record_real_property_transfer_party_improvements_history"
CREATE TRIGGER "record_real_property_transfer_party_improvements_history" BEFORE DELETE OR UPDATE ON "core"."real_property_transfer_party_improvements" FOR EACH ROW EXECUTE FUNCTION "core"."record_real_property_transfer_party_improvements_history"();
-- Create "real_property_transfer_code_types_history" table
CREATE TABLE "core"."real_property_transfer_code_types_history" (
  "real_property_transfer_code_type_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "real_property_transfer_code_type_id" bigint NOT NULL,
  "public_id" uuid NOT NULL,
  "legacy_id" text NULL,
  "name" text NOT NULL,
  "description" text NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("real_property_transfer_code_type_history_id")
);
-- Create index "idx_rpt_code_types_history_id" to table: "real_property_transfer_code_types_history"
CREATE INDEX "idx_rpt_code_types_history_id" ON "core"."real_property_transfer_code_types_history" ("real_property_transfer_code_type_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."real_property_transfer_code_types_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create "real_property_transfer_codes_history" table
CREATE TABLE "core"."real_property_transfer_codes_history" (
  "real_property_transfer_code_history_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "real_property_transfer_code_id" bigint NOT NULL,
  "public_id" uuid NOT NULL,
  "legacy_id" text NULL,
  "real_property_transfer_code_type_id" bigint NULL,
  "name" text NOT NULL,
  "description" text NULL,
  "system_valid_range" tstzrange NOT NULL,
  "trace_id" character varying(32) NOT NULL,
  PRIMARY KEY ("real_property_transfer_code_history_id")
);
-- Create index "idx_rpt_transfer_codes_history_id" to table: "real_property_transfer_codes_history"
CREATE INDEX "idx_rpt_transfer_codes_history_id" ON "core"."real_property_transfer_codes_history" ("real_property_transfer_code_id");
-- Create trigger "history_immutable"
CREATE TRIGGER "history_immutable" BEFORE DELETE OR TRUNCATE OR UPDATE ON "core"."real_property_transfer_codes_history" FOR EACH STATEMENT EXECUTE FUNCTION "core"."prevent_history_tampering"();
-- Create trigger "record_real_property_transfer_codes_history"
CREATE TRIGGER "record_real_property_transfer_codes_history" BEFORE DELETE OR UPDATE ON "core"."real_property_transfer_codes" FOR EACH ROW EXECUTE FUNCTION "core"."record_real_property_transfer_codes_history"();
