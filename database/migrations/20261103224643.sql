-- Drop index "idx_parcel_geometry_history_feature_id" from table: "parcel_geometry_history"
DROP INDEX "geo"."idx_parcel_geometry_history_feature_id";
-- Create index "idx_parcel_geometry_history_feature_id" to table: "parcel_geometry_history"
CREATE INDEX "idx_parcel_geometry_history_feature_id" ON "geo"."parcel_geometry_history" ("feature_id");
