#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"
EXAMPLE_DIR="${SCRIPT_DIR}/.."

if [[ ! -f "${EXAMPLE_DIR}/.env" ]]; then
  echo "⚠️  Missing .env file. Copy .env.example in the example directory and set secrets." >&2
  exit 1
fi

pushd "${EXAMPLE_DIR}" > /dev/null
set -a
source .env
set +a

if [[ -n "${PGVECTOR_PASSWORD:-}" ]]; then
  export PGPASSWORD="${PGVECTOR_PASSWORD}"
fi

echo "🚢 Starting Docker dependencies"
make start-docker

echo "🚀 Applying resources"
compozy knowledge apply --file compozy.yaml

echo "🧠 Ingesting documents"
compozy knowledge ingest --id company_docs --project . --batch-size 48 --verbose

echo "🔍 Querying knowledge base"
compozy knowledge query --id company_docs --text "What is our uptime target?" --top_k 3 --min_score 0.2 --output table

echo "🤖 Running workflow"
compozy run workflows/qa.yaml --input '{"question":"What is the incident response timeframe?"}'

unset PGPASSWORD
popd > /dev/null

echo "✅ Pgvector example complete"
