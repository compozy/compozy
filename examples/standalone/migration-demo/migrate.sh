#!/usr/bin/env bash
set -euo pipefail

# Simple helper that derives a distributed phase config from phase1 by copying
# project files and emitting reminders for required runtime env vars.

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
PHASE1="$ROOT_DIR/phase1-standalone"
PHASE2="$ROOT_DIR/phase2-distributed"

echo "Generating phase2 configuration from phase1…"
cp -f "$PHASE1/compozy.yaml" "$PHASE2/compozy.yaml"

cat <<EOF
Next steps:
- Ensure environment variables are set for distributed runtime:
  COMPOZY_MODE=distributed
  COMPOZY_REDIS_MODE=distributed
  COMPOZY_REDIS_ADDR=localhost:6379
  COMPOZY_TEMPORAL_MODE=remote
  COMPOZY_TEMPORAL_HOST_PORT=localhost:7233
  COMPOZY_TEMPORAL_NAMESPACE=default
EOF

echo "Done."

