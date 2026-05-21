##############################
### Extensions
##############################
schema "public" {
  comment = "standard public schema"
}

extension "postgis" {
  schema = schema.public

  version = "3.5.6"

  // Specifying the schema here doesn't work for some reason. Need to revisit this at some point
  // It defaults to 3.5.2
}

extension "btree_gist" {
  schema = schema.public
}

##############################
### System Settings
##############################
table "system_settings" {
  schema = schema.public

  column "system_setting_id" {
    type = int
    null = false
    default = 1
  }

  column "base_currency" {
    type = char(3)
    null = false
    default = "USD"
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }

  primary_key {
    columns = [column.system_setting_id]
  }

  check "enforce_single_row" {
    expr = "system_setting_id = 1"
  }
}

table "system_settings_history" {
  schema = schema.public

  column "system_setting_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "system_setting_id" {
    type = int
    null = false
  }

  column "base_currency" {
    type = char(3)
    null = false
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [column.system_setting_history_id]
  }
}

##############################
### Parcels
##############################

// Domain Anchor
table "parcels" {
  schema = schema.public

  column "parcel_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "is_voided" {
    type = boolean
    null = false
    default = false
  }

  column "voided_at" {
    type = timestamptz
    null = true
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.parcel_id ]
  }

  index "idx_parcels_public_id" {
    unique  = true
    columns = [column.public_id]
  }

  index "idx_parcels_legacy_id" {
    unique  = true
    columns = [column.legacy_id]
  }

  check "chk_voided_logic" {
    expr = "(is_voided = false AND voided_at IS NULL) OR (is_voided = true AND voided_at IS NOT NULL)"
  }
}

table "parcels_history" {
  schema = schema.public

  column "parcel_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "parcel_id" {
    type = bigint
    null = false
  }

  column "public_id" {
    type = uuid
    null = false
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "is_voided" {
    type = boolean
    null = false
  }

  column "voided_at" {
    type = timestamptz
    null = true
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.parcel_history_id ]
  }

  index "idx_parcels_history_parcel_id" {
    columns = [ column.parcel_id ]
  }
}

// Geometry
table "parcel_geometry" {
  schema = schema.public

  column "parcel_geometry_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "parcel_id" {
    type = bigint
    null = false
  }

  column "geom_web" {
    type = sql("geometry(MultiPolygon, 4326)")
    null = false
  }

  column "geom_legal" {
    type = sql("geometry(MultiPolygon)")
    null = true
  }

  column "local_srid" {
    type = int
    null = true
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.parcel_geometry_id ]
  }

  foreign_key "fk_parcel_id" {
    columns = [column.parcel_id]
    ref_columns = [table.parcels.column.parcel_id]
    on_delete = RESTRICT
  }

  index "idx_geom_web_parcel_geometry" {
    type = GIST

    on {
      column = column.geom_web
    }
  }

  exclude "no_overlapping_parcel_geometry" {
    type = GIST
    on {
      column = column.parcel_id
      op = "="
    }
    on {
      column = column.legal_valid_range
      op = "&&"
    }
  }
}

table "parcel_geometry_history" {
  schema = schema.public

  column "parcel_geometry_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "parcel_geometry_id" {
    type = bigint
    null = false
  }

  column "parcel_id" {
    type = bigint
    null = false
  }

  column "geom_web" {
    type = sql("geometry(MultiPolygon, 4326)")
    null = false
  }

  column "geom_legal" {
    type = sql("geometry(MultiPolygon)")
    null = true
  }

  column "local_srid" {
    type = int
    null = true
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.parcel_geometry_history_id ]
  }

  index "idx_parcel_geometry_history_parcel_geometry_id" {
    columns = [column.parcel_geometry_id]
  }

  index "idx_parcel_geometry_history_parcel_id" {
    columns = [column.parcel_id]
  }
}

// Attributes
table "parcel_attributes" {
  schema = schema.public

  column "parcel_attribute_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "parcel_id" {
    type = bigint
    null = false
  }

  column "address_id" {
    type = bigint
    null = false
  }

  column "land_use_id" {
    type = bigint
    null = false
  }

  column "land_area_sq_m" {
    type = double_precision
    null = true
  }

  column "frontage_m" {
    type = double_precision
    null = true
  }

  column "depth_m" {
    type = double_precision
    null = true
  }  

  column "properties" {
    type = jsonb
    null = true
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.parcel_attribute_id ]
  }

  foreign_key "fk_parcel_id" {
    columns = [column.parcel_id]
    ref_columns = [table.parcels.column.parcel_id]
    on_delete = RESTRICT
  }

  foreign_key "fk_address_id" {
    columns = [ column.address_id ]
    ref_columns = [ table.addresses.column.address_id ]
    on_delete = RESTRICT
  }

  foreign_key "fk_land_use_id" {
    columns = [ column.land_use_id ]
    ref_columns = [ table.land_uses.column.land_use_id ]
    on_delete = RESTRICT
  }

  exclude "no_overlapping_parcel_attributes" {
    type = GIST
    on {
      column = column.parcel_id
      op = "="
    }
    on {
      column = column.legal_valid_range
      op = "&&"
    }
  }

  check "properties_size_limit" {
    expr = "pg_column_size(properties) <= 5000"
  }
}

table "parcel_attributes_history" {
  schema = schema.public

  column "parcel_attribute_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "parcel_attribute_id" {
    type = bigint
    null = false
  }

  column "parcel_id" {
    type = bigint
    null = false
  }

  column "address_id" {
    type = bigint
    null = false
  }

  column "land_area_sq_m" {
    type = double_precision
    null = true
  }

  column "frontage_m" {
    type = double_precision
    null = true
  }

  column "depth_m" {
    type = double_precision
    null = true
  }  

  column "land_use_id" {
    type = bigint
    null = false
  }

  column "properties" {
    type = jsonb
    null = true
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.parcel_attribute_history_id ]
  }

  index "idx_parcel_attributes_history_parcel_attribute_id" {
    columns = [column.parcel_attribute_id]
  }

  index "idx_parcel_attributes_history_parcel_id" {
    columns = [column.parcel_id]
  }
}

// Parties
table "parcel_parties" {
  schema = schema.public

  column "parcel_party_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "parcel_id" {
    type = bigint
    null = false
  }

  column "party_id" {
    type = bigint
    null = false
  }

  column "ownership_share" {
    type = numeric(5,4)
    null = false
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.parcel_party_id ]
  }

  foreign_key "fk_parcel_id" {
    columns = [ column.parcel_id ]
    ref_columns = [ table.parcels.column.parcel_id ]
    on_delete = RESTRICT
  }

  foreign_key "fk_party_id" {
    columns = [ column.party_id ]
    ref_columns = [ table.parties.column.party_id ]
    on_delete = RESTRICT
  }

  exclude "no_overlapping_parcel_parties" {
    type = GIST
    on {
      column = column.parcel_id
      op = "="
    }
    on {
      column = column.party_id
      op = "="
    }
    on {
      column = column.legal_valid_range
      op = "&&"
    }
  }

  check "valid_parcel_ownership" {
    expr = "ownership_share > 0 AND ownership_share <= 1.0000"
  }
}

table "parcel_parties_history" {
  schema = schema.public

  column "parcel_party_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "parcel_party_id" {
    type = bigint
    null = false
  }

  column "parcel_id" {
    type = bigint
    null = false
  }

  column "party_id" {
    type = bigint
    null = false
  }

  column "ownership_share" {
    type = numeric(5,4)
    null = false
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.parcel_party_history_id ]
  }

  index "idx_parcel_parties_history_parcel_party_id" {
    columns = [column.parcel_party_id]
  }

  index "idx_parcel_parties_history_parcel_id" {
    columns = [column.parcel_id]
  }

  index "idx_parcel_parties_history_party_id" {
    columns = [column.party_id]
  }
}

// Affordances
table "parcel_affordances" {
  schema = schema.public

  column "parcel_affordance_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }

  column "parcel_id" {
    type = bigint
    null = false
  }

  column "zoning_id" {
    type = bigint
    null = true
  }

  column "affordance_type_id" {
    type = bigint
    null = false
  }

  column "source" {
    type = text
    null = true
  }

  column "precedence_rank" {
    type = int
    null = false
    default = 100
  }

  column "max_far" {
    type = numeric(6,2)
    null = true
  }

  column "min_lot_size_sq_m" {
    type = numeric(6,2)
    null = true
  }

  column "max_height_m" {
    type = numeric(6,2)
    null = true
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.parcel_affordance_id ]
  }

  foreign_key "fk_parcel_id" {
    columns = [column.parcel_id]
    ref_columns = [table.parcels.column.parcel_id]
    on_delete = RESTRICT
  }

  foreign_key "fk_zoning_id" {
    columns = [column.zoning_id]
    ref_columns = [table.zoning.column.zoning_id]
    on_delete = RESTRICT
  }

  foreign_key "fk_affordance_type_id" {
    columns = [ column.affordance_type_id ]
    ref_columns = [ table.affordance_types.column.affordance_type_id ]
    on_delete = RESTRICT
  }

  index "idx_parcel_affordances_public_id" {
    unique  = true
    columns = [column.public_id]
  }

  check "chk_require_origin" {
    expr = "zoning_id IS NOT NULL OR affordance_type_id != 1"
  }

  exclude "no_overlapping_zoning_affordance" {
    type = GIST
    on {
      column = column.parcel_id
      op = "="
    }
    on {
      column = column.zoning_id
      op = "="
    }
    on {
      column = column.legal_valid_range
      op = "&&"
    }

    # Ensure this check only applies to affordances based
    # on zoning, as otherwise there will be unpredictable
    # NULL = NULL behavior on the zoning ID check
    where = "(zoning_id IS NOT NULL)"
  }
}

table "parcel_affordances_history" {
  schema = schema.public

  column "parcel_affordance_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "parcel_affordance_id" {
    type = bigint
    null = false
  }

  column "public_id" {
    type = uuid
    null = false
  }

  column "parcel_id" {
    type = bigint
    null = false
  }

  column "zoning_id" {
    type = bigint
    null = true
  }

  column "affordance_type_id" {
    type = bigint
    null = false
  }

  column "source" {
    type = text
    null = true
  }

  column "precedence_rank" {
    type = int
    null = false
  }

  column "max_far" {
    type = numeric(6,2)
    null = true
  }

  column "min_lot_size_sq_m" {
    type = numeric(6,2)
    null = true
  }

  column "max_height_m" {
    type = numeric(6,2)
    null = true
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.parcel_affordance_history_id ]
  }

  index "idx_parcel_affordances_history_parcel_affordance_id" {
    columns = [column.parcel_affordance_id]
  }

  index "idx_parcel_affordances_history_parcel_id" {
    columns = [column.parcel_id]
  }
}

table "parcel_neighborhood_definitions" {
  schema = schema.public

  column "parcel_neighborhood_definition_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "parcel_id" {
    type = bigint
    null = false
  }

  column "neighborhood_id" {
    type = bigint
    null = false
  }

  column "neighborhood_definition_id" {
    type = bigint
    null = false
  }

  column "is_legal" {
    type = boolean
    null = false
  }

  column "legal_valid_range" {
    type = tstzrange
    null = true
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.parcel_neighborhood_definition_id ]
  }

  foreign_key "fk_parcel_id" {
    columns = [ column.parcel_id ]
    ref_columns = [ table.parcels.column.parcel_id ]
    on_delete = RESTRICT
  }

  foreign_key "fk_neighborhood_id" {
    columns = [ column.neighborhood_id ]
    ref_columns = [ table.neighborhoods.column.neighborhood_id ]
    on_delete = RESTRICT
  }

  foreign_key "fk_neighborhood_definition_id" {
    columns = [ column.neighborhood_definition_id, column.is_legal ]
    ref_columns = [ table.neighborhood_definitions.column.neighborhood_definition_id, table.neighborhood_definitions.column.is_legal ]
    on_delete = RESTRICT
  }

  check "chk_legal_must_have_time" {
    expr = "(is_legal = false) OR (is_legal = true AND legal_valid_range IS NOT NULL)"
  }

  // This check only applies to parcel groupings with a legal meaning
  exclude "no_overlapping_legal_parcel_neighborhood_definitions" {
    type = GIST
    on {
      column = column.parcel_id
      op = "="
    }
    on {
      column = column.neighborhood_definition_id
      op = "="
    }
    on {
      column = column.legal_valid_range
      op = "&&"
    }

    where = "(is_legal = true)"
  }

  // This check only applies to parcel groupings without a legal meaning
  index "idx_unique_non_legal_parcel_neighborhood" {
    unique = true
    columns = [ column.parcel_id, column.neighborhood_definition_id ]
    where   = "(is_legal = false)"
  }
}

table "parcel_neighborhood_definitions_history" {
  schema = schema.public

  column "parcel_neighborhood_definition_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "parcel_neighborhood_definition_id" {
    type = bigint
    null = false
  }

  column "parcel_id" {
    type = bigint
    null = false
  }

  column "neighborhood_id" {
    type = bigint
    null = false
  }

  column "neighborhood_definition_id" {
    type = bigint
    null = false
  }

  column "is_legal" {
    type = boolean
    null = false
  }

  column "legal_valid_range" {
    type = tstzrange
    null = true
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.parcel_neighborhood_definition_history_id ]
  }

  index "idx_parcel_neighborhood_definitions_history_parcel_neighborhood_definition_id" {
    columns = [column.parcel_neighborhood_definition_id]
  }

  index "idx_parcel_neighborhood_definitions_history_parcel_id" {
    columns = [column.parcel_id]
  }

  index "idx_parcel_neighborhood_definitions_history_neighborhood_id" {
    columns = [column.neighborhood_id]
  }

  index "idx_parcel_neighborhood_definitions_history_neighborhood_definition_id" {
    columns = [column.neighborhood_definition_id]
  }
}

##############################
### Improvements
##############################

// Domain Anchor
table "improvements" {
  schema = schema.public

  column "improvement_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "is_voided" {
    type = boolean
    null = false
    default = false
  }

  column "voided_at" {
    type = timestamptz
    null = true
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  primary_key {
    columns = [ column.improvement_id ]
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  index "idx_improvements_public_id" {
    unique  = true
    columns = [column.public_id]
  }

  index "idx_improvements_legacy_id" {
    unique  = true
    columns = [column.legacy_id]
  }

  check "chk_voided_logic" {
    expr = "(is_voided = false AND voided_at IS NULL) OR (is_voided = true AND voided_at IS NOT NULL)"
  }
}

table "improvements_history" {
  schema = schema.public

  column "improvement_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "improvement_id" {
    type = bigint
    null = false
  }

  column "public_id" {
    type = uuid
    null = false
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "is_voided" {
    type = boolean
    null = false
  }

  column "voided_at" {
    type = timestamptz
    null = true
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.improvement_history_id ]
  }

  index "idx_improvements_history_improvement_id" {
    columns = [column.improvement_id]
  }
}

// Geometry
table "improvement_geometry" {
  schema = schema.public

  column "improvement_geometry_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "improvement_id" {
    type = bigint
    null = false
  }

  column "geom_web" {
    type = sql("geometry(MultiPolygonZ, 4326)")
    null = false
  }

  column "geom_legal" {
    type = sql("geometry(MultiPolygonZ)")
    null = true
  }

  column "local_horizontal_srid" {
    type = int
    null = true
  }

  column "local_vertical_datum" {
    type = text
    null = true
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.improvement_geometry_id ]
  }

  foreign_key "fk_improvement_id" {
    columns = [ column.improvement_id ]
    ref_columns = [ table.improvements.column.improvement_id ]
    on_delete = RESTRICT
  }

  index "idx_geom_web_improvement_geometry" {
    type = GIST

    on {
      column = column.geom_web
    }
  }

  exclude "no_overlapping_improvement_geometry" {
    type = GIST
    on {
      column = column.improvement_id
      op = "="
    }
    on {
      column = column.legal_valid_range
      op = "&&"
    }
  }
}

table "improvement_geometry_history" {
  schema = schema.public

  column "improvement_geometry_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "improvement_geometry_id" {
    type = bigint
    null = false
  }

  column "improvement_id" {
    type = bigint
    null = false
  }

  column "geom_web" {
    type = sql("geometry(MultiPolygonZ, 4326)")
    null = false
  }

  column "geom_legal" {
    type = sql("geometry(MultiPolygonZ)")
    null = true
  }

  column "local_horizontal_srid" {
    type = int
    null = true
  }

  column "local_vertical_datum" {
    type = text
    null = true
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.improvement_geometry_history_id ]
  }

  index "idx_improvement_geometry_history_improvement_geometry_id" {
    columns = [ column.improvement_geometry_id ]
  }

  index "idx_improvement_geometry_history_improvement_id" {
    columns = [ column.improvement_id ]
  }
}

// Attributes
table "improvement_attributes" {
  schema = schema.public

  column "improvement_attribute_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "improvement_id" {
    type = bigint
    null = false
  }

  column "address_id" {
    type = bigint
    null = false
  }

  column "improvement_type_id" {
    type = bigint
    null = false
  }

  column "improvement_condition_id" {
    type = bigint
    null = true
  }

  column "area_sq_m" {
    type = double_precision
    null = true
  }

  column "bathrooms" {
    type = int
    null = true
  }

  column "bedrooms" {
    type = int
    null = true
  }

  column "year_built" {
    type = int
    null = true
  }

  column "units" {
    type = int
    null = true
  }

  column "properties" {
    type = jsonb
    null = true
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }

  primary_key {
    columns = [ column.improvement_attribute_id ]
  }

  foreign_key "fk_improvement_id" {
    columns = [ column.improvement_id ]
    ref_columns = [ table.improvements.column.improvement_id ]
    on_delete = RESTRICT
  }

  foreign_key "fk_address_id" {
    columns = [ column.address_id ]
    ref_columns = [ table.addresses.column.address_id ]
    on_delete = RESTRICT
  }

  foreign_key "fk_improvement_type_id" {
    columns = [ column.improvement_type_id ]
    ref_columns = [ table.improvement_types.column.improvement_type_id ]
    on_delete = RESTRICT
  }

  foreign_key "fk_improvement_condition_id" {
    columns = [ column.improvement_condition_id ]
    ref_columns = [ table.improvement_conditions.column.improvement_condition_id ]
    on_delete = RESTRICT
  }

  exclude "no_overlapping_improvement_attributes" {
    type = GIST
    on {
      column = column.improvement_id
      op = "="
    }
    on {
      column = column.legal_valid_range
      op = "&&"
    }
  }

  check "properties_size_limit" {
    expr = "pg_column_size(properties) <= 5000"
  }
}

table "improvement_attributes_history" {
  schema = schema.public

  column "improvement_attribute_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "improvement_attribute_id" {
    type = bigint
    null = false
  }

  column "improvement_id" {
    type = bigint
    null = false
  }

  column "address_id" {
    type = bigint
    null = false
  }

  column "area_sq_m" {
    type = double_precision
    null = true
  }

  column "bathrooms" {
    type = int
    null = true
  }

  column "bedrooms" {
    type = int
    null = true
  }

  column "year_built" {
    type = int
    null = true
  }

  column "condition_num" {
    type = int
    null = true
  }
  
  column "units" {
    type = int
    null = true
  }

  column "properties" {
    type = jsonb
    null = true
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.improvement_attribute_history_id ]
  }

  index "idx_improvement_attributes_history_improvement_attribute_id" {
    columns = [ column.improvement_attribute_id ]
  }

  index "idx_improvement_attributes_history_improvement_id" {
    columns = [ column.improvement_id ]
  }
}

// Parties
table "improvement_parties" {
  schema = schema.public

  column "improvement_party_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "improvement_id" {
    type = bigint
    null = false
  }

  column "party_id" {
    type = bigint
    null = false
  }

  column "ownership_share" {
    type = numeric(5,4)
    null = false
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.improvement_party_id ]
  }

  foreign_key "fk_improvement_id" {
    columns = [ column.improvement_id ]
    ref_columns = [ table.improvements.column.improvement_id ]
    on_delete = RESTRICT
  }

  foreign_key "fk_party_id" {
    columns = [ column.party_id ]
    ref_columns = [ table.parties.column.party_id ]
    on_delete = RESTRICT
  }

  exclude "no_overlapping_improvement_parties" {
    type = GIST
    on {
      column = column.improvement_id
      op = "="
    }
    on {
      column = column.party_id
      op = "="
    }
    on {
      column = column.legal_valid_range
      op = "&&"
    }
  }

  check "valid_improvement_ownership" {
    expr = "ownership_share > 0 AND ownership_share <= 1.0000"
  }
}

table "improvement_parties_history" {
  schema = schema.public

  column "improvement_party_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "improvement_party_id" {
    type = bigint
    null = false
  }

  column "improvement_id" {
    type = bigint
    null = false
  }

  column "party_id" {
    type = bigint
    null = false
  }

  column "ownership_share" {
    type = numeric(5,4)
    null = false
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.improvement_party_history_id ]
  }

  index "idx_improvement_parties_history_improvement_party_id" {
    columns = [column.improvement_party_id]
  }

  index "idx_improvement_parties_history_improvement_id" {
    columns = [column.improvement_id]
  }

  index "idx_improvement_parties_history_party_id" {
    columns = [column.party_id]
  }
}

##############################
### Improvement Conditions
##############################

// Domain Anchor
table "improvement_conditions" {
  schema = schema.public

  column "improvement_condition_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  } 

  column "public_id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "is_voided" {
    type = boolean
    null = false
    default = false
  }

  column "voided_at" {
    type = timestamptz
    null = true
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.improvement_condition_id ]
  }

  index "idx_improvement_conditions_public_id" {
    unique  = true
    columns = [column.public_id]
  }

  index "idx_improvement_conditions_legacy_id" {
    unique  = true
    columns = [column.legacy_id]
  }

  check "chk_voided_logic" {
    expr = "(is_voided = false AND voided_at IS NULL) OR (is_voided = true AND voided_at IS NOT NULL)"
  }  
}

table "improvement_conditions_history" {
  schema = schema.public

  column "improvement_condition_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  } 

  column "improvement_condition_id" {
    type = bigint
    null = false
  } 

  column "public_id" {
    type = uuid
    null = false
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "is_voided" {
    type = boolean
    null = false
  }

  column "voided_at" {
    type = timestamptz
    null = true
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.improvement_condition_history_id ]
  }

  index "idx_improvement_condition_history_improvement_condition_id" {
    columns = [column.improvement_condition_id]
  }
}

// Attributes
table "improvement_condition_attributes" {
  schema = schema.public

  column "improvement_condition_attribute_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  } 

  column "improvement_condition_id" {
    type = bigint
    null = false
  } 

  column "name" {
    type = text
    null = false
  }

  column "depreciation_modifier" {
    type = numeric(5,4)
    null = false
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.improvement_condition_attribute_id ]
  }

  foreign_key "fk_improvement_condition_id" {
    columns = [ column.improvement_condition_id ]
    ref_columns = [ table.improvement_conditions.column.improvement_condition_id ]
    on_delete = RESTRICT
  }

  exclude "no_overlapping_improvement_conditions" {
    type = GIST
    on {
      column = column.improvement_condition_id
      op = "="
    }
    on {
      column = column.legal_valid_range
      op = "&&"
    }
  }
}

table "improvement_condition_attributes_history" {
  schema = schema.public

  column "improvement_condition_attribute_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  } 

  column "improvement_condition_attribute_id" {
    type = bigint
    null = false
  } 

  column "improvement_condition_id" {
    type = bigint
    null = false
  } 

  column "name" {
    type = text
    null = false
  }

  column "depreciation_modifier" {
    type = numeric(5,4)
    null = false
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.improvement_condition_attribute_history_id ]
  }

  index "idx_impr_cond_attr_history_impr_cond_attr_id" {
    columns = [ column.improvement_condition_attribute_id ]
  }

  index "idx_impr_cond_attr_history_impr_cond_id" {
    columns = [ column.improvement_condition_id ]
  }

}

##############################
### Zoning
##############################

// Domain Anchor
table "zoning" {
  schema = schema.public

  column "zoning_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "is_voided" {
    type = boolean
    null = false
    default = false
  }

  column "voided_at" {
    type = timestamptz
    null = true
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.zoning_id ]
  }

  index "idx_zoning_public_id" {
    unique  = true
    columns = [ column.public_id ]
  }

  index "idx_zoning_legacy_id" {
    unique  = true
    columns = [column.legacy_id]
  }

  check "chk_voided_logic" {
    expr = "(is_voided = false AND voided_at IS NULL) OR (is_voided = true AND voided_at IS NOT NULL)"
  }
}

table "zoning_history" {
  schema = schema.public

  column "zoning_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "zoning_id" {
    type = bigint
    null = false
  }

  column "public_id" {
    type = uuid
    null = false
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "is_voided" {
    type = boolean
    null = false
  }

  column "voided_at" {
    type = timestamptz
    null = true
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.zoning_history_id ]
  }

  index "idx_zoning_history_zoning_id" {
    columns = [column.zoning_id]
  }
}

// Attributes
table "zoning_attributes" {
  schema = schema.public

  column "zoning_attribute_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "zoning_id" {
    type = bigint
    null = false
  } 

  column "name" {
    type = text
    null = false
  }

  column "code" {
    type = text
    null = false
  }

  column "max_far" {
    type = numeric(6,2)
    null = true
  }

  column "min_lot_size_sq_m" {
    type = numeric(6,2)
    null = true
  }

  column "max_height_m" {
    type = numeric(6,2)
    null = true
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.zoning_attribute_id ]
  }

  foreign_key "fk_zoning_id" {
    columns = [ column.zoning_id ]
    ref_columns = [ table.zoning.column.zoning_id ]
    on_delete = RESTRICT
  }

  exclude "no_overlapping_zoning_attributes" {
    type = GIST
    on {
      column = column.zoning_id
      op = "="
    }
    on {
      column = column.legal_valid_range
      op = "&&"
    }
  }
}

table "zoning_attributes_history" {
  schema = schema.public

  column "zoning_attribute_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "zoning_attribute_id" {
    type = bigint
    null = false
  }

  column "zoning_id" {
    type = bigint
    null = false
  } 

  column "name" {
    type = text
    null = false
  }

  column "code" {
    type = text
    null = false
  }

  column "max_far" {
    type = numeric(6,2)
    null = true
  }

  column "min_lot_size_sq_m" {
    type = numeric(6,2)
    null = true
  }

  column "max_height_m" {
    type = numeric(6,2)
    null = true
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.zoning_attribute_history_id ]
  }

  index "idx_zoning_attributes_history_zoning_attribute_id" {
    columns = [ column.zoning_attribute_id ]
  }

  index "idx_zoning_attributes_history_zoning_id" {
    columns = [ column.zoning_id ]
  }  
}

##############################
### Parties
##############################

// Domain Anchor
table "parties" {
  schema = schema.public

  column "party_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "is_voided" {
    type = boolean
    null = false
    default = false
  }

  column "voided_at" {
    type = timestamptz
    null = true
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.party_id ]
  }  

  index "idx_parties_public_id" {
    unique  = true
    columns = [ column.public_id ]
  }

  index "idx_parties_legacy_id" {
    unique  = true
    columns = [column.legacy_id]
  }

  check "chk_voided_logic" {
    expr = "(is_voided = false AND voided_at IS NULL) OR (is_voided = true AND voided_at IS NOT NULL)"
  }
}

table "parties_history" {
  schema = schema.public

  column "party_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "party_id" {
    type = bigint
    null = false
  }

  column "public_id" {
    type = uuid
    null = false
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "is_voided" {
    type = boolean
    null = false
  }

  column "voided_at" {
    type = timestamptz
    null = true
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.party_history_id ]
  }

  index "idx_parties_history_party_id" {
    columns = [column.party_id]
  }
}

// Attributes
table "party_attributes" {
  schema = schema.public

  column "party_attribute_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "party_id" {
    type = bigint
    null = false
  }

  column "name" {
    type = text
    null = false
  }

  column "address_id" {
    type = bigint
    null = false
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.party_attribute_id ]
  }

  foreign_key "fk_party_id" {
    columns = [ column.party_id ]
    ref_columns = [ table.parties.column.party_id ]
    on_delete = RESTRICT
  }

  foreign_key "fk_address_id" {
    columns = [ column.address_id ]
    ref_columns = [ table.addresses.column.address_id ]
    on_delete = RESTRICT
  }

  exclude "no_overlapping_party_attributes" {
    type = GIST
    on {
      column = column.party_id
      op = "="
    }
    on {
      column = column.legal_valid_range
      op = "&&"
    }
  }
}

table "party_attributes_history" {
  schema = schema.public

  column "party_attribute_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "party_attribute_id" {
    type = bigint
    null = false
  }

  column "party_id" {
    type = bigint
    null = false
  }

  column "name" {
    type = text
    null = false
  }

  column "address_id" {
    type = bigint
    null = false
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.party_attribute_history_id ]
  }

  index "idx_party_attributes_history_party_attribute_id" {
    columns = [ column.party_attribute_id ]
  }

  index "idx_party_attributes_history_party_id" {
    columns = [ column.party_id ]
  }  
}

##############################
### Addresses
##############################

// Domain Anchor
table "addresses" {
  schema = schema.public

  column "address_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "is_voided" {
    type = boolean
    null = false
    default = false
  }

  column "voided_at" {
    type = timestamptz
    null = true
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.address_id ]
  }  

  index "idx_addresses_public_id" {
    unique  = true
    columns = [ column.public_id ]
  }

  index "idx_addresses_legacy_id" {
    unique  = true
    columns = [column.legacy_id]
  }

  check "chk_voided_logic" {
    expr = "(is_voided = false AND voided_at IS NULL) OR (is_voided = true AND voided_at IS NOT NULL)"
  }
}

table "addresses_history" {
  schema = schema.public

  column "address_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "address_id" {
    type = bigint
    null = false
  }

  column "public_id" {
    type = uuid
    null = false
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "is_voided" {
    type = boolean
    null = false
  }

  column "voided_at" {
    type = timestamptz
    null = true
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.address_history_id ]
  }

  index "idx_addresses_history_address_id" {
    columns = [column.address_id]
  }
}

// Attributes
table "address_attributes" {
  schema = schema.public

  column "address_attribute_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "address_id" {
    type = bigint
    null = false
  }

  column "country_id" {
    type = bigint
    null = true
  }

  column "administrative_area_id" {
    type = bigint
    null = true
  }

  column "locality" {
    type = text
    null = true
  }

  column "sublocality" {
    type = text
    null = true
  }

  column "postal_code" {
    type = text
    null = true
  }

  column "address_line_1" {
    type = text
    null = true
  }

  column "address_line_2" {
    type = text
    null = true
  }

  column "address_line_3" {
    type = text
    null = true
  }

  column "formatted_address" {
    type = text
    null = false
  }

  column "coordinates" {
    type = sql("geometry(Point, 4326)")
    null = true
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.address_attribute_id ]
  }

  foreign_key "fk_address_id" {
    columns = [ column.address_id ]
    ref_columns = [ table.addresses.column.address_id ]
    on_delete = RESTRICT
  }

  foreign_key "fk_country_id" {
    columns = [ column.country_id ]
    ref_columns = [ table.countries.column.country_id ]
    on_delete = RESTRICT
  }

  foreign_key "fk_administrative_area_id" {
    columns = [ column.administrative_area_id ]
    ref_columns = [ table.administrative_areas.column.administrative_area_id ]
    on_delete = RESTRICT
  }

  exclude "no_overlapping_address_attributes" {
    type = GIST
    on {
      column = column.address_id
      op = "="
    }
    on {
      column = column.legal_valid_range
      op = "&&"
    }
  }
}

table "address_attributes_history" {
  schema = schema.public

  column "address_attribute_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "address_attribute_id" {
    type = bigint
    null = false
  }

  column "address_id" {
    type = bigint
    null = false
  }

  column "country_id" {
    type = bigint
    null = false
  }

  column "administrative_area_id" {
    type = bigint
    null = true
  }

  column "locality" {
    type = text
    null = true
  }

  column "sublocality" {
    type = text
    null = true
  }

  column "address_line_1" {
    type = text
    null = true
  }

  column "address_line_2" {
    type = text
    null = true
  }

  column "address_line_3" {
    type = text
    null = true
  }

  column "formatted_address" {
    type = text
    null = false
  }

  column "coordinates" {
    type = sql("geometry(Point, 4326)")
    null = true
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.address_attribute_history_id ]
  }

  index "idx_address_attributes_history_address_attribute_id" {
    columns = [ column.address_attribute_id ]
  }

  index "idx_address_attributes_history_address_id" {
    columns = [ column.address_id ]
  }  
}

##############################
### Real Property Transfers
##############################

# Real Property Transfers
table "real_property_transfers" {
  schema = schema.public

  column "real_property_transfer_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "transfer_timestamp" {
    type = timestamptz
    null = false
  }

  column "transfer_amount" {
    type = numeric(19, 4)
    null = true
  }

  column "deed_book" {
    type = text
    null = true
  }

  column "deed_page" {
    type = text
    null = true
  }

  column "deed_uri" {
    type = text
    null = true
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.real_property_transfer_id ]
  }

  index "idx_real_property_transfers_public_id" {
    unique  = true
    columns = [column.public_id]
  }

  index "idx_real_property_transfers_legacy_id" {
    unique  = true
    columns = [column.legacy_id]
  } 
}

table "real_property_transfers_history" {
  schema = schema.public

  column "real_property_transfer_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "real_property_transfer_id" {
    type = bigint
    null = false
  }

  column "public_id" {
    type = uuid
    null = false
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "transfer_timestamp" {
    type = timestamptz
    null = false
  }

  column "transfer_amount" {
    type = numeric(19, 4)
    null = true
  }

  column "deed_book" {
    type = text
    null = true
  }

  column "deed_page" {
    type = text
    null = true
  }

  column "deed_uri" {
    type = text
    null = true
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.real_property_transfer_history_id ]
  }

  index "idx_real_property_transfers_history_real_property_transfer_id" {
    columns = [ column.real_property_transfer_id ]
  }
}

# Linking Tables
table "real_property_transfer_party_parcels" {
  schema = schema.public

  column "real_property_transfer_party_parcel_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "parcel_id" {
    type = bigint
    null = false
  }

  column "real_property_transfer_id" {
    type = bigint
    null = false
  }

  column "party_id" {
    type = bigint
    null = false
  }

  # If false, implies they are the grantee
  column "is_grantor" {
    type = boolean
    null = false
  }

  column "transferred_share" {
    type = numeric(5,4)
    null = false    
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.real_property_transfer_party_parcel_id ]
  }

  foreign_key "fk_parcel_id" {
    columns = [ column.parcel_id ]
    ref_columns = [ table.parcels.column.parcel_id ]
    on_delete = RESTRICT
  }

  foreign_key "fk_real_property_transfer_id" {
    columns = [ column.real_property_transfer_id ]
    ref_columns = [ table.real_property_transfers.column.real_property_transfer_id ]
    on_delete = RESTRICT
  }

  foreign_key "fk_party_id" {
    columns = [ column.party_id ]
    ref_columns = [ table.parties.column.party_id ]
    on_delete = RESTRICT
  }

  check "valid_share_transfer" {
    expr = "transferred_share > 0 AND transferred_share <= 1.0000"
  }

  # Without is_grantor being included in the column, a party can only be a grantor or grantee
  # in a transfer, not both. That should cover almost every case, and if cases pop up where
  # that must be broken its time to start looking at a more robut solution for municipal clerks' offices
  index "idx_rpt_party_parcels_unique_party" {
    unique = true
    columns = [ column.parcel_id, column.real_property_transfer_id, column.party_id ]
  }
}

table "real_property_transfer_party_parcels_history" {
  schema = schema.public

  column "real_property_transfer_party_parcel_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "real_property_transfer_party_parcel_id" {
    type = bigint
    null = false
  }

  column "parcel_id" {
    type = bigint
    null = false
  }

  column "real_property_transfer_id" {
    type = bigint
    null = false
  }  

  column "party_id" {
    type = bigint
    null = false
  }

  column "is_grantor" {
    type = boolean
    null = false
  }

  column "transferred_share" {
    type = numeric(5,4)
    null = false    
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.real_property_transfer_party_parcel_history_id ]
  }

  index "idx_rpt_party_parcels_hist_id" {
    columns = [ column.real_property_transfer_party_parcel_id ]
  }  

  index "idx_rpt_party_parcels_hist_parcel_id" {
    columns = [ column.parcel_id ]
  }  

  index "idx_rpt_party_parcels_hist_transfer_id" {
    columns = [ column.real_property_transfer_id ]
  }  

  index "idx_rpt_party_parcels_hist_party_id" {
    columns = [ column.party_id ]
  }   
}

table "real_property_transfer_party_improvements" {
  schema = schema.public

  column "real_property_transfer_party_improvement_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "improvement_id" {
    type = bigint
    null = false
  }

  column "real_property_transfer_id" {
    type = bigint
    null = false
  }

  column "party_id" {
    type = bigint
    null = false
  }

  column "is_grantor" {
    type = boolean
    null = false
  }

  column "transferred_share" {
    type = numeric(5,4)
    null = false
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.real_property_transfer_party_improvement_id ]
  }

  foreign_key "fk_improvement_id" {
    columns = [ column.improvement_id ]
    ref_columns = [ table.improvements.column.improvement_id ]
    on_delete = RESTRICT
  }

  foreign_key "fk_real_property_transfer_id" {
    columns = [ column.real_property_transfer_id ]
    ref_columns = [ table.real_property_transfers.column.real_property_transfer_id ]
    on_delete = RESTRICT
  }

  foreign_key "fk_party_id" {
    columns = [ column.party_id ]
    ref_columns = [ table.parties.column.party_id ]
    on_delete = RESTRICT
  }

  check "valid_share_transfer" {
    expr = "transferred_share > 0 AND transferred_share <= 1.0000"
  }

  # Without is_grantor being included in the column, a party can only be a grantor or grantee
  # in a transfer, not both. That should cover almost every case, and if cases pop up where
  # that must be broken its time to start looking at a more robut solution for municipal clerks' offices
  index "idx_rpt_party_improvements_unique_party" {
    unique = true
    columns = [ column.improvement_id, column.real_property_transfer_id, column.party_id ]
  }
}

table "real_property_transfer_party_improvements_history" {
  schema = schema.public

  column "real_property_transfer_party_improvement_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "real_property_transfer_party_improvement_id" {
    type = bigint
    null = false
  }

  column "improvement_id" {
    type = bigint
    null = false
  }

  column "real_property_transfer_id" {
    type = bigint
    null = false
  }  

  column "party_id" {
    type = bigint
    null = false
  }

  column "is_grantor" {
    type = boolean
    null = false
  }

  column "transferred_share" {
    type = numeric(5,4)
    null = false    
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.real_property_transfer_party_improvement_history_id ]
  }

  index "idx_rpt_party_improvements_hist_id" {
    columns = [ column.real_property_transfer_party_improvement_id ]
  }  

  index "idx_rpt_party_improvements_hist_improvement_id" {
    columns = [ column.improvement_id ]
  }  

  index "idx_rpt_party_improvements_hist_transfer_id" {
    columns = [ column.real_property_transfer_id ]
  }  

  index "idx_rpt_party_improvements_hist_party_id" {
    columns = [ column.party_id ]
  } 
}

table "real_property_transfer_code_assignments" {
  schema = schema.public

  column "real_property_transfer_code_assignment_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "real_property_transfer_id" {
    type = bigint
    null = false
  }

  column "real_property_transfer_code_id" {
    type = bigint
    null = false
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.real_property_transfer_code_assignment_id ]
  }

  foreign_key "fk_real_property_transfer_id" {
    columns = [ column.real_property_transfer_id ]
    ref_columns = [ table.real_property_transfers.column.real_property_transfer_id ]
    on_delete = RESTRICT
  }

  foreign_key "fk_real_property_transfer_code_id" {
    columns = [ column.real_property_transfer_code_id ]
    ref_columns = [ table.real_property_transfer_codes.column.real_property_transfer_code_id ]
    on_delete = RESTRICT
  }

  index "idx_rpt_code_assignments_id_code_id" {
    unique = true
    columns = [ column.real_property_transfer_id, column.real_property_transfer_code_id ]
  }  
}

table "real_property_transfer_code_assignments_history" {
  schema = schema.public

  column "real_property_transfer_code_assignment_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "real_property_transfer_code_assignment_id" {
    type = bigint
    null = false
  }

  column "real_property_transfer_id" {
    type = bigint
    null = false
  }

  column "real_property_transfer_code_id" {
    type = bigint
    null = false
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.real_property_transfer_code_assignment_history_id ]
  }

  index "idx_rpt_code_assignments_hist_id" {
    columns = [ column.real_property_transfer_code_assignment_id ]
  }  

  index "idx_rpt_code_assignments_hist_rpt_id" {
    columns = [ column.real_property_transfer_id ]
  }  

  index "idx_rpt_code_assignments_hist_rpt_code_id" {
    columns = [ column.real_property_transfer_code_id ]
  }  
}

# Codes/Code Types
table "real_property_transfer_codes" {
  schema = schema.public

  column "real_property_transfer_code_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "real_property_transfer_code_type_id" {
    type = bigint
    null = true
  }

  column "name" {
    type = text
    null = false
  }

  column "description" {
    type = text
    null = true
  } 

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  } 

  primary_key {
    columns = [ column.real_property_transfer_code_id ]
  }

  foreign_key "fk_rpt_code_type_id" {
    columns = [ column.real_property_transfer_code_type_id ]
    ref_columns = [ table.real_property_transfer_code_types.column.real_property_transfer_code_type_id ]
    on_delete = RESTRICT
  }

  index "idx_rpt_codes_public_id" {
    unique  = true
    columns = [column.public_id]
  }

  index "idx_rpt_codes_legacy_id" {
    unique  = true
    columns = [column.legacy_id]
  }

  index "idx_rpt_codes_unique_name" {
    unique  = true
    columns = [column.real_property_transfer_code_type_id, column.name]
  }
}

table "real_property_transfer_codes_history" {
  schema = schema.public

  column "real_property_transfer_code_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "real_property_transfer_code_id" {
    type = bigint
    null = false
  }

  column "public_id" {
    type = uuid
    null = false
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "real_property_transfer_code_type_id" {
    type = bigint
    null = true
  }

  column "name" {
    type = text
    null = false
  }

  column "description" {
    type = text
    null = true
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.real_property_transfer_code_history_id ]
  }

  index "idx_rpt_transfer_codes_history_id" {
    columns = [ column.real_property_transfer_code_id ]
  } 
}

table "real_property_transfer_code_types" {
  schema = schema.public

  column "real_property_transfer_code_type_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "name" {
    type = text
    null = false
  }

  column "description" {
    type = text
    null = true
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.real_property_transfer_code_type_id ]
  }

  index "idx_rpt_code_types_public_id" {
    unique  = true
    columns = [column.public_id]
  }

  index "idx_rpt_code_types_legacy_id" {
    unique  = true
    columns = [column.legacy_id]
  }

  index "idx_rpt_code_types_name" {
    unique  = true
    columns = [column.name]
  }
}

table "real_property_transfer_code_types_history" {
  schema = schema.public

  column "real_property_transfer_code_type_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "real_property_transfer_code_type_id" {
    type = bigint
    null = false
  }

  column "public_id" {
    type = uuid
    null = false
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "name" {
    type = text
    null = false
  }

  column "description" {
    type = text
    null = true
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.real_property_transfer_code_type_history_id ]
  }

  index "idx_rpt_code_types_history_id" {
    columns = [ column.real_property_transfer_code_type_id ]
  } 
}

##############################
### Valuations
##############################

table "valuations" {
  schema = schema.public

  column "valuation_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "valuation_date" {
    type = timestamptz
    null = false
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.valuation_id ]
  }

  index "idx_valuations_public_id" {
    unique  = true
    columns = [column.public_id]
  }

  index "idx_valuations_legacy_id" {
    unique  = true
    columns = [column.legacy_id]
  }

}

table "valuations_history" {
  schema = schema.public

  column "valuation_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "valuation_id" {
    type = bigint
    null = false
  }

  column "public_id" {
    type = uuid
    null = false
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "valuation_date" {
    type = timestamptz
    null = false
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.valuation_history_id ]
  }

  index "idx_valuations_history_valuation_id" {
    columns = [column.valuation_id]
  }
}

table "parcel_valuations" {
  schema = schema.public

  column "parcel_valuation_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "parcel_id" {
    type = bigint
    null = false
  }

  column "valuation_id" {
    type = bigint
    null = false
  }

  column "market_value" {
    type = numeric(19, 4)
    null = false
  }

  column "assessed_value" {
    type = numeric(19, 4)
    null = false
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.parcel_valuation_id ]
  }

  foreign_key "fk_parcel_id" {
    columns = [ column.parcel_id ]
    ref_columns = [ table.parcels.column.parcel_id ]
    on_delete = RESTRICT
  }

  foreign_key "fk_valuation_id" {
    columns = [ column.valuation_id ]
    ref_columns = [ table.valuations.column.valuation_id ]
    on_delete = RESTRICT
  }

  index "idx_parcel_valuations_parcel_id_valuation_id" {
    unique = true
    columns = [ column.parcel_id, column.valuation_id ]
  }

  exclude "no_overlapping_parcel_valuations" {
    type = GIST
    on {
      column = column.parcel_id
      op = "="
    }
    on {
      column = column.legal_valid_range
      op = "&&"
    }
  }
}

table "parcel_valuations_history" {
  schema = schema.public

  column "parcel_valuation_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "parcel_valuation_id" {
    type = bigint
    null = false
  }

  column "parcel_id" {
    type = bigint
    null = false
  }

  column "valuation_id" {
    type = bigint
    null = false
  }

  column "market_value" {
    type = numeric(19, 4)
    null = false
  }

  column "assessed_value" {
    type = numeric(19, 4)
    null = false
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.parcel_valuation_history_id ]
  }

  index "idx_parcel_valuations_history_parcel_valuation_id" {
    columns = [ column.parcel_valuation_id ]
  }

  index "idx_parcel_valuations_history_parcel_id" {
    columns = [column.parcel_id]
  }

  index "idx_parcel_valuations_history_valuation_id" {
    columns = [column.valuation_id]
  }  
}

table "improvement_valuations" {
  schema = schema.public

  column "improvement_valuation_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "improvement_id" {
    type = bigint
    null = false
  }

  column "valuation_id" {
    type = bigint
    null = false
  }

  column "market_value" {
    type = numeric(19, 4)
    null = false
  }

  column "assessed_value" {
    type = numeric(19, 4)
    null = false
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.improvement_valuation_id ]
  }

  foreign_key "fk_improvement_id" {
    columns = [ column.improvement_id ]
    ref_columns = [ table.improvements.column.improvement_id ]
    on_delete = RESTRICT
  }

  foreign_key "fk_valuation_id" {
    columns = [ column.valuation_id ]
    ref_columns = [ table.valuations.column.valuation_id ]
    on_delete = RESTRICT
  }

  index "idx_improvement_valuations_improvement_id_valuation_id" {
    unique = true
    columns = [ column.improvement_id, column.valuation_id ]
  }

  exclude "no_overlapping_improvement_valuations" {
    type = GIST
    on {
      column = column.improvement_id
      op = "="
    }
    on {
      column = column.legal_valid_range
      op = "&&"
    }
  }
}

table "improvement_valuations_history" {
  schema = schema.public

  column "improvement_valuation_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "improvement_valuation_id" {
    type = bigint
    null = false
  }

  column "improvement_id" {
    type = bigint
    null = false
  }

  column "valuation_id" {
    type = bigint
    null = false
  }

  column "market_value" {
    type = numeric(19, 4)
    null = false
  }

  column "assessed_value" {
    type = numeric(19, 4)
    null = false
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.improvement_valuation_history_id ]
  }

  index "idx_improvement_valuations_history_improvement_valuation_id" {
    columns = [ column.improvement_valuation_id ]
  }

  index "idx_improvement_valuations_history_improvement_id" {
    columns = [column.improvement_id]
  }

  index "idx_improvement_valuations_history_valuation_id" {
    columns = [column.valuation_id]
  }  
}

##############################
### Neighborhoods
##############################

table "neighborhood_definitions" {
  schema = schema.public

  column "neighborhood_definition_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }

  column "name" {
    type = text
    null = false
  }

  column "is_legal" {
    type = boolean
    null = false
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.neighborhood_definition_id ]
  }

  # Required to allow the parcel_neighborhood_definition table's composite foreign key 
  # to safely reference the is_legal flag.
  index "idx_neighborhood_definitions_id_is_legal" {
    unique  = true
    columns = [ column.neighborhood_definition_id, column.is_legal ]
  }

  index "idx_neighborhood_definitions_public_id" {
    unique  = true
    columns = [ column.public_id ]
  }

  index "idx_neighborhood_definitions_name" {
    unique  = true
    columns = [ column.name ]
  }
}

table "neighborhood_definitions_history" {
  schema = schema.public

  column "neighborhood_definition_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "neighborhood_definition_id" {
    type = bigint
    null = false
  }

  column "public_id" {
    type = uuid
    null = false
  }

  column "name" {
    type = text
    null = false
  }

  column "is_legal" {
    type = boolean
    null = false
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.neighborhood_definition_history_id ]
  }

  index "idx_neighborhood_definitions_history_neighborhood_definition_id" {
    columns = [column.neighborhood_definition_id]
  }
}

table "neighborhoods" {
  schema = schema.public

  column "neighborhood_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "name" {
    type = text
    null = false
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.neighborhood_id ]
  }

  index "idx_neighborhoods_public_id" {
    unique  = true
    columns = [ column.public_id ]
  }

  index "idx_neighborhoods_legacy_id" {
    unique  = true
    columns = [column.legacy_id] 
  }  

  index "idx_neighborhoods_name" {
    unique  = true
    columns = [ column.name ]
  }
}

table "neighborhoods_history" {
  schema = schema.public

  column "neighborhood_history_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "neighborhood_id" {
    type = bigint
    null = false
  }

  column "public_id" {
    type = uuid
    null = false
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "name" {
    type = text
    null = false
  }

  column "system_valid_range" {
    type = tstzrange
    null = false
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.neighborhood_history_id ]
  }

  index "idx_neighborhoods_history_neighborhood_id" {
    columns = [ column.neighborhood_id ]
  }
}

##############################
### Lookup Tables
##############################

table "affordance_types" {
  schema = schema.public

  column "affordance_type_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }

  column "name" {
    type = text
    null = false
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.affordance_type_id ]
  }

  index "idx_affordance_types_public_id" {
    unique  = true
    columns = [ column.public_id ]
  }
}

table "countries" {
  schema = schema.public

  column "country_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }

  column "code" {
    type = varchar(2)
    null = false
  }

  column "name" {
    type = varchar(100)
    null = false
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.country_id ]
  }

  index "idx_countries_public_id" {
    unique  = true
    columns = [ column.public_id ]
  }

}

table "administrative_areas" {
  schema = schema.public

  column "administrative_area_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }

  column "code" {
    type = varchar(10)
    null = false
  }

  column "name" {
    type = varchar(100)
    null = false
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.administrative_area_id ]
  }

  index "idx_administrative_areas_public_id" {
    unique  = true
    columns = [ column.public_id ]
  }

}

table "land_uses" {
  schema = schema.public

  column "land_use_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "land_use_type_id" {
    type = bigint
    null = true
  }

  column "code" {
    type = text
    null = true
  }

  column "name" {
    type = text
    null = false
  }

  column "description" {
    type = text
    null = true
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.land_use_id ]
  }

  foreign_key "fk_land_use_type_id" {
    columns = [ column.land_use_type_id ]
    ref_columns = [ table.land_use_types.column.land_use_type_id ]
    on_delete = RESTRICT
  }

  index "idx_land_uses_public_id" {
    unique  = true
    columns = [ column.public_id ]
  }  

  index "idx_land_uses_legacy_id" {
    unique  = true
    columns = [column.legacy_id]
  }

  index "idx_land_uses_name" {
    unique  = true
    columns = [ column.name ]
  }
}

table "land_use_types" {
  schema = schema.public

  column "land_use_type_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "code" {
    type = text
    null = true
  }

  column "name" {
    type = text
    null = false
  }

  column "description" {
    type = text
    null = true
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.land_use_type_id ]
  }

  index "idx_land_use_types_public_id" {
    unique  = true
    columns = [ column.public_id ]
  }  

  index "idx_land_use_types_legacy_id" {
    unique  = true
    columns = [column.legacy_id]
  }

  index "idx_land_use_types_name" {
    unique  = true
    columns = [ column.name ]
  }
}

table "improvement_types" {
  schema = schema.public

  column "improvement_type_id" {
    type = bigint
    null = false

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }

  column "legacy_id" {
    type = text
    null = true
  }

  column "code" {
    type = text
    null = true
  }

  column "name" {
    type = text
    null = false
  }

  column "description" {
    type = text
    null = true
  }

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  column "trace_id" {
    type = varchar(32)
    null = false
  }  

  primary_key {
    columns = [ column.improvement_type_id ]
  }

  index "idx_improvement_types_public_id" {
    unique  = true
    columns = [ column.public_id ]
  }

  index "idx_improvement_types_legacy_id" {
    unique  = true
    columns = [column.legacy_id]
  }

  index "idx_improvement_types_name" {
    unique  = true
    columns = [ column.name ]
  }
}

##############################
### Geometry Functions
##############################

function "get_parcel_tiles" {
  schema = schema.public
  lang   = "plpgsql"

  // Function Arguments
  arg "z" {
    type = integer
  }
  
  arg "x" {
    type = integer
  }
  
  arg "y" {
    type = integer
  }

  // Return Type
  return = bytea

  // Modifiers (Performance & Security)
  volatility = STABLE
  strict     = true
  parallel   = SAFE

  // The Execution Body
  as = <<-SQL
    DECLARE
      mvt bytea;
      bounds geometry := ST_TileEnvelope(z, x, y);

    BEGIN
      SELECT ST_AsMVT(tile, 'parcels', 4096, 'geom')
      INTO mvt
      FROM (
        SELECT 
            p.parcel_id,
            pa.land_use_id,
            -- Clip the geometry to the tile boundary for performance
            ST_AsMVTGeom(ST_Transform(pg.geometry, 3857), bounds, 4096, 256, true) AS geom
        FROM 
            public.parcels p
        JOIN 
            public.parcel_attributes pa ON p.parcel_id = pa.parcel_id
        JOIN
            public.parcel_geometry pg ON p.parcel_id = pg.parcel_id
        WHERE 
            ST_Intersects(ST_Transform(pg.geometry, 3857), bounds)
    ) AS tile;
      RETURN mvt;
    END;
  SQL
}

##############################
### History Triggers
##############################
trigger "record_system_settings_history" {
  # Attach it to the base table
  on = table.system_settings
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_system_settings_history
  } 
} 

trigger "record_parcels_history" {
  # Attach it to the current-state table
  on = table.parcels
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_parcels_history
  } 
} 

trigger "record_parcel_geometry_history" {
  # Attach it to the current-state table
  on = table.parcel_geometry
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_parcel_geometry_history
  }   
}

trigger "record_parcel_attributes_history" {
  # Attach it to the current-state table
  on = table.parcel_attributes
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_parcel_attributes_history
  }
}

trigger "record_parcel_parties_history" {
  # Attach it to the current-state table
  on = table.parcel_parties
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_parcel_parties_history
  }
}

trigger "record_parcel_affordances_history" {
  # Attach it to the current-state table
  on = table.parcel_affordances
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_parcel_affordances_history
  }  
}

trigger "record_parcel_neighborhood_definitions_history" {
  # Attach it to the current-state table
  on = table.parcel_neighborhood_definitions
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_parcel_neighborhood_definitions_history
  }  
}

trigger "record_improvements_history" {
  # Attach it to the current-state table
  on = table.improvements
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_improvements_history
  }  
}

trigger "record_improvement_geometry_history" {
  # Attach it to the current-state table
  on = table.improvement_geometry
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_improvement_geometry_history
  }  
}

trigger "record_improvement_attributes_history" {
  # Attach it to the current-state table
  on = table.improvement_attributes
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_improvement_attributes_history
  }  
}

trigger "record_improvement_parties_history" {
  # Attach it to the current-state table
  on = table.improvement_parties
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_improvement_parties_history
  }
}

trigger "record_improvement_conditions_history" {
  # Attach it to the current-state table
  on = table.improvement_conditions
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_improvement_conditions_history
  }  
}

trigger "record_improvement_condition_attributes_history" {
  # Attach it to the current-state table
  on = table.improvement_condition_attributes
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_improvement_condition_attributes_history
  }  
}

trigger "record_zoning_history" {
  # Attach it to the current-state table
  on = table.zoning
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_zoning_history
  }
}

trigger "record_zoning_attributes_history" {
  # Attach it to the current-state table
  on = table.zoning_attributes
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_zoning_attributes_history
  }
}

trigger "record_parties_history" {
  # Attach it to the current-state table
  on = table.parties
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_parties_history
  }
}

trigger "record_party_attributes_history" {
  # Attach it to the current-state table
  on = table.party_attributes
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_party_attributes_history
  }
}

trigger "record_addresses_history" {
  # Attach it to the current-state table
  on = table.addresses
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_addresses_history
  }
}

trigger "record_address_attributes_history" {
  # Attach it to the current-state table
  on = table.address_attributes
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_address_attributes_history
  }
}

trigger "record_real_property_transfers_history" {
  # Attach it to the current-state table
  on = table.real_property_transfers
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_real_property_transfers_history
  }
}

trigger "record_real_property_transfer_party_parcels_history" {
  # Attach it to the current-state table
  on = table.real_property_transfer_party_parcels
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_real_property_transfer_party_parcels_history
  }
}

trigger "record_real_property_transfer_party_improvements_history" {
  # Attach it to the current-state table
  on = table.real_property_transfer_party_improvements
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_real_property_transfer_party_improvements_history
  }
}

trigger "record_real_property_transfer_code_assignments_history" {
  # Attach it to the current-state table
  on = table.real_property_transfer_code_assignments
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_real_property_transfer_code_assignments_history
  }
}

trigger "record_real_property_transfer_codes_history" {
  # Attach it to the current-state table
  on = table.real_property_transfer_codes
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_real_property_transfer_codes_history
  }
}

trigger "record_real_property_transfer_code_types_history" {
  # Attach it to the current-state table
  on = table.real_property_transfer_code_types
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_real_property_transfer_code_types_history
  }
}

trigger "record_valuations_history" {
  # Attach it to the current-state table
  on = table.valuations
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_valuations_history
  }
}

trigger "record_parcel_valuations_history" {
  # Attach it to the current-state table
  on = table.parcel_valuations
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_parcel_valuations_history
  }
}

trigger "record_improvement_valuations_history" {
  # Attach it to the current-state table
  on = table.improvement_valuations
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_improvement_valuations_history
  }
}

trigger "record_neighborhood_definitions_history" {
  # Attach it to the current-state table
  on = table.neighborhood_definitions
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_neighborhood_definitions_history
  }
}

trigger "record_neighborhoods_history" {
  # Attach it to the current-state table
  on = table.neighborhoods
  
  # Fire before the transaction is validated, as only that
  # allows commiting the new system_updated_at value
  # to the new base table record. If the base table update
  # then fails because of data type issues, it's fine because
  # the whole transaction will be rolled back
  before {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function that has the archive logic
  execute {
    function = function.record_neighborhoods_history
  }
}

trigger "history_immutable" {
  for_each = [
    table.parcels_history, table.parcel_geometry_history, table.parcel_attributes_history,
    table.parcel_affordances_history, table.parcel_neighborhood_definitions_history, table.improvements_history, 
    table.improvement_geometry_history, table.improvement_attributes_history, table.improvement_conditions_history,
    table.improvement_condition_attributes_history, table.zoning_history, table.zoning_attributes_history,
    table.parties_history, table.party_attributes_history, table.addresses_history,
    table.address_attributes_history, table.real_property_transfers_history, table.real_property_transfer_party_parcels_history,
    table.real_property_transfer_party_improvements_history, table.real_property_transfer_code_assignments_history, table.real_property_transfer_codes_history,
    table.real_property_transfer_code_types_history, table.valuations_history, table.parcel_valuations_history,
    table.improvement_valuations_history, table.neighborhood_definitions_history, table.neighborhoods_history
  ]
  on = each.value

  before {
    insert = false
    update = true
    delete = true
    truncate = true
  }

  for = STATEMENT

  execute {
    function = function.prevent_history_tampering
  } 
}

##############################
### History Functions
##############################
function "record_system_settings_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  security = DEFINER
  
  as = <<-SQL
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
    SQL
}

function "record_parcels_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  security = DEFINER
  
  as = <<-SQL
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
    SQL
}

function "record_parcel_geometry_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  security = DEFINER
  
  as = <<-SQL
      DECLARE
        current_transaction_time timestamptz := now();
      BEGIN
        IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
          INSERT INTO parcel_geometry_history (
            parcel_geometry_id,
            parcel_id,
            geom_web,
            geom_legal,
            local_srid,
            legal_valid_range,
            system_valid_range,
            trace_id
          ) VALUES (
            OLD.parcel_geometry_id,
            OLD.parcel_id,
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
    SQL  
}

function "record_parcel_attributes_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
      DECLARE
        current_transaction_time timestamptz := now();
      BEGIN
        IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
          INSERT INTO parcel_attributes_history (
            parcel_attribute_id,
            parcel_id,
            address_id,
            land_area_sq_m,
            land_use_id,
            neighborhood_id,
            market_area_id,
            properties,
            legal_valid_range,
            system_valid_range,
            trace_id
          ) VALUES (
            OLD.parcel_attribute_id,
            OLD.parcel_id,
            OLD.address_id,
            OLD.land_area_sq_m,
            OLD.land_use_id,
            OLD.neighborhood_id,
            OLD.market_area_id,
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
    SQL  
}

function "record_parcel_parties_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_parcel_affordances_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_parcel_neighborhood_definitions_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_improvements_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_improvement_geometry_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_improvement_attributes_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
      DECLARE
        current_transaction_time timestamptz := now();
      BEGIN
        IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
          INSERT INTO improvement_attributes_history (
            improvement_attribute_id,
            improvement_id,
            address_id,
            area_sq_m,
            bathrooms,
            bedrooms,
            year_built,
            condition_num,
            units,
            properties,
            legal_valid_range,
            system_valid_range,
            trace_id
          ) VALUES (
            OLD.improvement_attribute_id,
            OLD.improvement_id,
            OLD.address_id,
            OLD.area_sq_m,
            OLD.bathrooms,
            OLD.bedrooms,
            OLD.year_built,
            OLD.condition_num,
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
    SQL  
}

function "record_improvement_parties_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_improvement_conditions_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_improvement_condition_attributes_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_zoning_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_zoning_attributes_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_parties_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_party_attributes_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_addresses_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_address_attributes_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_real_property_transfers_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_real_property_transfer_party_parcels_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_real_property_transfer_party_improvements_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_real_property_transfer_code_assignments_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_real_property_transfer_codes_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_real_property_transfer_code_types_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_valuations_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_parcel_valuations_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_improvement_valuations_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "record_neighborhood_definitions_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
      DECLARE
        current_transaction_time timestamptz := now();
      BEGIN
        IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
          INSERT INTO neighborhood_definitions_history (
            neighborhood_definition_id,
            public_id,
            name,
            is_legal,
            system_valid_range,
            trace_id
          ) VALUES (
            OLD.neighborhood_definition_id,
            OLD.public_id,
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
    SQL  
}

function "record_neighborhoods_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  # Use the creator's role, as the caller shouldn't have insert privileges
  security = DEFINER 
  
  as = <<-SQL
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
    SQL  
}

function "prevent_history_tampering" {
  schema = schema.public
  lang   = "plpgsql"
  return = trigger
  
  as = <<-SQL
    BEGIN
      RAISE EXCEPTION 'TAMPER ALERT: History tables are immutable, append-only ledgers. UPDATE, DELETE, and TRUNCATE operations are strictly forbidden.';
    END;
  SQL
}