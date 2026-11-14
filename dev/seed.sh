#!/bin/sh
# Loads database/fixtures/synthetic.sql into an empty "civil" database.
# Skips when parcels already exist, so `docker compose up` can be re-run.
set -eu

if [ "$(psql -d civil -tAc 'SELECT count(*) FROM core.parcels')" != 0 ]; then
  echo "seed: data already present, skipping"
  exit 0
fi
psql -v ON_ERROR_STOP=1 -d civil -q -f /fixtures/synthetic.sql
echo "seed: loaded synthetic fixtures"
