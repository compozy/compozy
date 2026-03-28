#!/bin/bash

# validate-merge.sh
# Validates that merge/rebase is clean and ready for testing
# Usage: bash validate-merge.sh

set -e

echo "🔍 Merge/Rebase Validation"
echo "======================================"
echo ""

EXIT_CODE=0

# 1. Check for remaining conflict markers
echo "1️⃣  Checking for conflict markers..."
if git grep -l '<<<<<<\|======\|>>>>>>' 2>/dev/null | wc -l | grep -q '^0$'; then
    echo "   ✓ No conflict markers found"
else
    echo "   ❌ FOUND conflict markers:"
    git grep -l '<<<<<<\|======\|>>>>>>' 2>/dev/null || true
    echo ""
    echo "   ⚠️  Please resolve these files before continuing"
    EXIT_CODE=1
fi
echo ""

# 2. Check for unresolved git conflicts
echo "2️⃣  Checking git status..."
if [ -z "$(git diff --name-only --diff-filter=U)" ]; then
    echo "   ✓ No unresolved conflicts in git status"
else
    echo "   ❌ Git still shows unresolved conflicts:"
    git diff --name-only --diff-filter=U | sed 's/^/   - /'
    EXIT_CODE=1
fi
echo ""

# 3. Check if working directory is clean
echo "3️⃣  Checking working directory..."
if git status --porcelain | grep -q '^ M\| ??'; then
    echo "   ⚠️  Unstaged changes found:"
    git status --short | sed 's/^/   /'
    echo ""
    echo "   Tip: Run 'git add .' to stage all changes"
else
    echo "   ✓ Working directory clean"
fi
echo ""

# 4. Check for duplicate code (merged sections)
echo "4️⃣  Checking for duplicate code patterns..."
COMMON_DUPES=$(git diff HEAD | grep -c '^+.*{$' || echo 0)
if [ "$COMMON_DUPES" -lt 10 ]; then
    echo "   ✓ No obvious code duplication detected"
else
    echo "   ⚠️  Possible code duplication (many new code blocks):"
    echo "   Manual review recommended"
fi
echo ""

# 5. Check for common merge markers in comments
echo "5️⃣  Checking for merge message artifacts..."
if git diff HEAD | grep -i "merged\|conflict\|cherry-pick\|rebase"; then
    echo "   ⚠️  Found merge-related comments in changes"
    echo "   Review to ensure they're intentional"
else
    echo "   ✓ No merge artifacts in comments"
fi
echo ""

# 6. Show changed files
echo "6️⃣  Changed Files Summary:"
CHANGED=$(git diff --name-only)
CHANGED_COUNT=$(echo "$CHANGED" | grep -c . || echo 0)
echo "   $CHANGED_COUNT files changed:"
echo "$CHANGED" | sed 's/^/   - /'
echo ""

# 7. Show stats
echo "7️⃣  Diff Statistics:"
git diff --stat | sed 's/^/   /'
echo ""

# Final status
echo "======================================"
if [ $EXIT_CODE -eq 0 ]; then
    echo "✅ Validation PASSED - Ready for testing!"
    echo ""
    echo "Next steps:"
    echo "  1. Run tests: npm test"
    echo "  2. Run linter: npm run lint"
    echo "  3. Manual smoke test if needed"
    echo "  4. Then: git rebase --continue"
else
    echo "❌ Validation FAILED - Fix issues before proceeding"
    echo ""
    echo "Issues found:"
    echo "  - Conflict markers remain (resolve in files)"
    echo "  - Git shows unresolved conflicts"
    echo "  - Other validation errors above"
    echo ""
    echo "Fix and run: bash validate-merge.sh"
fi

exit $EXIT_CODE
