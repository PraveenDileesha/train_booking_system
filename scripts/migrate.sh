#!/bin/bash
#
# migrate.sh — applies database migrations using golang-migrate inside a throwaway Docker container.
# No local install of migrate is required.
#
# Usage:
#   ./scripts/migrate.sh up               # apply all pending migrations
#   ./scripts/migrate.sh up 1             # apply the next N migrations
#   ./scripts/migrate.sh down 1           # roll back the last N migrations
#   ./scripts/migrate.sh version          # print the current schema version
#   ./scripts/migrate.sh create <name>    # scaffold a new up/down migration pair
#
# Requires Docker and a running `postgres` service (`docker compose up -d postgres`).
# Reads DB credentials from .env in the project root.

set -euo pipefail
cd "$(dirname "$0")/.."

POSTGRES_CONTAINER="TBS_postgres"
MIGRATIONS_DIR="$(pwd)/db/migrations"

if [ ! -f .env ]; then
  echo "error: .env not found in project root — copy .env.sample to .env and fill it in first." >&2
  exit 1
fi

# Word splitting is intentional here: each "KEY=VALUE" line in .env becomes a separate argument to `export`.
# shellcheck disable=SC2046
export $(grep -v '^#' .env | grep -v '^$' | xargs)

if [ "${1:-}" = "create" ]; then
  if [ -z "${2:-}" ]; then
    echo "usage: $0 create <migration_name>" >&2
    exit 1
  fi
  docker run --rm \
    -v "$MIGRATIONS_DIR":/migrations \
    migrate/migrate \
    create -ext sql -dir /migrations -seq "$2"
  exit 0
fi

if ! docker inspect "$POSTGRES_CONTAINER" >/dev/null 2>&1; then
  echo "error: container '$POSTGRES_CONTAINER' isn't running — start it first with:" >&2
  echo "  docker compose up -d postgres" >&2
  exit 1
fi

# Resolve the Docker network that the `postgres` container is attached to.
NETWORK="$(docker inspect "$POSTGRES_CONTAINER" \
  --format '{{range $name, $_ := .NetworkSettings.Networks}}{{$name}}{{end}}')"

if [ -z "$NETWORK" ]; then
  echo "error: could not determine the Docker network for '$POSTGRES_CONTAINER'." >&2
  exit 1
fi

docker run --rm \
  --network "$NETWORK" \
  -v "$MIGRATIONS_DIR":/migrations \
  migrate/migrate \
  -path=/migrations \
  -database "postgres://$POSTGRES_USER:$POSTGRES_PASSWORD@$POSTGRES_CONTAINER:5432/$POSTGRES_DB?sslmode=disable" \
  "$@"
