#!/bin/bash
set -euo pipefail

# Script: publish-npm-packages.sh
# Purpose: Publish all tools packages to NPM registry
# Usage: ./scripts/release/publish-npm-packages.sh [--dry-run]

DRY_RUN=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        *)
            echo "Unknown option: $1" >&2
            echo "Usage: $0 [--dry-run]" >&2
            exit 1
            ;;
    esac
done

# Function to publish a package
publish_package() {
    local package_dir="$1"
    local package_name
    local package_version
    
    if [[ ! -f "$package_dir/package.json" ]]; then
        echo "WARNING: No package.json found in $package_dir" >&2
        return 1
    fi
    
    # Extract package name and version
    package_name=$(bun -e "console.log(JSON.parse(require('fs').readFileSync('$package_dir/package.json', 'utf8')).name)" 2>/dev/null || \
                   jq -r '.name' "$package_dir/package.json" 2>/dev/null || \
                   grep '"name"' "$package_dir/package.json" | sed 's/.*"name".*"\([^"]*\)".*/\1/')
    
    package_version=$(bun -e "console.log(JSON.parse(require('fs').readFileSync('$package_dir/package.json', 'utf8')).version)" 2>/dev/null || \
                      jq -r '.version' "$package_dir/package.json" 2>/dev/null || \
                      grep '"version"' "$package_dir/package.json" | sed 's/.*"version".*"\([^"]*\)".*/\1/')
    
    echo "Publishing $package_name@$package_version from $package_dir..."
    
    # Check if package is private
    is_private=$(bun -e "console.log(JSON.parse(require('fs').readFileSync('$package_dir/package.json', 'utf8')).private || false)" 2>/dev/null || echo "false")
    
    if [[ "$is_private" == "true" ]]; then
        echo "  Skipping private package: $package_name"
        return 0
    fi
    
    # Check if version already exists on NPM
    if npm view "$package_name@$package_version" > /dev/null 2>&1; then
        echo "  Version $package_version already published for $package_name, skipping..."
        return 0
    fi
    
    # Change to package directory for publishing
    (
        cd "$package_dir"
        
        if [[ "$DRY_RUN" == "true" ]]; then
            echo "  [DRY-RUN] Would publish: npm publish --access public"
            npm publish --dry-run --access public
        else
            # Publish the package
            npm publish --access public
            echo "  ✅ Successfully published $package_name@$package_version"
        fi
    )
}

# Function to check NPM authentication
check_npm_auth() {
    if ! npm whoami > /dev/null 2>&1; then
        echo "ERROR: Not authenticated with NPM registry" >&2
        echo "Please run 'npm login' or set NPM_TOKEN environment variable" >&2
        return 1
    fi
    echo "Authenticated as: $(npm whoami)"
}

echo "=== NPM Package Publishing ==="

# Check if we're authenticated
if [[ "$DRY_RUN" == "false" ]]; then
    echo "Checking NPM authentication..."
    check_npm_auth || exit 1
fi

# Publish tools packages
echo ""
echo "Publishing tools packages..."
PUBLISHED_COUNT=0
FAILED_COUNT=0

for tool_dir in tools/*/; do
    if [[ -d "$tool_dir" && -f "$tool_dir/package.json" ]]; then
        echo ""
        echo "Processing $(basename "$tool_dir")..."
        if publish_package "$tool_dir"; then
            ((PUBLISHED_COUNT++))
        else
            ((FAILED_COUNT++))
            echo "  ❌ Failed to publish $(basename "$tool_dir")"
        fi
    fi
done

echo ""
echo "=== Publishing Summary ==="
echo "Published: $PUBLISHED_COUNT packages"
if [[ $FAILED_COUNT -gt 0 ]]; then
    echo "Failed: $FAILED_COUNT packages"
    exit 1
fi

if [[ "$DRY_RUN" == "true" ]]; then
    echo ""
    echo "This was a dry-run. No packages were actually published."
    echo "Run without --dry-run to publish packages."
fi

echo ""
echo "✅ NPM publishing complete!"

# GitHub Actions outputs
echo "published_count=$PUBLISHED_COUNT"
echo "failed_count=$FAILED_COUNT"
