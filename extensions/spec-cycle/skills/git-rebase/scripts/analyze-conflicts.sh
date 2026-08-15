#!/bin/bash

# analyze-conflicts.sh
# Analyzes git rebase conflicts and provides detailed information
# Usage: bash analyze-conflicts.sh

set -eo pipefail

echo "🔍 Git Rebase Conflict Analyzer"
echo "======================================"
echo ""

# Check if we're in a rebase
if [ ! -d ".git/rebase-merge" ] && [ ! -d ".git/rebase-apply" ]; then
    echo "❌ No active rebase detected"
    echo "Start a rebase first: git rebase origin/main"
    exit 1
fi

echo "✓ Active rebase detected"
echo ""

# Get conflicting files
echo "📋 Conflicted Files:"
CONFLICTED_FILES=()
while IFS= read -r -d '' file; do
    CONFLICTED_FILES+=("$file")
done < <(git diff --name-only --diff-filter=U -z)
CONFLICT_COUNT=${#CONFLICTED_FILES[@]}

echo "   Found $CONFLICT_COUNT file(s) with conflicts:"
for file in "${CONFLICTED_FILES[@]}"; do
    printf '   - %q\n' "$file"
done
echo ""

# Analyze each conflict
echo "🔎 Conflict Analysis:"
echo ""

for file in "${CONFLICTED_FILES[@]}"; do
    printf '📄 File: %q\n' "$file"
    
    # Count conflict markers
    marker_status=0
    MARKER_COUNT=$(grep -c "<<<<<<" "$file") || marker_status=$?
    if [ "$marker_status" -gt 1 ]; then
        exit "$marker_status"
    fi
    MARKER_COUNT=${MARKER_COUNT:-0}
    echo "   Conflict sections: $MARKER_COUNT"
    
    # Show file size
    FILE_SIZE=$(wc -c < "$file")
    echo "   File size: $FILE_SIZE bytes"
    
    # Show line count
    LINE_COUNT=$(wc -l < "$file")
    echo "   Total lines: $LINE_COUNT"
    
    # Check for conflict markers
    if grep -q "<<<<<<" "$file"; then
        echo "   Status: ⚠️  UNRESOLVED (has conflict markers)"
        
        # Show first conflict
        FIRST_CONFLICT=$(grep -n "<<<<<<" "$file" | head -1 | cut -d: -f1)
        echo "   First conflict at line: $FIRST_CONFLICT"
        
        # Show context around first conflict (3 lines before and after markers)
        echo "   Context:"
        CONTEXT_START=$((FIRST_CONFLICT > 3 ? FIRST_CONFLICT - 3 : 1))
        CONTEXT_END=$((FIRST_CONFLICT + 10))
        sed -n "${CONTEXT_START},${CONTEXT_END}p" "$file" | \
            sed 's/^/      /'
    else
        echo "   Status: ✓ RESOLVED (no conflict markers)"
    fi
    
    echo ""
done

# Summary
echo "📊 Summary:"
UNRESOLVED=0
for file in "${CONFLICTED_FILES[@]}"; do
    marker_status=0
    grep -q "<<<<<<" "$file" || marker_status=$?
    if [ "$marker_status" -eq 0 ]; then
        UNRESOLVED=$((UNRESOLVED + 1))
    elif [ "$marker_status" -gt 1 ]; then
        exit "$marker_status"
    fi
done

RESOLVED=$((CONFLICT_COUNT - UNRESOLVED))

echo "   ✓ Resolved: $RESOLVED"
echo "   ⚠️  Unresolved: $UNRESOLVED"
echo ""

# Get rebase progress
if [ -f ".git/rebase-merge/msgnum" ]; then
    CURRENT=$(cat .git/rebase-merge/msgnum)
    TOTAL=$(cat .git/rebase-merge/end)
    echo "📈 Rebase Progress: $CURRENT / $TOTAL commits"
    echo ""
fi

# Suggestions
echo "💡 Next Steps:"
if [ "$UNRESOLVED" -gt 0 ]; then
    echo "   1. Edit conflicted files and resolve all markers"
else
    echo "   1. All conflicts appear resolved!"
fi
echo "   2. Run from the repository root: bash .agents/skills/git-rebase/scripts/validate-merge.sh"
echo "   3. Run from the repository root: make gate-full"
echo "   4. For focused frontend checks, use bunx turbo run <task> --filter=<workspace> from the repository root"
echo "   5. Run a manual smoke test if needed"
echo "   6. Stage the resolved files explicitly, then run: git rebase --continue"

echo ""
echo "✅ Analysis complete"
