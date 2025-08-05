#!/usr/bin/env bash

# Script to synchronize all tool package versions with the main project version
# Used after release-please creates a new version tag

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

# Function to update package.json version
update_package_version() {
    local package_path=$1
    local new_version=$2
    
    if [ ! -f "$package_path/package.json" ]; then
        print_color "$RED" "Warning: $package_path/package.json does not exist"
        return 1
    fi
    
    # Get current package info
    local package_name=$(jq -r '.name' "$package_path/package.json")
    local current_version=$(jq -r '.version' "$package_path/package.json")
    
    if [ "$current_version" == "$new_version" ]; then
        print_color "$YELLOW" "Package $package_name already at version $new_version"
        return 0
    fi
    
    print_color "$GREEN" "Updating $package_name from $current_version to $new_version"
    
    if [ "$DRY_RUN" = true ]; then
        print_color "$BLUE" "  [DRY RUN] Would update $package_path/package.json"
    else
        # Update version in package.json
        jq --arg ver "$new_version" '.version = $ver' "$package_path/package.json" > "$package_path/package.json.tmp"
        mv "$package_path/package.json.tmp" "$package_path/package.json"
    fi
    
    return 0
}

# Main function
main() {
    local new_version=""
    
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            -h|--help)
                echo "Usage: $0 <version> [--dry-run]"
                echo ""
                echo "Arguments:"
                echo "  <version>    The version to sync all tool packages to (e.g., 0.0.5)"
                echo ""
                echo "Options:"
                echo "  --dry-run    Simulate the sync process without modifying files"
                echo "  -h, --help   Show this help message"
                echo ""
                echo "This script synchronizes all tool package versions to match the main project version."
                echo "It finds all packages in the tools/ directory and updates their package.json files."
                exit 0
                ;;
            *)
                new_version=$1
                shift
                ;;
        esac
    done
    
    if [ -z "$new_version" ]; then
        print_color "$RED" "Error: No version provided"
        echo "Usage: $0 <version> [--dry-run]"
        echo "Example: $0 0.0.5"
        echo "         $0 0.0.5 --dry-run"
        exit 1
    fi
    
    # Validate version format (basic semver)
    if ! echo "$new_version" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?$'; then
        print_color "$RED" "Error: Invalid version format. Must be semver compliant (e.g., 1.2.3)"
        exit 1
    fi
    
    if [ "$DRY_RUN" = true ]; then
        print_color "$BLUE" "DRY RUN MODE - No files will be modified"
    fi
    
    print_color "$GREEN" "Synchronizing all tool packages to version $new_version"
    
    # Find all tool directories
    local tool_dirs=$(find tools -maxdepth 1 -type d -name "*" | grep -v "^tools$" | sort)
    
    if [ -z "$tool_dirs" ]; then
        print_color "$RED" "No tool directories found"
        exit 1
    fi
    
    # Count packages
    local total_packages=$(echo "$tool_dirs" | wc -l | xargs)
    local updated_packages=0
    local failed_packages=0
    
    print_color "$GREEN" "Found $total_packages tool packages to update"
    echo ""
    
    # Update each package
    for dir in $tool_dirs; do
        if update_package_version "$dir" "$new_version"; then
            updated_packages=$((updated_packages + 1))
        else
            failed_packages=$((failed_packages + 1))
        fi
    done
    
    echo ""
    print_color "$GREEN" "Summary:"
    print_color "$GREEN" "- Total packages: $total_packages"
    print_color "$GREEN" "- Updated: $updated_packages"
    
    if [ $failed_packages -gt 0 ]; then
        print_color "$RED" "- Failed: $failed_packages"
        exit 1
    fi
    
    # Check if we're in a git repository and if there are changes
    if git rev-parse --git-dir > /dev/null 2>&1; then
        if git diff --quiet tools/*/package.json 2>/dev/null; then
            print_color "$YELLOW" "No changes detected in package.json files"
        else
            print_color "$GREEN" "Changes detected in package.json files:"
            git diff --name-only tools/*/package.json
            
            # Show version changes
            echo ""
            print_color "$GREEN" "Version changes:"
            git diff tools/*/package.json | grep -E '^\+\s*"version":|^-\s*"version":' || true
        fi
    fi
    
    print_color "$GREEN" "Version synchronization complete!"
}

# Run main function with all arguments
main "$@"