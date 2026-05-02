schema "public" {
  comment = "standard public schema"
}

extension "postgis" {
  schema = schema.public
}

extension "btree_gist" {
  schema = schema.public
}

// Entity tables
table "parcels" {
  schema = schema.public

  column "parcel_id" {
    null = false
    type = bigint

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    null = false
    type = uuid
    default = sql("gen_random_uuid()")
  }

  column "legacy_id" {
    null = true
    type = text
  }

  primary_key {
    columns = [column.parcel_id]
  }
}

table "parcels_geometry" {
  schema = schema.public

  column "parcel_geometry_id" {
    null = false
    type = bigint

    identity {
      generated = ALWAYS
    }
  }

  column "parcel_id" {
    null = false
    type = bigint
  }

  column "geom_web" {
    null = false
    type = sql("geometry(MultiPolygon, 4326)")
  }

  column "geom_legal" {
    null = true
    type = sql("geometry(MultiPolygon)")
  }

  column "local_srid" {
    null = true
    type = int
  }

  column "legal_valid_range" {
    null = false
    type = tstzrange
  }

  column "system_updated_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  exclude "no_overlapping_legal_state" {
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

  primary_key {
    columns = [column.parcel_geometry_id]
  }

  foreign_key "fk_parcel_id" {
    columns = [column.parcel_id]
    ref_columns = [table.parcels.column.parcel_id]
    on_delete = CASCADE
  }

}

table "parcel_attributes" {
  schema = schema.public

  column "parcel_attribute_id" {
    null = false
    type = bigint

    identity {
      generated = ALWAYS
    }
  }

  column "parcel_id" {
    null = false
    type = bigint
  }

  column "owner_id" {
    null = false
    type = bigint
  }

  column "address_id" {
    null = false
    type = bigint
  }

  column "land_area_sq_m" {
    null = false
    type = double_precision
  }

  column "land_use_id" {
    null = true
    type = bigint
  }

  column "neighborhood_id" {
    null = true
    type = bigint
  }

  column "market_area_id" {
    null = true
    type = bigint
  }

  column "properties" {
    null = true
    type = jsonb
  }

  column "legal_valid_range" {
    null = false
    type = tstzrange
  }

  column "system_updated_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  exclude "no_overlapping_legal_state" {
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

  primary_key {
    columns = [column.parcel_attribute_id]
  }

  foreign_key "fk_parcel_id" {
    columns = [column.parcel_id]
    ref_columns = [table.parcels.column.parcel_id]
    on_delete = CASCADE
  }

  foreign_key "fk_owner_id" {
    columns = [column.owner_id]
    ref_columns = [table.owners.column.owner_id]
    on_delete = RESTRICT
  }

  foreign_key "fk_address_id" {
    columns = [column.address_id]
    ref_columns = [table.addresses.column.address_id]
    on_delete = RESTRICT
  }

  foreign_key "fk_land_use_id" {
    columns = [column.land_use_id]
    ref_columns = [table.land_uses.column.land_use_id]
    on_delete = RESTRICT
  }

  foreign_key "fk_neighborhood_id" {
    columns = [column.neighborhood_id]
    ref_columns = [table.neighborhoods.column.neighborhood_id]
    on_delete = RESTRICT
  }

  foreign_key "fk_market_area_id" {
    columns = [column.market_area_id]
    ref_columns = [table.market_areas.column.market_area_id]
    on_delete = RESTRICT
  }
}

table "parcel_affordances" {
  schema = schema.public

  column "parcel_affordance_id" {
    null = false
    type = bigint

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    null = false
    type = uuid
    default = sql("gen_random_uuid()")
  }

  column "parcel_id" {
    null = false
    type = bigint
  }

  column "zoning_id" {
    null = true
    type = bigint
  }

  column "affordance_type_id" {
    null = false
    type = smallint
  }

  column "source" {
    null = true
    type = text
  }

  column "precedence_rank" {
    null = false
    type = integer
    default = 100 
  }

  column "max_far" {
    null = true
    type = numeric(6,2)
  }

  column "min_lot_size_sq_m" {
    null = true
    type = numeric(6,2)
  }

  column "max_height_m" {
    null = true
    type = numeric(6,2)
  }

  column "legal_valid_range" {
    null = false
    type = tstzrange
  }

  column "system_updated_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  exclude "no_overlapping_legal_state" {
    type = GIST

    on { 
      column = column.parcel_affordance_id
      op = "=" 
    }
    on { 
      column = column.legal_valid_range 
      op = "&&" 
    }
  }
  
  primary_key {
    columns = [ column.parcel_affordance_id ]
  }

  foreign_key "fk_parcel_id" {
    columns = [column.parcel_id]
    ref_columns = [table.parcels.column.parcel_id]
    on_delete = CASCADE
  }

  foreign_key "fk_zoning_id" {
    columns = [column.zoning_id]
    ref_columns = [table.zoning.column.zoning_id]
    on_delete = RESTRICT
  }

  foreign_key "fk_affordance_type_id" {
    columns = [column.affordance_type_id]
    ref_columns = [table.affordance_types.column.affordance_type_id]
    on_delete = RESTRICT
  }

  check "require_origin" {
    expr = "zoning_id IS NOT NULL OR affordance_type_id != 1"
  }
}

table "improvements" {
  schema = schema.public

  column "improvement_id" {
    null = false
    type = bigint
  
    identity {
      generated = ALWAYS
    }  
  }

  column "public_id" {
    null = false
    type = uuid
    default = sql("gen_random_uuid()")
  }

  column "legacy_id" {
    null = true
    type = text
  }

  column "geom_web" {
    null = false
    type = sql("geometry(MultiPolygonZ, 4326)")
  }

  column "geom_legal" {
    null = true
    type = sql("geometry(MultiPolygonZ)")
  }

  column "local_horizontal_srid" {
    null = true
    type = int
  }

  column "local_vertical_datum" {
    null = true
    type = text
  }

  column "legal_valid_range" {
    null = false
    type = tstzrange
  }

  column "system_updated_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  index "idx_improvements_current_legal_state" {
    columns = [column.improvement_id]
    where   = "legal_valid_range @> now()"
  }

  exclude "no_overlapping_legal_state" {
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

  primary_key {
    columns = [column.improvement_id]
  }
}

table "improvement_attributes" {
  schema = schema.public

  column "improvement_id" {
    null = false
    type = bigint
  }

  column "owner_id" {
    null = false
    type = bigint
  }

  column "address_id" {
    null = false
    type = bigint
  }

  column "area_sq_m" {
    null = true
    type = double_precision
  }

  column "bathrooms" {
    null = true
    type = int
  }

  column "bedrooms" {
    null = true
    type = int
  }

  column "year_built" {
    null = true
    type = int
  }

  column "condition_num" {
    null = true
    type = int
  }

  column "units" {
    null = true
    type = int
  }

    column "properties" {
    null = true
    type = jsonb
  }

  column "legal_valid_range" {
    null = false
    type = tstzrange
  }

  column "system_updated_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  index "idx_improvement_attributes_current_legal_state" {
    columns = [column.improvement_id]
    where   = "legal_valid_range @> now()"
  }

  exclude "no_overlapping_legal_state" {
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

  primary_key {
    columns = [column.improvement_id]
  }

  foreign_key "fk_improvement_id" {
    columns = [column.improvement_id]
    ref_columns = [table.improvements.column.improvement_id]
    on_delete = CASCADE
  }

  foreign_key "fk_owner_id" {
    columns = [column.owner_id]
    ref_columns = [table.owners.column.owner_id]
    on_delete = RESTRICT
  }

  foreign_key "fk_address_id" {
    columns = [column.address_id]
    ref_columns = [table.addresses.column.address_id]
    on_delete = RESTRICT
  }
}

table "owners" {
  schema = schema.public

  column "owner_id" {
    null = false
    type = bigint

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    null = false
    type = uuid
    default = sql("gen_random_uuid()")
  }

  column "legacy_id" {
    null = true
    type = text
  }

  column "name" {
    null = false
    type = text
  }

  column "address_id" {
    null = true
    type = bigint
  }

  column "legal_valid_range" {
    null = false
    type = tstzrange
  }

  column "system_updated_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  index "idx_owners_current_legal_state" {
    columns = [column.owner_id]
    where   = "legal_valid_range @> now()"
  }

  exclude "no_overlapping_legal_state" {
    type = GIST

    on { 
      column = column.owner_id
      op = "=" 
    }
    on { 
      column = column.legal_valid_range 
      op = "&&" 
    }
  }

  primary_key {
    columns = [column.owner_id]
  }

  foreign_key "fk_address_id" {
    columns = [column.address_id]
    ref_columns = [table.addresses.column.address_id]
    on_delete = RESTRICT
  }
}

table "addresses" {
  schema = schema.public

  column "address_id" {
    null = false
    type = bigint

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    null = false
    type = uuid
    default = sql("gen_random_uuid()")
  }

  column "country_id" {
    null = false
    type = int
  }

  column "administrative_area" {
    null = true
    type = text
  }

  column "locality" {
    null = true
    type = text
  }

  column "sublocality" {
    null = true
    type = text
  }

  column "postal_code" {
    null = true
    type = text
  }

  column "address_line_1" {
    null = true
    type = text
  }

  column "address_line_2" {
    null = true
    type = text
  }

  column "address_line_3" {
    null = true
    type = text
  }

  column "formatted_address" {
    null = false
    type = text
  }

  column "coordinates" {
    null = false
    type = sql("geometry(Point, 4326)")
  }

  column "legal_valid_range" {
    null = false
    type = tstzrange
  }

  column "system_updated_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  index "idx_addresses_current_legal_state" {
    columns = [column.address_id]
    where   = "legal_valid_range @> now()"
  }

  exclude "no_overlapping_legal_state" {
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

  primary_key {
    columns = [column.address_id]
  }

  foreign_key "fk_country_id" {
    columns = [column.country_id]
    ref_columns = [table.countries.column.country_id]
    on_delete = RESTRICT
  }
}

table "zoning" {
  schema = schema.public

  column "zoning_id" {
    null = false
    type = bigint

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    null = false
    type = uuid
    default = sql("gen_random_uuid()")
  }

  column "code" {
    null = false
    type = text
  }

  column "name" {
    null = false
    type = text
  }

  column "max_far" {
    null = true
    type = numeric(6,2)
  }

  column "min_lot_size_sq_m" {
    null = true
    type = numeric(6,2)
  }

  column "max_height_m" {
    null = true
    type = numeric(6,2)
  }

  column "legal_valid_range" {
    null = false
    type = tstzrange
  }

  column "system_updated_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  index "idx_zoning_current_legal_state" {
    columns = [column.zoning_id]
    where   = "legal_valid_range @> now()"
  }

  exclude "no_overlapping_legal_state" {
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

  primary_key {
    columns = [column.zoning_id]
  }
}

// Event tables
table "sales" {
  schema = schema.public

  column "sale_id" {
    null = false
    type = bigint

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    null = false
    type = uuid
    default = sql("gen_random_uuid()")
  }

  column "legacy_id" {
    null = true
    type = text
  }

  column "seller_id" {
    null = false
    type = bigint
  }

  column "buyer_id" {
    null = false
    type = bigint
  }

  column "sale_date" {
    null = false
    type = timestamptz
  }

  column "sale_price" {
    null = false
    type = money
  }

  column "sale_deed_book" {
    null = true
    type = text
  }

  column "sale_deed_page" {
    null = true
    type = text
  }

  column "sale_deed_uri" {
    null = true
    type = text
  }

  column "system_updated_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  primary_key {
    columns = [column.sale_id]
  }

  foreign_key "fk_seller_id" {
    columns = [column.seller_id]
    ref_columns = [table.owners.column.owner_id]
    on_delete = RESTRICT
  }

  foreign_key "fk_buyer_id" {
    columns = [column.buyer_id]
    ref_columns = [table.owners.column.owner_id]
    on_delete = RESTRICT
  }
}

table "valuations" {
  schema = schema.public

  column "valuation_id" {
    null = false
    type = bigint    

    identity {
      generated = ALWAYS
    }
  }

  column "valuation_amount" {
    null = false
    type = money
  }

  column "valuation_date" {
    null = false
    type = timestamptz
  }

  column "system_updated_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  primary_key {
    columns = [column.valuation_id]
  }
}

// Linking tables
table "parcel_improvements" {
  schema = schema.public

  column "parcel_id" {
    null = false
    type = bigint
  }

  column "improvement_id" {
    null = false
    type = bigint
  }

  column "legal_valid_range" {
    null = false
    type = tstzrange
  }

  column "system_updated_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  # 3. Fast "Right Now" Index
  index "idx_parcel_improvements_current_legal_state" {
    columns = [column.parcel_id, column.improvement_id]
    where   = "legal_valid_range @> now()"
  }

  foreign_key "fk_parcel_id" {
    columns = [column.parcel_id]
    ref_columns = [table.parcels.column.parcel_id]
    on_delete = CASCADE
  }

  foreign_key "fk_improvement_id" {
    columns = [column.improvement_id]
    ref_columns = [table.improvements.column.improvement_id]
    on_delete = CASCADE
  }

  exclude "no_overlapping_legal_link" {
    type = GIST
    
    on { 
      column = column.parcel_id
      op = "=" 
    }
    on { 
      column = column.improvement_id
      op = "=" 
    }
    on { 
      column = column.legal_valid_range 
      op = "&&" 
    }
  }

  # Note: A standard Primary Key doesn't work well here anymore because
  # the same parcel-improvement pair will exist multiple times in history.
  # The exclusion constraint above acts as your temporal primary key.

}

table "parcel_sales" {
  schema = schema.public

  column "parcel_id" {
    null = false
    type = bigint
  }

  column "sale_id" {
    null = false
    type = bigint
  }

  column "system_updated_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  primary_key {
    columns = [column.parcel_id, column.sale_id]
  }

  foreign_key "fk_parcel_id" {
    columns = [column.parcel_id]
    ref_columns = [table.parcels.column.parcel_id]
    on_delete = CASCADE
  }

  foreign_key "fk_sale_id" {
    columns = [column.sale_id]
    ref_columns = [table.sales.column.sale_id]
    on_delete = CASCADE
  }

}

table "improvement_sales" {
  schema = schema.public

  column "improvement_id" {
    null = false
    type = bigint
  }

  column "sale_id" {
    null = false
    type = bigint
  }

  column "system_updated_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  primary_key {
    columns = [column.improvement_id, column.sale_id]
  }

  foreign_key "fk_improvement_id" {
    columns = [column.improvement_id]
    ref_columns = [table.improvements.column.improvement_id]
    on_delete = CASCADE
  }

  foreign_key "fk_sale_id" {
    columns = [column.sale_id]
    ref_columns = [table.sales.column.sale_id]
    on_delete = CASCADE
  }

}

table "parcel_valuations" {
  schema = schema.public

  column "parcel_id" {
    null = false
    type = bigint
  }

  column "valuation_id" {
    null = false
    type = bigint
  }

  column "system_updated_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }  

  primary_key {
    columns = [column.parcel_id, column.valuation_id]
  }

  foreign_key "fk_parcel_id" {
    columns = [column.parcel_id]
    ref_columns = [table.parcels.column.parcel_id]
    on_delete = CASCADE
  }

  foreign_key "fk_valuation_id" {
    columns = [column.valuation_id]
    ref_columns = [table.valuations.column.valuation_id]
    on_delete = CASCADE
  }

}

table "improvement_valuations" {
  schema = schema.public

  column "improvement_id" {
    null = false
    type = bigint
  }

  column "valuation_id" {
    null = false
    type = bigint
  }

  column "system_updated_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  primary_key {
    columns = [column.improvement_id, column.valuation_id]
  }

  foreign_key "fk_improvement_id" {
    columns = [column.improvement_id]
    ref_columns = [table.improvements.column.improvement_id]
    on_delete = CASCADE
  }

  foreign_key "fk_valuation_id" {
    columns = [column.valuation_id]
    ref_columns = [table.valuations.column.valuation_id]
    on_delete = CASCADE
  }

}

// Lookup tables
table "countries" {
  schema = schema.public

  column "country_id" {
    null = false
    type = int

    identity {
      generated = ALWAYS
    }
  }

  column "code" {
    null = false
    type = varchar(2)
  }

  column "name" {
    null = false
    type = varchar(100)
  }

  primary_key {
    columns = [column.country_id]
  }
}

table "land_uses" {
  schema = schema.public

  column "land_use_id" {
    null = false
    type = bigint

    identity {
      generated = ALWAYS
    }
  }

  column "public_id" {
    null = false
    type = uuid
    default = sql("gen_random_uuid()")
  }

  column "name" {
    null = false
    type = text
  }

  primary_key {
    columns = [column.land_use_id]
  }
}

table "neighborhoods" {
  schema = schema.public

  column "neighborhood_id" {
    null = false
    type = bigint

    identity {
      generated = ALWAYS
    }
  }

  column "name" {
    null = false
    type = text
  }

  primary_key {
    columns = [column.neighborhood_id]
  }
}

table "market_areas" {
  schema = schema.public

  column "market_area_id" {
    null = false
    type = bigint
  }

  column "name" {
    null = false
    type = text
  }

  primary_key {
    columns = [column.market_area_id]
  }
}

table "affordance_types" {
  schema = schema.public

  column "affordance_type_id" {
    null = false
    type = smallint
  }

  column "name" {
    null = false
    type = text
  }

  primary_key {
    columns = [column.affordance_type_id]
  }  
}

// History (shadow) tables
table "parcels_geometry_history" {
  schema = schema.public

  column "parcel_geometry_history_id" {
    null = false
    type = bigint

    identity {
      generated = ALWAYS
    }
  }

  column "parcel_id" {
    null = false
    type = bigint
  }

  column "public_id" {
    null = false
    type = uuid
  }

  column "legacy_id" {
    null = true
    type = text
  }

  column "geom_web" {
    null = false
    type = sql("geometry(MultiPolygon, 4326)")
  }

  column "geom_legal" {
    null = true
    type = sql("geometry(MultiPolygon)")
  }

  column "local_srid" {
    null = true
    type = int
  }

  column "valid_range" {
    null = false
    type = tstzrange
  }

  primary_key {
    columns = [column.parcel_history_id]
  }

  index "idx_parcels_history_parcel_id" {
    columns = [column.parcel_id]
  }

  exclude "no_overlapping_history" {
    # GIST is required for range, but btree_gist allows for including the bigint id
    # in the constraint as well
    type = GIST
    
    on {
      column = column.parcel_id
      op     = "="
    }
    on {
      column = column.valid_range
      op     = "&&"
    }
  }
}

table "parcel_attributes_history" {
  schema = schema.public

  column "parcel_attributes_history_id" {
    null = false
    type = bigint

   
    identity {
      generated = ALWAYS
    }
  }

  column "parcel_id" {
    null = false
    type = bigint
  }

  column "owner_id" {
    null = false
    type = bigint
  }

  column "address_id" {
    null = false
    type = bigint
  }

  column "land_area_sq_m" {
    null = false
    type = double_precision
  }

  column "land_use_id" {
    null = true
    type = bigint
  }

  column "neighborhood_id" {
    null = true
    type = bigint
  }

  column "market_area_id" {
    null = true
    type = bigint
  }

  column "properties" {
    null = true
    type = jsonb
  }

  column "valid_range" {
    null = false
    type = tstzrange
  }

  primary_key {
    columns = [column.parcel_attributes_history_id]
  }

  index "idx_parcels_history_parcel_id" {
    columns = [column.parcel_id]
  }

  index "idx_parcel_attributes_history_owner_id" {
    columns = [column.owner_id]
  }

  index "idx_parcel_attributes_history_land_use_id" {
    columns = [column.land_use_id]
  }

  index "idx_parcel_attributes_history_neighborhood_id" {
    columns = [column.neighborhood_id]
  }

  index "idx_parcel_attributes_history_market_area_id" {
    columns = [column.market_area_id]
  }

  exclude "no_overlapping_history" {
    # GIST is required for range, but btree_gist allows for including the bigint id
    # in the constraint as well
    type = GIST
    
    on {
      column = column.parcel_id
      op     = "="
    }
    
    on {
      column = column.valid_range
      op     = "&&"
    }
  }
}

table "parcel_affordances_history" {
  schema = schema.public

  column "parcel_affordance_history_id" {
    null = false
    type = bigint

    identity {
      generated = ALWAYS
    }
  }

  column "parcel_affordance_id" {
    null = false
    type = bigint
  }

  column "public_id" {
    null = false
    type = uuid
  }

  column "parcel_id" {
    null = false
    type = bigint
  }

  column "zoning_id" {
    null = true
    type = bigint
  }

  column "affordance_type_id" {
    null = false
    type = smallint
  }

  column "source" {
    null = true
    type = text
  }

  column "precedence_rank" {
    null = false
    type = integer
    default = 100 
  }

  column "max_far" {
    null = true
    type = numeric(6,2)
  }

  column "min_lot_size_sq_m" {
    null = true
    type = numeric(6,2)
  }

  column "max_height_m" {
    null = true
    type = numeric(6,2)
  }

  column "valid_range" {
    null = false
    type = tstzrange
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

  index "idx_parcel_affordances_history_zoning_id" {
    columns = [column.zoning_id]
  }

  index "idx_parcel_affordances_history_affordance_type_id" {
    columns = [column.affordance_type_id]
  }  

  check "require_origin" {
    expr = "zoning_id IS NOT NULL OR affordance_type != 'BASE_ZONING'"
  }

  # Prevent the exact same zone from applying to the same parcel twice at the same time
  exclude "no_overlapping_history" {
    # GIST is required for range, but btree_gist allows for including the bigint id
    # in the constraint as well
    type = GIST
    
    on {
      column = column.parcel_affordance_id
      op     = "="
    }
    on {
      column = column.valid_range
      op     = "&&"
    }
  }
}

table "improvements_history" {
  schema = schema.public

  column "improvement_history_id" {
    null = false
    type = bigint

    identity {
      generated = ALWAYS
    }
  }

  column "improvement_id" {
    null = false
    type = bigint
  }

  column "public_id" {
    null = false
    type = uuid
  }

  column "legacy_id" {
    null = true
    type = text
  }

  column "geom_web" {
    null = false
    type = sql("geometry(MultiPolygonZ, 4326)")
  }

  column "geom_legal" {
    null = true
    type = sql("geometry(MultiPolygonZ)")
  }

  column "local_horizontal_srid" {
    null = true
    type = int
  }

  column "local_vertical_datum" {
    null = true
    type = text
  }

  column "valid_range" {
    null = false
    type = tstzrange
  }

  primary_key {
    columns = [column.improvement_history_id]
  }

  index "idx_improvements_history_improvement_id" {
    columns = [column.improvement_id]
  }

  exclude "no_overlapping_history" {
    # GIST is required for range, but btree_gist allows for including the bigint id
    # in the constraint as well
    type = GIST
    
    on {
      column = column.improvement_id
      op     = "="
    }
    on {
      column = column.valid_range
      op     = "&&"
    }
  }
}

table "improvement_attributes_history" {
  schema = schema.public

  column "improvement_attributes_history_id" {
    null = false
    type = bigint

    identity {
      generated = ALWAYS
    }
  }

  column "improvement_id" {
    null = false
    type = bigint
  }

  column "owner_id" {
    null = false
    type = bigint
  }

  column "address_id" {
    null = false
    type = bigint
  }

  column "area_sq_m" {
    null = true
    type = double_precision
  }

  column "bathrooms" {
    null = true
    type = int
  }

  column "bedrooms" {
    null = true
    type = int
  }

  column "year_built" {
    null = true
    type = int
  }

  column "condition_num" {
    null = true
    type = int
  }

  column "units" {
    null = true
    type = int
  }

    column "properties" {
    null = true
    type = jsonb
  }

  column "valid_range" {
    null = false
    type = tstzrange
  }

  primary_key {
    columns = [column.improvement_attributes_history_id]
  }

  index "idx_improvement_attributes_history_improvement_id" {
    columns = [column.improvement_id]
  }

  index "idx_improvement_attributes_history_owner_id" {
    columns = [column.owner_id]
  }

  exclude "no_overlapping_history" {
    # GIST is required for range, but btree_gist allows for including the bigint id
    # in the constraint as well
    type = GIST
    
    on {
      column = column.improvement_id
      op     = "="
    }
    on {
      column = column.valid_range
      op     = "&&"
    }
  }
}

table "owners_history" {
  schema = schema.public

  column "owner_history_id" {
    null = false
    type = bigint

    identity {
      generated = ALWAYS
    }
  }

  column "owner_id" {
    null = false
    type = bigint
  }

  column "public_id" {
    null = false
    type = uuid
  }

  column "legacy_id" {
    null = true
    type = text
  }

  column "name" {
    null = false
    type = text
  }

  column "address_id" {
    null = true
    type = bigint
  }

  column "valid_range" {
    null = false
    type = tstzrange
  }

  primary_key {
    columns = [column.owner_history_id]
  }

  index "idx_owners_history_owner_id" {
    columns = [column.owner_id]
  }

  exclude "no_overlapping_history" {
    # GIST is required for range, but btree_gist allows for including the bigint id
    # in the constraint as well
    type = GIST
    
    on {
      column = column.owner_id
      op     = "="
    }
    on {
      column = column.valid_range
      op     = "&&"
    }
  }
}

table "addresses_history" {
  schema = schema.public

  column "address_history_id" {
    null = false
    type = bigint

    identity {
      generated = ALWAYS
    }
  }

  column "address_id" {
    null = false
    type = bigint
  }

  column "public_id" {
    null = false
    type = uuid
  }

  column "country_id" {
    null = false
    type = int
  }

  column "administrative_area" {
    null = true
    type = text
  }

  column "locality" {
    null = true
    type = text
  }

  column "sublocality" {
    null = true
    type = text
  }

  column "postal_code" {
    null = true
    type = text
  }

  column "address_line_1" {
    null = true
    type = text
  }

  column "address_line_2" {
    null = true
    type = text
  }

  column "address_line_3" {
    null = true
    type = text
  }

  column "formatted_address" {
    null = false
    type = text
  }

  column "coordinates" {
    null = false
    type = sql("geometry(Point, 4326)")
  }

  column "valid_range" {
    null = false
    type = tstzrange
  }

  primary_key {
    columns = [column.address_history_id]
  }

  index "idx_addresses_history_address_id" {
    columns = [column.address_id]
  }

  exclude "no_overlapping_history" {
    # GIST is required for range, but btree_gist allows for including the bigint id
    # in the constraint as well
    type = GIST
    
    on {
      column = column.address_id
      op     = "="
    }
    on {
      column = column.valid_range
      op     = "&&"
    }
  }
}

table "zoning_history" {
  schema = schema.public

  column "zoning_history_id" {
    null = false
    type = bigint

    identity {
      generated = ALWAYS
    }
  }

  column "zoning_id" {
    null = false
    type = bigint
  }

  column "public_id" {
    null = false
    type = uuid
  }

  column "code" {
    null = false
    type = text
  }

  column "name" {
    null = false
    type = text
  }

  column "max_far" {
    null = true
    type = numeric(6,2)
  }

  column "min_lot_size_sq_m" {
    null = true
    type = numeric(6,2)
  }

  column "max_height_m" {
    null = true
    type = numeric(6,2)
  }

  column "valid_range" {
    null = false
    type = tstzrange
  }

  primary_key {
    columns = [column.zoning_history_id]
  }

  index "idx_zoning_history_zoning_id" {
    columns = [column.zoning_id]
  }

  exclude "no_overlapping_history" {
    # GIST is required for range, but btree_gist allows for including the bigint id
    # in the constraint as well
    type = GIST
    
    on {
      column = column.zoning_id
      op     = "="
    }
    on {
      column = column.valid_range
      op     = "&&"
    }
  }  
}

table "sales_history" {

}

table "assessments_history" {

}

table "parcel_improvements_history" {

}

table "parcel_sales_history" {

}

table "improvement_sales_history" {

}

table "parcel_valuations_history" {

}

table "improvement_valuations_history" {

}

## Triggers
trigger "record_parcel_history" {
  # Attach it to your main, current-state table
  on = table.parcels
  
  # Fire AFTER the transaction is validated
  after {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function defined above
  execute {
    function = function.record_parcel_history
  } 
}

trigger "record_parcel_attributes_history" {
  # Attach it to your main, current-state table
  on = table.parcel_attributes
  
  # Fire AFTER the transaction is validated
  after {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function defined above
  execute {
    function = function.record_parcel_attributes_history
  } 
}

trigger "record_parcel_affordances_history" {
  # Attach it to your main, current-state table
  on = table.parcel_affordances
  
  # Fire AFTER the transaction is validated
  after {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function defined above
  execute {
    function = function.record_parcel_affordances_history
  } 
}

trigger "record_improvements_history" {
  # Attach it to your main, current-state table
  on = table.improvements
  
  # Fire AFTER the transaction is validated
  after {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function defined above
  execute {
    function = function.record_improvements_history
  } 
}

trigger "record_improvement_attributes_history" {
  # Attach it to your main, current-state table
  on = table.improvement_attributes
  
  # Fire AFTER the transaction is validated
  after {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function defined above
  execute {
    function = function.record_improvement_attributes_history
  } 
}

trigger "record_owners_history" {
  # Attach it to your main, current-state table
  on = table.owners
  
  # Fire AFTER the transaction is validated
  after {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function defined above
  execute {
    function = function.record_owners_history
  } 
}

trigger "record_addresses_history" {
  # Attach it to your main, current-state table
  on = table.addresses
  
  # Fire AFTER the transaction is validated
  after {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function defined above
  execute {
    function = function.record_addresses_history
  } 
}

trigger "record_zoning_history" {
  # Attach it to your main, current-state table
  on = table.zoning
  
  # Fire AFTER the transaction is validated
  after {
    insert = false
    update = true
    delete = true
  }

  for = ROW
  
  # Point to the function defined above
  execute {
    function = function.record_zoning_history
  } 
}

## Functions
function "record_parcel_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = "trigger"
  
  as = <<-SQL
      BEGIN
        IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
          INSERT INTO parcels_history (
            parcel_id,
            public_id,
            legacy_id,
            geom_web,
            geom_legal,
            local_srid, 
            valid_range
          ) VALUES (
            OLD.parcel_id,
            OLD.public_id,
            OLD.legacy_id,
            OLD.geom_web,
            OLD.geom_legal,
            OLD.local_srid, 
            tstzrange(OLD.updated_at, now(), '[)')
          );
          
          -- Safely route the return pointer
          IF (TG_OP = 'DELETE') THEN
              RETURN OLD;
          ELSE
              RETURN NEW;
          END IF;
        END IF;
        
        RETURN NULL;
      END;
    SQL
}

function "record_parcel_attributes_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = "trigger"
  
  as = <<-SQL
      BEGIN
        IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
          INSERT INTO parcel_attributes_history (
            parcel_id,
            owner_id,
            address_id,
            land_area_sq_m,
            land_use_id,
            neighborhood_id,
            market_area_id,
            properties,
            valid_range
          ) VALUES (
            OLD.parcel_id,
            OLD.owner_id,
            OLD.address_id,
            OLD.land_area_sq_m,
            OLD.land_use_id,
            OLD.neighborhood_id,
            OLD.market_area_id,
            OLD.properties,
            tstzrange(OLD.updated_at, now(), '[)')
          );
          
          -- Safely route the return pointer
          IF (TG_OP = 'DELETE') THEN
              RETURN OLD;
          ELSE
              RETURN NEW;
          END IF;
        END IF;
        
        RETURN NULL;
      END;
    SQL
}

function "record_parcel_affordances_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = "trigger"
  
  as = <<-SQL
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
            valid_range
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
            tstzrange(OLD.updated_at, now(), '[)')
          );
          
          -- Safely route the return pointer
          IF (TG_OP = 'DELETE') THEN
              RETURN OLD;
          ELSE
              RETURN NEW;
          END IF;
        END IF;
        
        RETURN NULL;
      END;
    SQL
}

function "record_improvements_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = "trigger"
  
  as = <<-SQL
      BEGIN
        IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
          INSERT INTO improvements_history (
            improvement_id,
            public_id,
            legacy_id,
            geom_web,
            geom_legal,
            local_horizontal_srid,
            local_vertical_datum,
            valid_range
          ) VALUES (
            OLD.improvement_id,
            OLD.public_id,
            OLD.legacy_id,
            OLD.geom_web,
            OLD.geom_legal,
            OLD.local_horizontal_srid,
            OLD.local_vertical_datum,
            tstzrange(OLD.updated_at, now(), '[)')
          );
          
          -- Safely route the return pointer
          IF (TG_OP = 'DELETE') THEN
              RETURN OLD;
          ELSE
              RETURN NEW;
          END IF;
        END IF;
        
        RETURN NULL;
      END;
    SQL
}

function "record_improvement_attributes_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = "trigger"
  
  as = <<-SQL
      BEGIN
        IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
          INSERT INTO improvement_attributes_history (
            improvement_id,
            public_id,
            legacy_id,
            geom_web,
            geom_legal,
            local_horizontal_srid,
            local_vertical_datum,
            valid_range
          ) VALUES (
            OLD.improvement_id,
            OLD.public_id,
            OLD.legacy_id,
            OLD.geom_web,
            OLD.geom_legal,
            OLD.local_horizontal_srid,
            OLD.local_vertical_datum,
            tstzrange(OLD.updated_at, now(), '[)')
          );
          
          -- Safely route the return pointer
          IF (TG_OP = 'DELETE') THEN
              RETURN OLD;
          ELSE
              RETURN NEW;
          END IF;
        END IF;
        
        RETURN NULL;
      END;
    SQL
}

function "record_owners_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = "trigger"
  
  as = <<-SQL
      BEGIN
        IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
          INSERT INTO owners_history (
            owner_id,
            public_id,
            legacy_id,
            name,
            address_id,
            valid_range
          ) VALUES (
            OLD.owner_id,
            OLD.public_id,
            OLD.legacy_id,
            OLD.name,
            OLD.address_id,
            tstzrange(OLD.updated_at, now(), '[)')
          );
          
          -- Safely route the return pointer
          IF (TG_OP = 'DELETE') THEN
              RETURN OLD;
          ELSE
              RETURN NEW;
          END IF;
        END IF;
        
        RETURN NULL;
      END;
    SQL
}

function "record_addresses_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = "trigger"
  
  as = <<-SQL
      BEGIN
        IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
          INSERT INTO addresses_history (
            address_id,
            public_id,
            country_id,
            administrative_area,
            locality,
            sublocality,
            postal_code,
            address_line_1,
            address_line_2,
            address_line_3,
            formatted_address,
            coordinates,
            valid_range
          ) VALUES (
            OLD.address_id,
            OLD.public_id,
            OLD.country_id,
            OLD.administrative_area,
            OLD.locality,
            OLD.sublocality,
            OLD.postal_code,
            OLD.address_line_1,
            OLD.address_line_2,
            OLD.address_line_3,
            OLD.formatted_address,
            OLD.coordinates,
            tstzrange(OLD.updated_at, now(), '[)')
          );
          
          -- Safely route the return pointer
          IF (TG_OP = 'DELETE') THEN
              RETURN OLD;
          ELSE
              RETURN NEW;
          END IF;
        END IF;
        
        RETURN NULL;
      END;
    SQL
}

function "record_zoning_history" {
  schema = schema.public
  lang   = "plpgsql"
  return = "trigger"
  
  as = <<-SQL
      BEGIN
        IF (TG_OP = 'UPDATE' OR TG_OP = 'DELETE') THEN
          INSERT INTO zoning_history (
            zoning_id,
            public_id,
            code,
            name,
            max_far,
            min_lot_size_sq_m,
            max_height_m,
            valid_range
          ) VALUES (
            OLD.zoning_id,
            OLD.public_id,
            OLD.code,
            OLD.name,
            OLD.max_far,
            OLD.min_lot_size_sq_m,
            OLD.max_height_m,
            tstzrange(OLD.updated_at, now(), '[)')
          );
          
          -- Safely route the return pointer
          IF (TG_OP = 'DELETE') THEN
              RETURN OLD;
          ELSE
              RETURN NEW;
          END IF;
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

## Data

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