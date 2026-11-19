-- Create index "idx_address_attributes_formatted_address_trgm" to table: "address_attributes"
CREATE INDEX "idx_address_attributes_formatted_address_trgm" ON "core"."address_attributes" USING GIN ((core.search_normalize(formatted_address)) extensions.gin_trgm_ops);
