#!/bin/bash
set -euo pipefail

# Script: update-package-versions.sh
# Purpose: Update version in all package.json files (root and tools)
# Usage: ./scripts/release/update-package-versions.sh <version>

VERSION="${1:-}"

# Validate arguments
if [[ -z "$VERSION" ]]; then
    echo "ERROR: Version argument is required" >&2
    echo "Usage: $0 <version>" >&2
    exit 1
fi

# Remove 'v' prefix if present
VERSION="${VERSION#v}"

# Function to update package.json version
update_package_json() {
    local file="$1"
    local version="$2"
    
    if [[ ! -f "$file" ]]; then
        echo "WARNING: File not found: $file" >&2
        return 1
    fi
    
    # Update version using bun's JSON parsing for safety
    bun -e "
        const fs = require('fs');
        const path = '$file';
        const pkg = JSON.parse(fs.readFileSync(path, 'utf8'));
        pkg.version = '$version';
        fs.writeFileSync(path, JSON.stringify(pkg, null, 2) + '\\n');
        console.log('Updated ' + path + ' to version ' + pkg.version);
    " || {
        # Fallback to sed if bun is not available
        if command -v jq &> /dev/null; then
            jq --arg v "$version" '.version = $v' "$file" > "$file.tmp" && mv "$file.tmp" "$file"
            echo "Updated $file to version $version"
        else
            # Last resort: use sed (less safe but works)
            sed -i.bak "s/\"version\": \"[^\"]*\"/\"version\": \"$version\"/" "$file"
            rm -f "$file.bak"
            echo "Updated $file to version $version"
        fi
    }
}

echo "=== Updating package versions to $VERSION ==="

# Update root package.json
echo "Updating root package.json..."
update_package_json "package.json" "$VERSION"

# Update all tools/*/package.json
echo "Updating tools packages..."
for tool_dir in tools/*/; do
    if [[ -d "$tool_dir" && -f "$tool_dir/package.json" ]]; then
        update_package_json "$tool_dir/package.json" "$VERSION"
    fi
done

echo ""
echo "=== Version update complete ==="
echo "version=$VERSION" # GitHub Actions output
