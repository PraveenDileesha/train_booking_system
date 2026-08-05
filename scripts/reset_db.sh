#!/bin/bash
# Wipes every table's data (all rows, all tables in the public schema) without touching the schema itself, so seed_demo.sh always starts from a clean slate and IDs restart at 1.
# Table list is discovered at runtime from pg_tables rather than hardcoded, so it stays correct as the schema grows.
#
# Run as ./scripts/reset_db.sh
set -euo pipefail

cd "$(dirname "$0")/.."
export $(grep -v '^#' .env | xargs)

read -p "This will permanently delete ALL data in ${POSTGRES_DB}. Continue? [y/N] " confirm
if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
  echo "Aborted."
  exit 1
fi

docker exec -i TBS_postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<'SQL'
DO $$
DECLARE
  table_list text;
BEGIN
  SELECT string_agg(format('%I.%I', schemaname, tablename), ', ')
  INTO table_list
  FROM pg_tables
  WHERE schemaname = 'public';

  IF table_list IS NOT NULL THEN
    EXECUTE 'TRUNCATE TABLE ' || table_list || ' RESTART IDENTITY CASCADE';
  END IF;
END $$;
SQL

echo "Database wiped. Run ./scripts/seed_demo.sh to repopulate."
