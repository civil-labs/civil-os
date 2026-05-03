##############################
### Extensions
##############################
schema "public" {
  comment = "standard public schema"
}

extension "postgis" {
  schema = schema.public
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

  primary_key {
    columns = [ column.parcel_id ]
  }

  index "idx_parcels_public_id" {
    unique  = true
    columns = [column.public_id]
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

  primary_key {
    columns = [ column.parcel_history_id ]
  }

  index "idx_parcels_history_parcel_id" {
    columns = [column.parcel_id]
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

  column "owner_id" {
    type = bigint
    null = false
  }

  column "address_id" {
    type = bigint
    null = false
  }

  column "land_area_sq_m" {
    type = double_precision
    null = false
  }

  column "land_use_id" {
    type = bigint
    null = false
  }

  column "neighborhood_id" {
    type = bigint
    null = false
  }

  column "market_area_id" {
    type = bigint
    null = false
  }

  column "properties" {
    type = jsonb
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

  primary_key {
    columns = [ column.parcel_attribute_id ]
  }

  foreign_key "fk_parcel_id" {
    columns = [column.parcel_id]
    ref_columns = [table.parcels.column.parcel_id]
    on_delete = RESTRICT
  }

  foreign_key "fk_owner_id" {
    columns = [ column.owner_id ]
    ref_columns = [ table.owners.column.owner_id ]
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

  foreign_key "fk_neighborhood_id" {
    columns = [ column.neighborhood_id ]
    ref_columns = [ table.neighborhoods.column.neighborhood_id ]
    on_delete = RESTRICT
  }

  foreign_key "fk_market_area_id" {
    columns = [ column.market_area_id ]
    ref_columns = [ table.market_areas.column.market_area_id ]
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

  column "owner_id" {
    type = bigint
    null = false
  }

  column "address_id" {
    type = bigint
    null = false
  }

  column "land_area_sq_m" {
    type = double_precision
    null = false
  }

  column "land_use_id" {
    type = bigint
    null = false
  }

  column "neighborhood_id" {
    type = bigint
    null = false
  }

  column "market_area_id" {
    type = bigint
    null = false
  }

  column "properties" {
    type = jsonb
  }

  column "legal_valid_range" {
    type = tstzrange
    null = false
  }

  column "system_valid_range" {
    type = tstzrange
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
    type = smallint
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
    where = "zoning_id IS NOT NULL"
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
    type = smallint
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

  index "idx_improvements_public_id" {
    unique  = true
    columns = [column.public_id]
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

  column "owner_id" {
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

  column "system_updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }

  primary_key {
    columns = [ column.improvement_attribute_id ]
  }

  foreign_key "fk_improvement_id" {
    columns = [ column.improvement_id ]
    ref_columns = [ table.improvements.column.improvement_id ]
    on_delete = RESTRICT
  }

  foreign_key "fk_owner_id" {
    columns = [ column.owner_id ]
    ref_columns = [ table.owners.column.owner_id ]
    on_delete = RESTRICT
  }

  foreign_key "fk_address_id" {
    columns = [ column.address_id ]
    ref_columns = [ table.addresses.column.address_id ]
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

  column "owner_id" {
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

  primary_key {
    columns = [ column.zoning_id ]
  }

  index "idx_zoning_public_id" {
    unique  = true
    columns = [ column.public_id ]
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
### Owners
##############################

// Domain Anchor

// Attributes

##############################
### Addresses
##############################

// Domain Anchor

// Attributes

##############################
### Sales
##############################

##############################
### Valuations
##############################

##############################
### Lookup Tables
##############################

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

trigger "history_immutable" {
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
  return = "trigger"
  security = DEFINER
  
  as = <<-SQL
      DECLARE
        current_transaction_time timestamptz := now();
      BEGIN
        IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
          INSERT INTO system_settings_history (
            system_setting_id,
            base_currency,
            system_valid_range
          ) VALUES (
            OLD.system_setting_id,
            OLD.base_currency,
            tstzrange(OLD.system_updated_at, current_transaction_time, '[)')
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
  return = "trigger"
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
            system_valid_range
          ) VALUES (
            OLD.parcel_id,
            OLD.public_id,
            OLD.legacy_id,
            OLD.is_voided,
            OLD.voided_at,
            tstzrange(OLD.system_updated_at, current_transaction_time, '[)')
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
  return = "trigger"
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
            system_valid_range
          ) VALUES (
            OLD.parcel_geometry_id,
            OLD.parcel_id,
            OLD.geom_web,
            OLD.geom_legal,
            OLD.local_srid,
            OLD.legal_valid_range,
            tstzrange(OLD.system_updated_at, current_transaction_time, '[)')
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
  return = "trigger"
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
            owner_id,
            address_id,
            land_area_sq_m,
            land_use_id,
            neighborhood_id,
            market_area_id,
            properties,
            legal_valid_range,
            system_valid_range
          ) VALUES (
            OLD.parcel_attribute_id,
            OLD.parcel_id,
            OLD.owner_id,
            OLD.address_id,
            OLD.land_area_sq_m,
            OLD.land_use_id,
            OLD.neighborhood_id,
            OLD.market_area_id,
            OLD.properties,
            OLD.legal_valid_range,
            tstzrange(OLD.system_updated_at, current_transaction_time, '[)')
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
  return = "trigger"
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
            system_valid_range
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
            tstzrange(OLD.system_updated_at, current_transaction_time, '[)')
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
  return = "trigger"
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
            system_valid_range
          ) VALUES (
            OLD.improvement_id,
            OLD.public_id,
            OLD.legacy_id,
            OLD.is_voided,
            OLD.voided_at,
            tstzrange(OLD.system_updated_at, current_transaction_time, '[)')
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
  return = "trigger"
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
            system_valid_range
          ) VALUES (
            OLD.improvement_geometry_id,
            OLD.improvement_id,
            OLD.geom_web,
            OLD.geom_legal,
            OLD.local_horizontal_srid,
            OLD.local_vertical_datum,
            OLD.legal_valid_range,
            tstzrange(OLD.system_updated_at, current_transaction_time, '[)')
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
  return = "trigger"
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
            owner_id,
            address_id,
            area_sq_m,
            bathrooms,
            bedrooms,
            year_built,
            condition_num,
            units,
            properties,
            legal_valid_range,
            system_valid_range
          ) VALUES (
            OLD.improvement_attribute_id,
            OLD.improvement_id,
            OLD.owner_id,
            OLD.address_id,
            OLD.area_sq_m,
            OLD.bathrooms,
            OLD.bedrooms,
            OLD.year_built,
            OLD.condition_num,
            OLD.units,
            OLD.properties,
            OLD.legal_valid_range,
            tstzrange(OLD.system_updated_at, current_transaction_time, '[)')
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
  return = "trigger"
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
            system_valid_range
          ) VALUES (
            OLD.zoning_id,
            OLD.public_id,
            OLD.legacy_id,
            OLD.is_voided,
            OLD.voided_at,
            tstzrange(OLD.system_updated_at, current_transaction_time, '[)')
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
  return = "trigger"
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
            system_valid_range
          ) VALUES (
            OLD.zoning_attribute_id,
            OLD.zoning_id,
            OLD.name,
            OLD.code,
            OLD.max_far,
            OLD.min_lot_size_sq_m,
            OLD.max_height_m,
            OLD.legal_valid_range,
            tstzrange(OLD.system_updated_at, current_transaction_time, '[)')
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
  return = "trigger"
  
  as = <<-SQL
    BEGIN
      RAISE EXCEPTION 'TAMPER ALERT: History tables are immutable, append-only ledgers. UPDATE and DELETE operations are strictly forbidden.';
    END;
  SQL
}

##############################
### Lookup Table Data Prepopulation
##############################

## Populate countries lookup table
data {
  table = table.countries
  rows = [
    { country_id = 1, code = "AF", name = "Afghanistan" },
    { country_id = 2, code = "AL", name = "Albania" },
    { country_id = 3, code = "DZ", name = "Algeria" },
    { country_id = 4, code = "AD", name = "Andorra" },
    { country_id = 5, code = "AO", name = "Angola" },
    { country_id = 6, code = "AG", name = "Antigua and Barbuda" },
    { country_id = 7, code = "AR", name = "Argentina" },
    { country_id = 8, code = "AM", name = "Armenia" },
    { country_id = 9, code = "AU", name = "Australia" },
    { country_id = 10, code = "AT", name = "Austria" },
    { country_id = 11, code = "AZ", name = "Azerbaijan" },
    { country_id = 12, code = "BS", name = "Bahamas" },
    { country_id = 13, code = "BH", name = "Bahrain" },
    { country_id = 14, code = "BD", name = "Bangladesh" },
    { country_id = 15, code = "BB", name = "Barbados" },
    { country_id = 16, code = "BY", name = "Belarus" },
    { country_id = 17, code = "BE", name = "Belgium" },
    { country_id = 18, code = "BZ", name = "Belize" },
    { country_id = 19, code = "BJ", name = "Benin" },
    { country_id = 20, code = "BT", name = "Bhutan" },
    { country_id = 21, code = "BO", name = "Bolivia" },
    { country_id = 22, code = "BA", name = "Bosnia and Herzegovina" },
    { country_id = 23, code = "BW", name = "Botswana" },
    { country_id = 24, code = "BR", name = "Brazil" },
    { country_id = 25, code = "BN", name = "Brunei" },
    { country_id = 26, code = "BG", name = "Bulgaria" },
    { country_id = 27, code = "BF", name = "Burkina Faso" },
    { country_id = 28, code = "BI", name = "Burundi" },
    { country_id = 29, code = "CV", name = "Cabo Verde" },
    { country_id = 30, code = "KH", name = "Cambodia" },
    { country_id = 31, code = "CM", name = "Cameroon" },
    { country_id = 32, code = "CA", name = "Canada" },
    { country_id = 33, code = "CF", name = "Central African Republic" },
    { country_id = 34, code = "TD", name = "Chad" },
    { country_id = 35, code = "CL", name = "Chile" },
    { country_id = 36, code = "CN", name = "China" },
    { country_id = 37, code = "CO", name = "Colombia" },
    { country_id = 38, code = "KM", name = "Comoros" },
    { country_id = 39, code = "CG", name = "Congo" },
    { country_id = 40, code = "CR", name = "Costa Rica" },
    { country_id = 41, code = "HR", name = "Croatia" },
    { country_id = 42, code = "CU", name = "Cuba" },
    { country_id = 43, code = "CY", name = "Cyprus" },
    { country_id = 44, code = "CZ", name = "Czechia" },
    { country_id = 45, code = "CD", name = "Democratic Republic of the Congo" },
    { country_id = 46, code = "DK", name = "Denmark" },
    { country_id = 47, code = "DJ", name = "Djibouti" },
    { country_id = 48, code = "DM", name = "Dominica" },
    { country_id = 49, code = "DO", name = "Dominican Republic" },
    { country_id = 50, code = "EC", name = "Ecuador" },
    { country_id = 51, code = "EG", name = "Egypt" },
    { country_id = 52, code = "SV", name = "El Salvador" },
    { country_id = 53, code = "GQ", name = "Equatorial Guinea" },
    { country_id = 54, code = "ER", name = "Eritrea" },
    { country_id = 55, code = "EE", name = "Estonia" },
    { country_id = 56, code = "SZ", name = "Eswatini" },
    { country_id = 57, code = "ET", name = "Ethiopia" },
    { country_id = 58, code = "FJ", name = "Fiji" },
    { country_id = 59, code = "FI", name = "Finland" },
    { country_id = 60, code = "FR", name = "France" },
    { country_id = 61, code = "GA", name = "Gabon" },
    { country_id = 62, code = "GM", name = "Gambia" },
    { country_id = 63, code = "GE", name = "Georgia" },
    { country_id = 64, code = "DE", name = "Germany" },
    { country_id = 65, code = "GH", name = "Ghana" },
    { country_id = 66, code = "GR", name = "Greece" },
    { country_id = 67, code = "GD", name = "Grenada" },
    { country_id = 68, code = "GT", name = "Guatemala" },
    { country_id = 69, code = "GN", name = "Guinea" },
    { country_id = 70, code = "GW", name = "Guinea-Bissau" },
    { country_id = 71, code = "GY", name = "Guyana" },
    { country_id = 72, code = "HT", name = "Haiti" },
    { country_id = 73, code = "HN", name = "Honduras" },
    { country_id = 74, code = "HU", name = "Hungary" },
    { country_id = 75, code = "IS", name = "Iceland" },
    { country_id = 76, code = "IN", name = "India" },
    { country_id = 77, code = "ID", name = "Indonesia" },
    { country_id = 78, code = "IR", name = "Iran" },
    { country_id = 79, code = "IQ", name = "Iraq" },
    { country_id = 80, code = "IE", name = "Ireland" },
    { country_id = 81, code = "IL", name = "Israel" },
    { country_id = 82, code = "IT", name = "Italy" },
    { country_id = 83, code = "JM", name = "Jamaica" },
    { country_id = 84, code = "JP", name = "Japan" },
    { country_id = 85, code = "JO", name = "Jordan" },
    { country_id = 86, code = "KZ", name = "Kazakhstan" },
    { country_id = 87, code = "KE", name = "Kenya" },
    { country_id = 88, code = "KI", name = "Kiribati" },
    { country_id = 89, code = "KW", name = "Kuwait" },
    { country_id = 90, code = "KG", name = "Kyrgyzstan" },
    { country_id = 91, code = "LA", name = "Laos" },
    { country_id = 92, code = "LV", name = "Latvia" },
    { country_id = 93, code = "LB", name = "Lebanon" },
    { country_id = 94, code = "LS", name = "Lesotho" },
    { country_id = 95, code = "LR", name = "Liberia" },
    { country_id = 96, code = "LY", name = "Libya" },
    { country_id = 97, code = "LI", name = "Liechtenstein" },
    { country_id = 98, code = "LT", name = "Lithuania" },
    { country_id = 99, code = "LU", name = "Luxembourg" },
    { country_id = 100, code = "MG", name = "Madagascar" },
    { country_id = 101, code = "MW", name = "Malawi" },
    { country_id = 102, code = "MY", name = "Malaysia" },
    { country_id = 103, code = "MV", name = "Maldives" },
    { country_id = 104, code = "ML", name = "Mali" },
    { country_id = 105, code = "MT", name = "Malta" },
    { country_id = 106, code = "MH", name = "Marshall Islands" },
    { country_id = 107, code = "MR", name = "Mauritania" },
    { country_id = 108, code = "MU", name = "Mauritius" },
    { country_id = 109, code = "MX", name = "Mexico" },
    { country_id = 110, code = "FM", name = "Micronesia" },
    { country_id = 111, code = "MD", name = "Moldova" },
    { country_id = 112, code = "MC", name = "Monaco" },
    { country_id = 113, code = "MN", name = "Mongolia" },
    { country_id = 114, code = "ME", name = "Montenegro" },
    { country_id = 115, code = "MA", name = "Morocco" },
    { country_id = 116, code = "MZ", name = "Mozambique" },
    { country_id = 117, code = "MM", name = "Myanmar" },
    { country_id = 118, code = "NA", name = "Namibia" },
    { country_id = 119, code = "NR", name = "Nauru" },
    { country_id = 120, code = "NP", name = "Nepal" },
    { country_id = 121, code = "NL", name = "Netherlands" },
    { country_id = 122, code = "NZ", name = "New Zealand" },
    { country_id = 123, code = "NI", name = "Nicaragua" },
    { country_id = 124, code = "NE", name = "Niger" },
    { country_id = 125, code = "NG", name = "Nigeria" },
    { country_id = 126, code = "KP", name = "North Korea" },
    { country_id = 127, code = "MK", name = "North Macedonia" },
    { country_id = 128, code = "NO", name = "Norway" },
    { country_id = 129, code = "OM", name = "Oman" },
    { country_id = 130, code = "PK", name = "Pakistan" },
    { country_id = 131, code = "PW", name = "Palau" },
    { country_id = 132, code = "PS", name = "Palestine" },
    { country_id = 133, code = "PA", name = "Panama" },
    { country_id = 134, code = "PG", name = "Papua New Guinea" },
    { country_id = 135, code = "PY", name = "Paraguay" },
    { country_id = 136, code = "PE", name = "Peru" },
    { country_id = 137, code = "PH", name = "Philippines" },
    { country_id = 138, code = "PL", name = "Poland" },
    { country_id = 139, code = "PT", name = "Portugal" },
    { country_id = 140, code = "QA", name = "Qatar" },
    { country_id = 141, code = "RO", name = "Romania" },
    { country_id = 142, code = "RU", name = "Russia" },
    { country_id = 143, code = "RW", name = "Rwanda" },
    { country_id = 144, code = "KN", name = "Saint Kitts and Nevis" },
    { country_id = 145, code = "LC", name = "Saint Lucia" },
    { country_id = 146, code = "VC", name = "Saint Vincent and the Grenadines" },
    { country_id = 147, code = "WS", name = "Samoa" },
    { country_id = 148, code = "SM", name = "San Marino" },
    { country_id = 149, code = "ST", name = "Sao Tome and Principe" },
    { country_id = 150, code = "SA", name = "Saudi Arabia" },
    { country_id = 151, code = "SN", name = "Senegal" },
    { country_id = 152, code = "RS", name = "Serbia" },
    { country_id = 153, code = "SC", name = "Seychelles" },
    { country_id = 154, code = "SL", name = "Sierra Leone" },
    { country_id = 155, code = "SG", name = "Singapore" },
    { country_id = 156, code = "SK", name = "Slovakia" },
    { country_id = 157, code = "SI", name = "Slovenia" },
    { country_id = 158, code = "SB", name = "Solomon Islands" },
    { country_id = 159, code = "SO", name = "Somalia" },
    { country_id = 160, code = "ZA", name = "South Africa" },
    { country_id = 161, code = "KR", name = "South Korea" },
    { country_id = 162, code = "SS", name = "South Sudan" },
    { country_id = 163, code = "ES", name = "Spain" },
    { country_id = 164, code = "LK", name = "Sri Lanka" },
    { country_id = 165, code = "SD", name = "Sudan" },
    { country_id = 166, code = "SR", name = "Suriname" },
    { country_id = 167, code = "SE", name = "Sweden" },
    { country_id = 168, code = "CH", name = "Switzerland" },
    { country_id = 169, code = "SY", name = "Syria" },
    { country_id = 170, code = "TJ", name = "Tajikistan" },
    { country_id = 171, code = "TZ", name = "Tanzania" },
    { country_id = 172, code = "TH", name = "Thailand" },
    { country_id = 173, code = "TL", name = "Timor-Leste" },
    { country_id = 174, code = "TG", name = "Togo" },
    { country_id = 175, code = "TO", name = "Tonga" },
    { country_id = 176, code = "TT", name = "Trinidad and Tobago" },
    { country_id = 177, code = "TN", name = "Tunisia" },
    { country_id = 178, code = "TR", name = "Turkey" },
    { country_id = 179, code = "TM", name = "Turkmenistan" },
    { country_id = 180, code = "TV", name = "Tuvalu" },
    { country_id = 181, code = "UG", name = "Uganda" },
    { country_id = 182, code = "UA", name = "Ukraine" },
    { country_id = 183, code = "AE", name = "United Arab Emirates" },
    { country_id = 184, code = "GB", name = "United Kingdom" },
    { country_id = 185, code = "US", name = "United States" },
    { country_id = 186, code = "UY", name = "Uruguay" },
    { country_id = 187, code = "UZ", name = "Uzbekistan" },
    { country_id = 188, code = "VU", name = "Vanuatu" },
    { country_id = 189, code = "VA", name = "Vatican City" },
    { country_id = 190, code = "VE", name = "Venezuela" },
    { country_id = 191, code = "VN", name = "Vietnam" },
    { country_id = 192, code = "YE", name = "Yemen" },
    { country_id = 193, code = "ZM", name = "Zambia" },
    { country_id = 194, code = "ZW", name = "Zimbabwe" }
  ]
}

## Populate affordance types lookup table
data {
  table = table.affordance_types

  rows = [
    { affordance_type_id = 1, name = "BASE_ZONING" },
    { affordance_type_id = 2, name = "OVERLAY" },
    { affordance_type_id = 3, name = "VARIANCE" },
    { affordance_type_id = 4, name = "DEED_RESTRICTION" }
  ]
}