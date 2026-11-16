#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "$SCRIPT_DIR/../.." && pwd)"

COMPOSE_FILE="$ROOT_DIR/deploy/local-docker-compose.yaml"
MYSQL_ENV_FILE="$ROOT_DIR/deploy/mysql/local/.env"

if [[ ! -f "$COMPOSE_FILE" ]]; then
  echo "Missing docker compose file: $COMPOSE_FILE" >&2
  exit 1
fi

if [[ ! -f "$MYSQL_ENV_FILE" ]]; then
  echo "Missing MySQL local environment file: $MYSQL_ENV_FILE" >&2
  exit 1
fi

if [[ $# -eq 0 ]]; then
  set -- up -d
fi

exec docker compose \
  --env-file "$MYSQL_ENV_FILE" \
  -f "$COMPOSE_FILE" \
  "$@"
