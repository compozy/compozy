#!/bin/bash

# analyze-conflicts.sh
# Analyzes git rebase conflicts and provides detailed information
# Usage: bash analyze-conflicts.sh

set -e

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
CONFLICTED_FILES=$(git diff --name-only --diff-filter=U)
CONFLICT_COUNT=$(echo "$CONFLICTED_FILES" | wc -l)

echo "   Found $CONFLICT_COUNT file(s) with conflicts:"
echo "$CONFLICTED_FILES" | sed 's/^/   - /'
echo ""

# Analyze each conflict
echo "🔎 Conflict Analysis:"
echo ""

for file in $CONFLICTED_FILES; do
    echo "📄 File: $file"
    
    # Count conflict markers
    MARKER_COUNT=$(($(grep -c "<<<<<<" "$file" || echo 0)))
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
        sed -n "$((FIRST_CONFLICT-3)),$((FIRST_CONFLICT+10))p" "$file" | \
            sed 's/^/      /'
    else
        echo "   Status: ✓ RESOLVED (no conflict markers)"
    fi
    
    echo ""
done

# Summary
echo "📊 Summary:"
UNRESOLVED=$(for f in $CONFLICTED_FILES; do
    grep -l "<<<<<<" "$f" 2>/dev/null || true
done | wc -l)

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
    echo "   2. Run: bash scripts/validate-merge.sh"
    echo "   3. Run: npm run lint && npm test"
    echo "   4. Run: git add ."
    echo "   5. Run: git rebase --continue"
else
    echo "   1. All conflicts appear resolved!"
    echo "   2. Run: bash scripts/validate-merge.sh"
    echo "   3. Run: npm run lint && npm test"
    echo "   4. Run: git add ."
    echo "   5. Run: git rebase --continue"
fi

echo ""
echo "✅ Analysis complete"
