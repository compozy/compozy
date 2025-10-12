#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f .env ]]; then
  echo "⚠️  Missing .env file. Copy .env.example and set OPENAI_API_KEY." >&2
  exit 1
fi

set -a
source .env
set +a

echo "🚀 Applying knowledge base"
compozy knowledge apply --file compozy.yaml

echo "🧠 Ingesting runbook"
compozy knowledge ingest --id runbook_kb --project .

echo "🔍 Running sample queries"
compozy knowledge query --id runbook_kb --text "How long can downtime last?" --top_k 3 --min_score 0.2 --output table
compozy knowledge query --id runbook_kb --text "Who should we escalate to?" --filters category=runbook --top_k 2 --output table

echo "✅ Query CLI example complete"
