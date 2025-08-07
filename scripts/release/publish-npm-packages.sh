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
            echo "ERROR: Unknown option: $1" >&2
            echo "Usage: $0 [--dry-run]" >&2
            exit 1
            ;;
    esac
done

# Function to safely read JSON field from package.json
read_package_field() {
    local package_file="$1"
    local field="$2"
    local default="${3:-}"
    
    if [[ ! -f "$package_file" ]]; then
        echo "$default"
        return 1
    fi
    
    # Use jq for safe JSON parsing (required dependency)
    if command -v jq &> /dev/null; then
        jq -r ".$field // \"$default\"" "$package_file" 2>/dev/null || echo "$default"
    else
        echo "ERROR: jq is required for JSON parsing" >&2
        exit 1
    fi
}

# Function to publish a package
publish_package() {
    local package_dir="$1"
    local package_json="$package_dir/package.json"
    local package_name
    local package_version
    local is_private
    
    if [[ ! -f "$package_json" ]]; then
        echo "WARNING: No package.json found in $package_dir" >&2
        return 1
    fi
    
    # Safely extract package information using jq
    package_name=$(read_package_field "$package_json" "name" "")
    package_version=$(read_package_field "$package_json" "version" "")
    is_private=$(read_package_field "$package_json" "private" "false")
    
    if [[ -z "$package_name" ]] || [[ -z "$package_version" ]]; then
        echo "ERROR: Invalid package.json in $package_dir (missing name or version)" >&2
        return 1
    fi
    
    echo "Publishing $package_name@$package_version from $package_dir..."
    
    # Check if package is private
    if [[ "$is_private" == "true" ]]; then
        echo "  Skipping private package: $package_name"
        return 0
    fi
    
    # Check if version already exists on NPM (idempotency)
    if npm view "$package_name@$package_version" > /dev/null 2>&1; then
        echo "  Version $package_version already published for $package_name, skipping..."
        return 0
    fi
    
    # Change to package directory for publishing
    (
        cd "$package_dir" || exit 1
        
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
    local npm_user
    npm_user=$(npm whoami 2>/dev/null || echo "unknown")
    echo "Authenticated as: $npm_user"
}

# Validate required tools
validate_dependencies() {
    local missing_deps=()
    
    if ! command -v jq &> /dev/null; then
        missing_deps+=("jq")
    fi
    
    if ! command -v npm &> /dev/null; then
        missing_deps+=("npm")
    fi
    
    if [[ ${#missing_deps[@]} -gt 0 ]]; then
        echo "ERROR: Required dependencies are missing:" >&2
        printf '  - %s\n' "${missing_deps[@]}" >&2
        echo "Please install them before running this script" >&2
        exit 1
    fi
}

echo "=== NPM Package Publishing ==="

# Validate dependencies
validate_dependencies

# Check if we're authenticated (skip for dry-run)
if [[ "$DRY_RUN" == "false" ]]; then
    echo "Checking NPM authentication..."
    check_npm_auth || exit 1
fi

# Publish tools packages
echo ""
echo "Publishing tools packages..."
PUBLISHED_COUNT=0
FAILED_COUNT=0
FAILED_PACKAGES=()

# Process tools directory
if [[ ! -d "tools" ]]; then
    echo "WARNING: tools directory not found" >&2
    exit 0
fi

for tool_dir in tools/*/; do
    if [[ -d "$tool_dir" && -f "$tool_dir/package.json" ]]; then
        tool_name=$(basename "$tool_dir")
        echo ""
        echo "Processing $tool_name..."
        if publish_package "$tool_dir"; then
            ((PUBLISHED_COUNT++))
        else
            ((FAILED_COUNT++))
            FAILED_PACKAGES+=("$tool_name")
            echo "  ❌ Failed to publish $tool_name"
        fi
    fi
done

echo ""
echo "=== Publishing Summary ==="
echo "Published: $PUBLISHED_COUNT packages"
if [[ $FAILED_COUNT -gt 0 ]]; then
    echo "Failed: $FAILED_COUNT packages"
    if [[ ${#FAILED_PACKAGES[@]} -gt 0 ]]; then
        echo "Failed packages:"
        printf '  - %s\n' "${FAILED_PACKAGES[@]}"
    fi
    exit 1
fi

if [[ "$DRY_RUN" == "true" ]]; then
    echo ""
    echo "This was a dry-run. No packages were actually published."
    echo "Run without --dry-run to publish packages."
fi

echo ""
echo "✅ NPM publishing complete!"

# GitHub Actions outputs (write directly if available)
if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    {
        echo "published_count=$PUBLISHED_COUNT"
        echo "failed_count=$FAILED_COUNT"
    } >> "$GITHUB_OUTPUT"
else
    echo "published_count=$PUBLISHED_COUNT"
    echo "failed_count=$FAILED_COUNT"
fi