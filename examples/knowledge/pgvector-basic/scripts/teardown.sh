#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"
EXAMPLE_DIR="${SCRIPT_DIR}/.."

if [[ ! -f "${EXAMPLE_DIR}/.env" ]]; then
  echo "ℹ️  No .env file found. Nothing to clean."
  exit 0
fi

pushd "${EXAMPLE_DIR}" > /dev/null
set -a
source .env
set +a

if [[ -n "${PGVECTOR_PASSWORD:-}" ]]; then
  export PGPASSWORD="${PGVECTOR_PASSWORD}"
fi

if command -v psql >/dev/null 2>&1; then
  echo "🗑️  Dropping pgvector table"
  PGOPTIONS='--client-min-messages=warning' psql "$PGVECTOR_DSN" -qAtc 'DROP TABLE IF EXISTS knowledge_chunks;'
else
  echo "⚠️  psql not available; skip table cleanup."
fi

rm -f .env
unset PGPASSWORD
popd > /dev/null

echo "🧹 Teardown complete"
