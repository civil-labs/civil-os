-- Rename a column from "bathrooms" to "full_bathrooms"
ALTER TABLE "core"."improvement_attributes" RENAME COLUMN "bathrooms" TO "full_bathrooms";
-- Modify "improvement_attributes" table
ALTER TABLE "core"."improvement_attributes" ADD CONSTRAINT "chk_improvement_attributes_full_bathrooms" CHECK ((full_bathrooms IS NULL) OR (full_bathrooms >= 0)), ADD CONSTRAINT "chk_improvement_attributes_half_bathrooms" CHECK ((half_bathrooms IS NULL) OR (half_bathrooms >= 0)), ADD COLUMN "half_bathrooms" integer NULL;
-- Rename a column from "bathrooms" to "full_bathrooms"
ALTER TABLE "core"."improvement_attributes_history" RENAME COLUMN "bathrooms" TO "full_bathrooms";
-- Modify "improvement_attributes_history" table
ALTER TABLE "core"."improvement_attributes_history" ADD COLUMN "half_bathrooms" integer NULL;
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
      full_bathrooms,
      half_bathrooms,
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
      OLD.full_bathrooms,
      OLD.half_bathrooms,
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
