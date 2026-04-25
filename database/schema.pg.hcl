schema "public" {
  comment = "standard public schema"
}

extension "postgis" {
  schema = schema.public
}

// Entity tables

table "parcels" {
  schema = schema.public

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

  column "updated_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  primary_key {
    columns = [column.parcel_id]
  }
}

table "parcel_attributes" {
  schema = schema.public

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

  column "zoning_id" {
    null = true
    type = bigint
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

  column "updated_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  primary_key {
    columns = [column.parcel_id]
  }

  foreign_key "fk_parcel_id" {
    columns = [column.parcel_id]
    ref_columns = [table.parcels.column.parcel_id]
  }

  foreign_key "fk_owner_id" {
    columns = [column.owner_id]
    ref_columns = [table.owners.column.owner_id]
  }

  foreign_key "fk_address_id" {
    columns = [column.address_id]
    ref_columns = [table.addresses.column.address_id]
  }

  foreign_key "fk_zoning_id" {
    columns = [column.zoning_id]
    ref_columns = [table.zoning.column.zoning_id]
  }

  foreign_key "fk_land_use_id" {
    columns = [column.land_use_id]
    ref_columns = [table.land_uses.column.land_use_id]
  }

  foreign_key "fk_neighborhood_id" {
    columns = [column.neighborhood_id]
    ref_columns = [table.neighborhoods.column.neighborhood_id]
  }

  foreign_key "fk_market_area_id" {
    columns = [column.market_area_id]
    ref_columns = [table.market_areas.column.market_area_id]
  }
}

table "improvements" {
  schema = schema.public

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

  column "updated_at" {
    null = false
    default = sql("now()")
    type = timestamptz
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

  column "updated_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  primary_key {
    columns = [column.improvement_id]
  }

  foreign_key "fk_improvement_id" {
    columns = [column.improvement_id]
    ref_columns = [table.improvements.column.improvement_id]
  }

  foreign_key "fk_owner_id" {
    columns = [column.owner_id]
    ref_columns = [table.owners.column.owner_id]
  }

  foreign_key "fk_address_id" {
    columns = [column.address_id]
    ref_columns = [table.addresses.column.address_id]
  }
}

table "owners" {
  schema = schema.public

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

  column "updated_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  primary_key {
    columns = [column.owner_id]
  }

  foreign_key "fk_address_id" {
    columns = [column.address_id]
    ref_columns = [table.addresses.column.address_id]
  }
}

table "addresses" {
  schema = schema.public

  column "address_id" {
    null = false
    type = bigint
  }

  column "public_id" {
    null = false
    type = uuid
  }

  column "country_code_id" {
    null = false
    type = bigint
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

  column "updated_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  primary_key {
    columns = [column.address_id]
  }

  foreign_key "fk_country_code_id" {
    columns = [column.country_code_id]
    ref_columns = [table.country_codes.column.country_code_id]
  }
}

// Event tables
table "sales" {
  schema = schema.public

  column "sale_id" {
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

  primary_key {
    columns = [column.sale_id]
  }

  foreign_key "fk_seller_id" {
    columns = [column.seller_id]
    ref_columns = [table.owners.column.owner_id]
  }

  foreign_key "fk_buyer_id" {
    columns = [column.buyer_id]
    ref_columns = [table.owners.column.owner_id]
  }
}

table "valuations" {
  schema = schema.public

  column "valuation_id" {
    null = false
    type = bigint    
  }

  column "valuation_amount" {
    null = false
    type = money
  }

  column "valuation_date" {
    null = false
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

  primary_key {
    columns = [column.parcel_id, column.improvement_id]
  }


  foreign_key "fk_parcel_id" {
    columns = [column.parcel_id]
    ref_columns = [table.parcels.column.parcel_id]
  }

  foreign_key "fk_improvement_id" {
    columns = [column.improvement_id]
    ref_columns = [table.improvements.column.improvement_id]
  }
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

  primary_key {
    columns = [column.parcel_id, column.sale_id]
  }

  foreign_key "fk_parcel_id" {
    columns = [column.parcel_id]
    ref_columns = [table.parcels.column.parcel_id]
  }

  foreign_key "fk_sale_id" {
    columns = [column.sale_id]
    ref_columns = [table.sales.column.sale_id]
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

  primary_key {
    columns = [column.improvement_id, column.sale_id]
  }

  foreign_key "fk_improvement_id" {
    columns = [column.improvement_id]
    ref_columns = [table.improvements.column.improvement_id]
  }

  foreign_key "fk_sale_id" {
    columns = [column.sale_id]
    ref_columns = [table.sales.column.sale_id]
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

  primary_key {
    columns = [column.parcel_id, column.valuation_id]
  }

  foreign_key "fk_parcel_id" {
    columns = [column.parcel_id]
    ref_columns = [table.parcels.column.parcel_id]
  }

  foreign_key "fk_valuation_id" {
    columns = [column.valuation_id]
    ref_columns = [table.valuations.column.valuation_id]
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

  primary_key {
    columns = [column.improvement_id, column.valuation_id]
  }

  foreign_key "fk_improvement_id" {
    columns = [column.improvement_id]
    ref_columns = [table.improvements.column.improvement_id]
  }

  foreign_key "fk_valuation_id" {
    columns = [column.valuation_id]
    ref_columns = [table.valuations.column.valuation_id]
  }

}

// Lookup tables
table "country_codes" {
  schema = schema.public

  column "country_code_id" {
    null = false
    type = bigint
  }

  column "country_code" {
    null = false
    type = char(2)
  }

  primary_key {
    columns = [column.country_code_id]
  }
}

table "zoning" {
  schema = schema.public

  column "zoning_id" {
    null = false
    type = bigint
  }

  column "name" {
    null = false
    type = text
  }

  primary_key {
    columns = [column.zoning_id]
  }
}

table "land_uses" {
  schema = schema.public

  column "land_use_id" {
    null = false
    type = bigint
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

// History (shadow) tables
table "parcels_history" {
  schema = schema.public

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

  column "start_at" {
    null = false
    type = timestamptz
  }

  column "end_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  primary_key {
    columns = [column.parcel_id]
  }

  foreign_key "fk_parcel_id" {
    columns = [column.parcel_id]
    ref_columns = [table.parcels.column.parcel_id]
  }

}

table "parcel_attributes_history" {
  schema = schema.public

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

  column "zoning_id" {
    null = true
    type = bigint
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

  column "start_at" {
    null = false
    type = timestamptz
  }

  column "end_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  primary_key {
    columns = [column.parcel_id]
  }

  foreign_key "fk_parcel_id" {
    columns = [column.parcel_id]
    ref_columns = [table.parcels.column.parcel_id]
  }

  foreign_key "fk_owner_id" {
    columns = [column.owner_id]
    ref_columns = [table.owners.column.owner_id]
  }

  foreign_key "fk_address_id" {
    columns = [column.address_id]
    ref_columns = [table.addresses.column.address_id]
  }

  foreign_key "fk_zoning_id" {
    columns = [column.zoning_id]
    ref_columns = [table.zoning.column.zoning_id]
  }

  foreign_key "fk_land_use_id" {
    columns = [column.land_use_id]
    ref_columns = [table.land_uses.column.land_use_id]
  }

  foreign_key "fk_neighborhood_id" {
    columns = [column.neighborhood_id]
    ref_columns = [table.neighborhoods.column.neighborhood_id]
  }

  foreign_key "fk_market_area_id" {
    columns = [column.market_area_id]
    ref_columns = [table.market_areas.column.market_area_id]
  }
}

table "improvements_history" {
  schema = schema.public

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

  column "start_at" {
    null = false
    type = timestamptz
  }

  column "end_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  primary_key {
    columns = [column.improvement_id]
  }

  foreign_key "fk_improvement_id" {
    columns = [column.improvement_id]
    ref_columns = [table.improvements.column.improvement_id]
  }
}

table "improvement_attributes_history" {
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

  column "start_at" {
    null = false
    type = timestamptz
  }

  column "end_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  primary_key {
    columns = [column.improvement_id]
  }

  foreign_key "fk_improvement_id" {
    columns = [column.improvement_id]
    ref_columns = [table.improvements.column.improvement_id]
  }

  foreign_key "fk_owner_id" {
    columns = [column.owner_id]
    ref_columns = [table.owners.column.owner_id]
  }

  foreign_key "fk_address_id" {
    columns = [column.address_id]
    ref_columns = [table.addresses.column.address_id]
  }
}

table "owners_history" {
  schema = schema.public

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

  column "start_at" {
    null = false
    type = timestamptz
  }

  column "end_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  primary_key {
    columns = [column.owner_id]
  }

  foreign_key "fk_owner_id" {
    columns = [column.owner_id]
    ref_columns = [table.owners.column.owner_id]
  }


  foreign_key "fk_address_id" {
    columns = [column.address_id]
    ref_columns = [table.addresses.column.address_id]
  }
}

table "addresses_history" {
  schema = schema.public

  column "address_id" {
    null = false
    type = bigint
  }

  column "public_id" {
    null = false
    type = uuid
  }

  column "country_code_id" {
    null = false
    type = bigint
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

  column "start_at" {
    null = false
    type = timestamptz
  }

  column "end_at" {
    null = false
    default = sql("now()")
    type = timestamptz
  }

  primary_key {
    columns = [column.address_id]
  }

  foreign_key "fk_address_id" {
    columns = [column.address_id]
    ref_columns = [table.addresses.column.address_id]
  }

  foreign_key "fk_country_code_id" {
    columns = [column.country_code_id]
    ref_columns = [table.country_codes.column.country_code_id]
  }
}