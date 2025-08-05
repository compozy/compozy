#!/usr/bin/env bash

# Script to publish npm packages from released paths
# Used by the release-please GitHub Action workflow

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_color() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# Function to publish a single package
publish_package() {
    local path=$1
    
    print_color "$YELLOW" "Publishing package in $path"
    
    if [ ! -d "$path" ]; then
        print_color "$RED" "Warning: $path does not exist"
        return 1
    fi
    
    if [ ! -f "$path/package.json" ]; then
        print_color "$RED" "Warning: $path/package.json does not exist"
        return 1
    fi
    
    # Save current directory
    local original_dir=$(pwd)
    
    # Change to package directory
    cd "$path"
    
    # Get package info
    local package_name=$(jq -r '.name' package.json)
    local package_version=$(jq -r '.version' package.json)
    
    print_color "$GREEN" "Installing dependencies for $package_name@$package_version"
    
    # Install dependencies if needed
    if [ -f "package-lock.json" ]; then
        npm ci
    elif [ -f "bun.lockb" ]; then
        bun install --frozen-lockfile
    else
        bun install
    fi
    
    # Publish to npm
    print_color "$GREEN" "Publishing $package_name@$package_version to npm"
    npm publish --access public
    
    # Return to original directory
    cd "$original_dir"
    
    return 0
}

# Function to generate summary
generate_summary() {
    local tools_paths=$1
    local summary_file=${GITHUB_STEP_SUMMARY:-/dev/stdout}
    
    echo "## NPM Package Publishing Summary" >> "$summary_file"
    echo "" >> "$summary_file"
    
    for path in $tools_paths; do
        if [ -f "$path/package.json" ]; then
            local package_name=$(jq -r '.name' "$path/package.json")
            local package_version=$(jq -r '.version' "$path/package.json")
            echo "- ✅ Published \`$package_name@$package_version\`" >> "$summary_file"
        fi
    done
}

# Main function
main() {
    local paths_released=${1:-""}
    
    if [ -z "$paths_released" ]; then
        print_color "$RED" "Error: No paths provided"
        echo "Usage: $0 '<json-array-of-paths>'"
        exit 1
    fi
    
    # Check if NODE_AUTH_TOKEN is set
    if [ -z "${NODE_AUTH_TOKEN:-}" ]; then
        print_color "$RED" "Error: NODE_AUTH_TOKEN environment variable is not set"
        exit 1
    fi
    
    print_color "$GREEN" "Parsing released paths..."
    
    # Parse the JSON array of paths
    local paths=$(echo "$paths_released" | jq -r '.[]' 2>/dev/null || echo "")
    
    if [ -z "$paths" ]; then
        print_color "$YELLOW" "No paths found in release"
        exit 0
    fi
    
    # Filter for tools packages
    local tools_paths=""
    for path in $paths; do
        if [[ "$path" == tools/* ]]; then
            tools_paths="$tools_paths $path"
        fi
    done
    
    # Trim leading/trailing spaces
    tools_paths=$(echo "$tools_paths" | xargs)
    
    if [ -z "$tools_paths" ]; then
        print_color "$YELLOW" "No tools packages found in release"
        exit 0
    fi
    
    print_color "$GREEN" "Tools paths to publish: $tools_paths"
    
    # Publish each package
    local failed=0
    for path in $tools_paths; do
        if ! publish_package "$path"; then
            failed=$((failed + 1))
        fi
    done
    
    # Generate summary
    generate_summary "$tools_paths"
    
    if [ $failed -gt 0 ]; then
        print_color "$RED" "Failed to publish $failed package(s)"
        exit 1
    fi
    
    print_color "$GREEN" "Successfully published all packages!"
}

# Run main function with all arguments
main "$@"