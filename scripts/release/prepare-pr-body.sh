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
if [ -f "$TEMPLATE_FILE" ]; then
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
if [ -f "$CHANGELOG_FILE" ]; then
    CHANGELOG_PREVIEW=$(cat "$CHANGELOG_FILE")
else
    CHANGELOG_PREVIEW="No changes detected - force release requested"
fi

# Replace placeholders
BODY=$(echo "$TEMPLATE" | sed "s|{{ .Version }}|$VERSION|g")
BODY=$(echo "$BODY" | sed "s|{{ .ChangelogPreview }}|$CHANGELOG_PREVIEW|g")

# Save to file for the PR action
echo "$BODY" > "$OUTPUT_FILE"
