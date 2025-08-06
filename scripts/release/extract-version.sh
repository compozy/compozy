#!/bin/bash
set -euo pipefail

COMMIT_MESSAGE="${1}"

# Extract version from the commit message
VERSION=$(echo "$COMMIT_MESSAGE" | grep -oP 'v\d+\.\d+\.\d+' | head -1)

if [ -z "$VERSION" ]; then
    echo "❌ Could not extract version from commit message"
    exit 1
fi

echo "version=$VERSION"
echo "📦 Preparing to release version: $VERSION"
