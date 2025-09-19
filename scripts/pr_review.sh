#!/bin/bash

# Check if PR number is provided
if [ $# -eq 0 ]; then
    echo "Usage: $0 <pr_number>"
    exit 1
fi

PR_NUMBER=$1
JSON_FILE="pr_reviews_${PR_NUMBER}.json"
OUTPUT_DIR="./ai-docs/reviews-pr-${PR_NUMBER}"
SUMMARY_FILE="${OUTPUT_DIR}/_summary.md"

echo "Fetching comments for PR #$PR_NUMBER"

# Fetch comments using GitHub CLI
# Try both review comments (pulls) and issue comments (issues) since CodeRabbit AI might use either
echo "Fetching review comments..."
gh api repos/compozy/compozy/pulls/${PR_NUMBER}/comments --jq '.[] | select(.user.login == "coderabbitai[bot]") | {body, path, line, user: .user.login, created_at, resolved_at}' > "${JSON_FILE}.review"

echo "Fetching issue comments..."
gh api repos/compozy/compozy/issues/${PR_NUMBER}/comments --jq '.[] | select(.user.login == "coderabbitai[bot]") | {body, user: .user.login, created_at}' > "${JSON_FILE}.issue"

# Combine both types of comments
jq -s 'flatten' "${JSON_FILE}.review" "${JSON_FILE}.issue" > "$JSON_FILE"

# Clean up temporary files
rm -f "${JSON_FILE}.review" "${JSON_FILE}.issue"

# Check if JSON file was created and has content
if [ ! -s "$JSON_FILE" ]; then
    echo "No CodeRabbit AI comments found for PR #$PR_NUMBER"
    rm -f "$JSON_FILE"
    exit 0
fi

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Calculate statistics
TOTAL_COUNT=$(jq length "$JSON_FILE")
REVIEW_COMMENTS=$(jq '[.[] | select(has("path"))] | length' "$JSON_FILE")
ISSUE_COMMENTS=$((TOTAL_COUNT - REVIEW_COMMENTS))
RESOLVED_COUNT=$(jq '[.[] | select(.resolved_at != null)] | length' "$JSON_FILE")
UNRESOLVED_COUNT=$((TOTAL_COUNT - RESOLVED_COUNT))

# Create individual comment files
echo "Creating individual comment files..."
jq -r '.[] | @base64' "$JSON_FILE" | nl -w3 -s' ' | while read -r index comment_b64; do
  comment=$(echo "$comment_b64" | base64 -d)

  # Extract comment data
  comment_type=$(echo "$comment" | jq -r 'if has("path") and has("line") then "review" else "issue" end')
  created_at=$(echo "$comment" | jq -r '.created_at')
  body=$(echo "$comment" | jq -r '.body')

  # Format date
  formatted_date=$(date -j -f "%Y-%m-%dT%H:%M:%SZ" "$created_at" "+%Y-%m-%d %H:%M" 2>/dev/null || echo "$created_at")

  # Create individual comment file
  comment_file="${OUTPUT_DIR}/comment_${index}.md"

  if [ "$comment_type" = "review" ]; then
    path=$(echo "$comment" | jq -r '.path')
    line=$(echo "$comment" | jq -r '.line')
    resolved_at=$(echo "$comment" | jq -r '.resolved_at')

    if [ "$resolved_at" = "null" ]; then
      status="- [ ] UNRESOLVED"
    else
      status="- [x] RESOLVED ✓"
    fi

    cat > "$comment_file" << EOF
# Comment ${index} - Review Comment

**File:** \`${path}:${line}\`
**Date:** ${formatted_date}
**Status:** ${status}

## Comment Body

${body}

---
*Generated from PR #${PR_NUMBER} - CodeRabbit AI*
EOF
  else
    cat > "$comment_file" << EOF
# Comment ${index} - General Comment

**Date:** ${formatted_date}
**Status:** - [ ] UNRESOLVED (General comments cannot be resolved)

## Comment Body

${body}

---
*Generated from PR #${PR_NUMBER} - CodeRabbit AI*
EOF
  fi

  echo "Created $comment_file"
done

# Create summary file
cat > "$SUMMARY_FILE" << EOF
# PR Review #$PR_NUMBER - CodeRabbit AI Comments

This folder contains individual comment files from CodeRabbit AI for PR #$PR_NUMBER.

## Summary

- **Total comments:** $TOTAL_COUNT
- **Review comments:** $REVIEW_COMMENTS (inline code comments)
- **Issue comments:** $ISSUE_COMMENTS (general PR comments)
- **Resolved:** $RESOLVED_COUNT ✓
- **Unresolved:** $UNRESOLVED_COUNT

**Generated on:** $(date)

## Comments

EOF

# Add links to individual comment files
for i in $(seq 1 $TOTAL_COUNT); do
  comment_file="comment_$(printf "%03d" $i).md"
  echo "- [ ] [Comment ${i}](${comment_file})" >> "$SUMMARY_FILE"
done

echo ""
echo "Created $SUMMARY_FILE with links to $TOTAL_COUNT comment files"
echo "JSON data saved to $JSON_FILE"

# Clean up JSON file
rm "$JSON_FILE"
