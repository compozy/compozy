#!/usr/bin/env bash
# Bootstrap helper — creates an isolated work dir and installs capture/diff dependencies via bun.
# Required: bun, Google Chrome installed at the default macOS / Linux location.
# Usage:
#   bash setup-workdir.sh [<workdir>]
# If <workdir> is omitted, defaults to /tmp/agh-ui-screenshot.
# Output:
#   stdout: the absolute path of the prepared workdir.
#   stderr: bun install errors.
#   exit 0 on success.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKDIR="${1:-/tmp/agh-ui-screenshot}"
mkdir -p "$WORKDIR"
cd "$WORKDIR"

if [ ! -f package.json ]; then
  cat > package.json <<'EOF'
{
  "name": "agh-ui-screenshot-workdir",
  "private": true,
  "type": "module"
}
EOF
fi

if [ ! -d node_modules/chrome-launcher ] || \
   [ ! -d node_modules/chrome-remote-interface ] || \
   [ ! -d node_modules/pixelmatch ] || \
   [ ! -d node_modules/pngjs ]; then
  bun add --exact chrome-launcher@1.2.1 chrome-remote-interface@0.34.0 pixelmatch@7.2.0 pngjs@7.0.0 >&2
fi

cp "$SCRIPT_DIR/cap.mjs" "$WORKDIR/cap.mjs"
cp "$SCRIPT_DIR/list-stories.mjs" "$WORKDIR/list-stories.mjs"
cp "$SCRIPT_DIR/visual-diff.mjs" "$WORKDIR/visual-diff.mjs"
cp "$SCRIPT_DIR/validate-visual-contract.mjs" "$WORKDIR/validate-visual-contract.mjs"

echo "$WORKDIR"
