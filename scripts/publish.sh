#!/usr/bin/env bash

# Script to publish npm packages from released paths
# Used by the release-please GitHub Action workflow

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Dry run mode
DRY_RUN=false

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
    
    if [ "$DRY_RUN" = true ]; then
        print_color "$BLUE" "  [DRY RUN] Would install dependencies for $package_name@$package_version"
        print_color "$BLUE" "  [DRY RUN] Would publish $package_name@$package_version to npm"
    else
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
    fi
    
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
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            -h|--help)
                echo "Usage: $0 [--dry-run]"
                echo ""
                echo "Options:"
                echo "  --dry-run    Simulate the publish process without actually publishing"
                echo "  -h, --help   Show this help message"
                echo ""
                echo "This script publishes all tool packages found in the tools/ directory to npm."
                exit 0
                ;;
            *)
                shift
                ;;
        esac
    done
    
    # Check if NODE_AUTH_TOKEN is set (not required for dry run)
    if [ "$DRY_RUN" = false ] && [ -z "${NODE_AUTH_TOKEN:-}" ]; then
        print_color "$RED" "Error: NODE_AUTH_TOKEN environment variable is not set"
        exit 1
    fi
    
    if [ "$DRY_RUN" = true ]; then
        print_color "$BLUE" "DRY RUN MODE - No packages will be published"
    fi
    
    print_color "$GREEN" "Finding all tool packages to publish..."
    
    # Find all tool directories
    local tools_paths=$(find tools -maxdepth 1 -type d -name "*" | grep -v "^tools$" | sort)
    
    if [ -z "$tools_paths" ]; then
        print_color "$YELLOW" "No tool packages found"
        exit 0
    fi
    
    # Convert to space-separated list
    tools_paths=$(echo "$tools_paths" | tr '\n' ' ' | xargs)
    
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
