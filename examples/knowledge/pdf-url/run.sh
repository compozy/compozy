#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f .env ]]; then
  echo "⚠️  Missing .env file. Copy .env.example and set OPENAI_API_KEY." >&2
  exit 1
fi

set -a
source .env
set +a

echo "🚀 Applying knowledge resources"
compozy knowledge apply --file compozy.yaml

echo "🧠 Ingesting remote PDF"
compozy knowledge ingest --id pdf_demo --project . --verbose

echo "🔍 Querying knowledge"
compozy knowledge query --id pdf_demo --text "What placeholder text does the PDF contain?" --top_k 3 --output table

echo "🤖 Running workflow"
compozy run workflows/qa.yaml --input '{"question":"Summarize the PDF"}'

echo "✅ PDF example complete"
