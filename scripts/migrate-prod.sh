#!/bin/sh
# Applies packages/db/migrations in filename order, once each. Used by the
# one-shot `migrate` service in docker-compose.prod.yml. The two skipped files
# are Supabase-only.
set -eu

export PGPASSWORD="$POSTGRES_PASSWORD"
PSQL="psql -v ON_ERROR_STOP=1 -q -h postgres -U agently -d agently"

until pg_isready -q -h postgres -U agently -d agently; do sleep 1; done

$PSQL -c "CREATE TABLE IF NOT EXISTS schema_migrations (name text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now());"

for file in /migrations/*.sql; do
  name=$(basename "$file")
  case "$name" in
    0002_rls.sql|0004_realtime.sql) continue ;;
  esac
  applied=$($PSQL -tA -c "SELECT 1 FROM schema_migrations WHERE name = '$name';")
  if [ -z "$applied" ]; then
    echo "applying $name"
    # Each migration file manages its own transaction.
    $PSQL -f "$file"
    $PSQL -c "INSERT INTO schema_migrations (name) VALUES ('$name');"
  fi
done
echo "migrations up to date"
