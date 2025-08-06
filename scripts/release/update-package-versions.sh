#!/bin/bash
set -euo pipefail

# Script: update-package-versions.sh
# Purpose: Update version in all package.json files (root and tools)
# Usage: ./scripts/release/update-package-versions.sh <version>

# Validate arguments
if [[ $# -lt 1 ]]; then
    echo "ERROR: Version argument is required" >&2
    echo "Usage: $0 <version>" >&2
    exit 1
fi

VERSION="${1}"

# Function to validate version format
validate_version() {
    local version="${1}"
    # Remove 'v' prefix for validation
    local clean_version="${version#v}"
    if [[ ! "$clean_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?$ ]]; then
        return 1
    fi
    return 0
}

# Validate version format
if ! validate_version "$VERSION"; then
    echo "ERROR: Invalid version format: $VERSION" >&2
    echo "Expected format: X.Y.Z or vX.Y.Z (with optional prerelease/build metadata)" >&2
    exit 1
fi

# Remove 'v' prefix if present (package.json uses plain semver)
VERSION="${VERSION#v}"

# Function to update package.json version using jq
update_package_json() {
    local file="$1"
    local version="$2"
    local temp_file
    
    if [[ ! -f "$file" ]]; then
        echo "WARNING: File not found: $file" >&2
        return 1
    fi
    
    # Create temp file for atomic update
    temp_file=$(mktemp "${file}.XXXXXX")
    trap "rm -f '$temp_file'" EXIT
    
    # Use jq for safe JSON manipulation (required dependency)
    if ! command -v jq &> /dev/null; then
        echo "ERROR: jq is required for JSON manipulation" >&2
        echo "Please install jq: https://stedolan.github.io/jq/download/" >&2
        exit 1
    fi
    
    # Update version in package.json
    if jq --arg v "$version" '.version = $v' "$file" > "$temp_file" 2>/dev/null; then
        # Validate the JSON is still valid
        if jq empty "$temp_file" 2>/dev/null; then
            mv "$temp_file" "$file"
            echo "Updated $file to version $version"
            return 0
        else
            echo "ERROR: Failed to update $file - invalid JSON after update" >&2
            rm -f "$temp_file"
            return 1
        fi
    else
        echo "ERROR: Failed to update $file" >&2
        rm -f "$temp_file"
        return 1
    fi
}

# Validate required tools
validate_dependencies() {
    local missing_deps=()
    
    if ! command -v jq &> /dev/null; then
        missing_deps+=("jq")
    fi
    
    if [[ ${#missing_deps[@]} -gt 0 ]]; then
        echo "ERROR: Required dependencies are missing:" >&2
        printf '  - %s\n' "${missing_deps[@]}" >&2
        echo "Please install them before running this script" >&2
        exit 1
    fi
}

echo "=== Updating package versions to $VERSION ==="

# Validate dependencies
validate_dependencies

UPDATE_COUNT=0
FAILED_COUNT=0
FAILED_FILES=()

# Update root package.json
echo "Updating root package.json..."
if update_package_json "package.json" "$VERSION"; then
    ((UPDATE_COUNT++))
else
    ((FAILED_COUNT++))
    FAILED_FILES+=("package.json")
fi

# Update all tools/*/package.json
echo "Updating tools packages..."
if [[ -d "tools" ]]; then
    for tool_dir in tools/*/; do
        if [[ -d "$tool_dir" ]]; then
            package_file="$tool_dir/package.json"
            if [[ -f "$package_file" ]]; then
                if update_package_json "$package_file" "$VERSION"; then
                    ((UPDATE_COUNT++))
                else
                    ((FAILED_COUNT++))
                    FAILED_FILES+=("$package_file")
                fi
            fi
        fi
    done
else
    echo "WARNING: tools directory not found" >&2
fi

echo ""
echo "=== Version update summary ==="
echo "Updated: $UPDATE_COUNT files"
if [[ $FAILED_COUNT -gt 0 ]]; then
    echo "Failed: $FAILED_COUNT files"
    if [[ ${#FAILED_FILES[@]} -gt 0 ]]; then
        echo "Failed files:"
        printf '  - %s\n' "${FAILED_FILES[@]}"
    fi
    exit 1
fi

echo ""
echo "✅ Version update complete!"

# GitHub Actions output (write directly if available)
if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    echo "version=$VERSION" >> "$GITHUB_OUTPUT"
else
    echo "version=$VERSION"
fi