-- Modify "get_parcel_tiles" function
CREATE OR REPLACE FUNCTION "core"."get_parcel_tiles" ("z" integer, "x" integer, "y" integer) RETURNS bytea LANGUAGE plpgsql STRICT STABLE PARALLEL SAFE AS $$
DECLARE
  mvt bytea;
  bounds extensions.geometry;
BEGIN
   -- Guard against invalid zoom levels causing ST_TileEnvelope to overflow
  IF z < 0 OR z > 24 THEN
      RETURN NULL;
  END IF;
      
  bounds := extensions.ST_TileEnvelope(z, x, y);
      
  WITH tile AS (
    SELECT 
        pg.feature_id AS id,
        -- Clip the geometry to the tile boundary for performance
        extensions.ST_AsMVTGeom(extensions.ST_Transform(pg.geom_web, 3857), bounds, 4096, 256, true) AS geom
    FROM 
        core.parcels p
    JOIN
        geo.parcel_geometry pg ON p.parcel_id = pg.parcel_id
    WHERE 
        p.is_voided = false AND
        pg.legal_valid_range @> NOW() AND
        extensions.ST_Intersects(pg.geom_web, extensions.ST_Transform(bounds, 4326))
  )
  SELECT extensions.ST_AsMVT(tile, 'parcels', 4096, 'geom', 'id')
  INTO mvt
  FROM tile;
  RETURN mvt;
END;
$$;
-- Modify "get_primary_improvements" function
CREATE OR REPLACE FUNCTION "core"."get_primary_improvements" ("parcel_ids" bigint[], "target_time" timestamptz) RETURNS TABLE ("parcel_id" bigint, "improvement_id" bigint) LANGUAGE plpgsql STABLE PARALLEL SAFE AS $$
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
      core.improvement_attributes ia ON pi.improvement_id = ia.improvement_id 
      AND ia.legal_valid_range @> target_time
    WHERE 
      pi.parcel_id = ANY(parcel_ids)
      AND pi.legal_valid_range @> target_time
  ) AS sub
  WHERE 
    sub.rn = 1;
END;
$$;
-- Modify "record_address_attributes_history" function
CREATE OR REPLACE FUNCTION "core"."record_address_attributes_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.address_attributes_history (
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
-- Modify "record_addresses_history" function
CREATE OR REPLACE FUNCTION "core"."record_addresses_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.addresses_history (
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
-- Modify "record_improvement_attributes_history" function
CREATE OR REPLACE FUNCTION "core"."record_improvement_attributes_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.improvement_attributes_history (
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
-- Modify "record_improvement_condition_attributes_history" function
CREATE OR REPLACE FUNCTION "core"."record_improvement_condition_attributes_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.improvement_condition_attributes_history (
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
-- Modify "record_improvement_conditions_history" function
CREATE OR REPLACE FUNCTION "core"."record_improvement_conditions_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.improvement_conditions_history (
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
-- Modify "record_improvement_geometry_history" function
CREATE OR REPLACE FUNCTION "core"."record_improvement_geometry_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO geo.improvement_geometry_history (
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
-- Modify "record_improvement_parties_history" function
CREATE OR REPLACE FUNCTION "core"."record_improvement_parties_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.improvement_parties_history (
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
-- Modify "record_improvement_qualities_history" function
CREATE OR REPLACE FUNCTION "core"."record_improvement_qualities_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.improvement_qualities_history (
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
-- Modify "record_improvement_quality_attributes_history" function
CREATE OR REPLACE FUNCTION "core"."record_improvement_quality_attributes_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.improvement_quality_attributes_history (
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
-- Modify "record_improvement_valuations_history" function
CREATE OR REPLACE FUNCTION "core"."record_improvement_valuations_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.improvement_valuations_history (
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
-- Modify "record_improvements_history" function
CREATE OR REPLACE FUNCTION "core"."record_improvements_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.improvements_history (
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
-- Modify "record_neighborhood_definitions_history" function
CREATE OR REPLACE FUNCTION "core"."record_neighborhood_definitions_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.neighborhood_definitions_history (
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
-- Modify "record_neighborhoods_history" function
CREATE OR REPLACE FUNCTION "core"."record_neighborhoods_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.neighborhoods_history (
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
-- Modify "record_parcel_affordances_history" function
CREATE OR REPLACE FUNCTION "core"."record_parcel_affordances_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.parcel_affordances_history (
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
-- Modify "record_parcel_attributes_history" function
CREATE OR REPLACE FUNCTION "core"."record_parcel_attributes_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.parcel_attributes_history (
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
-- Modify "record_parcel_geometry_history" function
CREATE OR REPLACE FUNCTION "core"."record_parcel_geometry_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO geo.parcel_geometry_history (
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
-- Modify "record_parcel_improvements_history" function
CREATE OR REPLACE FUNCTION "core"."record_parcel_improvements_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.parcel_improvements_history (
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
-- Modify "record_parcel_neighborhood_definitions_history" function
CREATE OR REPLACE FUNCTION "core"."record_parcel_neighborhood_definitions_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.parcel_neighborhood_definitions_history (
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
-- Modify "record_parcel_parties_history" function
CREATE OR REPLACE FUNCTION "core"."record_parcel_parties_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.parcel_parties_history (
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
-- Modify "record_parcel_valuations_history" function
CREATE OR REPLACE FUNCTION "core"."record_parcel_valuations_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.parcel_valuations_history (
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
-- Modify "record_parcels_history" function
CREATE OR REPLACE FUNCTION "core"."record_parcels_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.parcels_history (
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
-- Modify "record_parties_history" function
CREATE OR REPLACE FUNCTION "core"."record_parties_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.parties_history (
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
-- Modify "record_party_attributes_history" function
CREATE OR REPLACE FUNCTION "core"."record_party_attributes_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.party_attributes_history (
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
-- Modify "record_real_property_transfer_code_assignments_history" function
CREATE OR REPLACE FUNCTION "core"."record_real_property_transfer_code_assignments_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.real_property_transfer_code_assignments_history (
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
-- Modify "record_real_property_transfer_code_types_history" function
CREATE OR REPLACE FUNCTION "core"."record_real_property_transfer_code_types_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.real_property_transfer_code_types_history (
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
-- Modify "record_real_property_transfer_codes_history" function
CREATE OR REPLACE FUNCTION "core"."record_real_property_transfer_codes_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.real_property_transfer_codes_history (
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
-- Modify "record_real_property_transfer_party_improvements_history" function
CREATE OR REPLACE FUNCTION "core"."record_real_property_transfer_party_improvements_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.real_property_transfer_party_improvements_history (
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
-- Modify "record_real_property_transfer_party_parcels_history" function
CREATE OR REPLACE FUNCTION "core"."record_real_property_transfer_party_parcels_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.real_property_transfer_party_parcels_history (
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
-- Modify "record_real_property_transfers_history" function
CREATE OR REPLACE FUNCTION "core"."record_real_property_transfers_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.real_property_transfers_history (
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
-- Modify "record_system_settings_history" function
CREATE OR REPLACE FUNCTION "core"."record_system_settings_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.system_settings_history (
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
-- Modify "record_valuations_history" function
CREATE OR REPLACE FUNCTION "core"."record_valuations_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.valuations_history (
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
-- Modify "record_zoning_attributes_history" function
CREATE OR REPLACE FUNCTION "core"."record_zoning_attributes_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.zoning_attributes_history (
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
-- Modify "record_zoning_history" function
CREATE OR REPLACE FUNCTION "core"."record_zoning_history" () RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE
  current_transaction_time timestamptz := now();
BEGIN
  IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
    INSERT INTO core.zoning_history (
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
-- Modify "generate_secure_mvt_id" function
CREATE OR REPLACE FUNCTION "geo"."generate_secure_mvt_id" () RETURNS bigint LANGUAGE plpgsql PARALLEL SAFE AS $$
DECLARE
    b bytea := extensions.gen_random_bytes(7);
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
