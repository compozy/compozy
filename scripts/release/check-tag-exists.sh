#!/bin/bash
set -euo pipefail

VERSION="${1}"

if git rev-parse "$VERSION" >/dev/null 2>&1; then
    echo "❌ Tag $VERSION already exists!"
    echo "tag_exists=true"
    exit 0
else
    echo "✅ Tag does not exist, proceeding with release"
    echo "tag_exists=false"
fi
