#!/bin/sh
# Applies database/migrations/*.sql in filename order (the order Atlas uses)
# to the "civil" database, once each. Local development only: production
# applies migrations with Atlas. Run by the compose "migrate" service.
set -eu

DB=civil

# Create the database from template0: the PostGIS image installs PostGIS into
# template1, which would clash with the migrations' own CREATE EXTENSION.
if [ "$(psql -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname = '$DB'")" != 1 ]; then
  psql -v ON_ERROR_STOP=1 -d postgres -c "CREATE DATABASE $DB TEMPLATE template0"
fi

# Production infrastructure sets this; the reader relies on it.
psql -v ON_ERROR_STOP=1 -d postgres -c "ALTER DATABASE $DB SET search_path = core, geo, extensions, public"

psql -v ON_ERROR_STOP=1 -d "$DB" -q -c "
  CREATE SCHEMA IF NOT EXISTS civil_os_dev;
  CREATE TABLE IF NOT EXISTS civil_os_dev.applied_migrations (
    file text PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
  );"

for f in /migrations/*.sql; do
  name=$(basename "$f")
  if [ "$(psql -d "$DB" -tAc "SELECT 1 FROM civil_os_dev.applied_migrations WHERE file = '$name'")" = 1 ]; then
    echo "migrate: $name already applied"
    continue
  fi
  echo "migrate: applying $name"
  psql -v ON_ERROR_STOP=1 -d "$DB" -q --single-transaction \
    -f "$f" \
    -c "INSERT INTO civil_os_dev.applied_migrations (file) VALUES ('$name')"
done
echo "migrate: done"
