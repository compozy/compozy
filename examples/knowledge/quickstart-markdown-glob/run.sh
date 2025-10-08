#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f .env ]]; then
  echo "⚠️  Missing .env file. Copy .env.example and add required secrets." >&2
  exit 1
fi

set -a
source .env
set +a

echo "🚀 Applying knowledge resources"
compozy knowledge apply --file compozy.yaml

echo "🧠 Ingesting Markdown documents"
compozy knowledge ingest --id quickstart_docs --project . --verbose

echo "🔍 Querying knowledge base"
compozy knowledge query --id quickstart_docs --text "What is our escalation policy?" --top_k 2 --min_score 0.2 --output table

echo "🤖 Running workflow"
compozy run workflows/qa.yaml --input '{"question":"How quickly should we respond to incidents?"}'

echo "✅ Quickstart complete"
