#!/bin/bash
set -euo pipefail

# Script: prepare-pr-body.sh
# Purpose: Prepare pull request body from template
# Usage: ./scripts/release/prepare-pr-body.sh <version> [template_file] [changelog_file] [output_file]

VERSION="${1:-}"
TEMPLATE_FILE="${2:-.github/RELEASE_PR_TEMPLATE.md}"
CHANGELOG_FILE="${3:-CHANGELOG_PR.md}"
OUTPUT_FILE="${4:-PR_BODY.md}"

# Validate version argument
if [[ -z "$VERSION" ]]; then
    echo "ERROR: Version argument is required" >&2
    echo "Usage: $0 <version> [template_file] [changelog_file] [output_file]" >&2
    exit 1
fi

echo "Preparing PR body for version: $VERSION"

# Load the template
if [[ -f "$TEMPLATE_FILE" ]]; then
    TEMPLATE=$(cat "$TEMPLATE_FILE")
else
    # Fallback template if file doesn't exist
    TEMPLATE="## 🚀 Release $VERSION

This PR prepares the release of version **$VERSION**.

### Changes

{{ .ChangelogPreview }}

### Checklist

- [ ] Changelog has been updated
- [ ] Version numbers are correct
- [ ] CI/CD checks pass
- [ ] Documentation is up to date"
fi

# Get changelog preview
if [[ -f "$CHANGELOG_FILE" ]]; then
    CHANGELOG_PREVIEW=$(cat "$CHANGELOG_FILE")
else
    CHANGELOG_PREVIEW="No changes detected - force release requested"
fi

# Create a temporary file for the changelog to avoid multi-line variable issues
TEMP_CHANGELOG=$(mktemp)
trap "rm -f '$TEMP_CHANGELOG'" EXIT
echo "$CHANGELOG_PREVIEW" > "$TEMP_CHANGELOG"

# Use awk with file reading instead of variable passing for multi-line content
{
    echo "$TEMPLATE" | awk -v version="$VERSION" -v changelog_file="$TEMP_CHANGELOG" '
    BEGIN {
        # Read the entire changelog file into a variable
        changelog = ""
        while ((getline line < changelog_file) > 0) {
            if (changelog != "") {
                changelog = changelog "\n" line
            } else {
                changelog = line
            }
        }
        close(changelog_file)
        
        # Escape special characters for safe replacement
        # Escape backslashes first
        gsub(/\\/, "\\\\", changelog)
        # Escape ampersands (which have special meaning in sub/gsub replacement)
        gsub(/&/, "\\\\&", changelog)
    }
    {
        # Replace {{ .Version }} with actual version
        gsub(/\{\{ \.Version \}\}/, version)
        
        # For {{ .ChangelogPreview }}, we need special handling
        if (match($0, /\{\{ \.ChangelogPreview \}\}/)) {
            # Replace the placeholder with the changelog content
            sub(/\{\{ \.ChangelogPreview \}\}/, changelog)
        }
        print
    }'
} > "$OUTPUT_FILE"
