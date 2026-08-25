#!/bin/bash

# validate-merge.sh
# Validates that merge/rebase is clean and ready for testing
# Usage: bash validate-merge.sh

set -eo pipefail

echo "🔍 Merge/Rebase Validation"
echo "======================================"
echo ""

EXIT_CODE=0
UNMERGED_FILES=()
while IFS= read -r -d '' file; do
    UNMERGED_FILES+=("$file")
done < <(git diff --name-only --diff-filter=U -z)
CHANGED_FILES=()
while IFS= read -r -d '' file; do
    CHANGED_FILES+=("$file")
done < <(git diff HEAD --name-only --diff-filter=ACMRTUXB -z)

# 1. Check for remaining conflict markers
echo "1️⃣  Checking for conflict markers..."
MARKER_FILES=()
for file in "${CHANGED_FILES[@]}"; do
    [ -f "$file" ] || continue
    marker_status=0
    grep -q '^<<<<<<< \|^=======$\|^>>>>>>> ' "$file" || marker_status=$?
    if [ "$marker_status" -eq 0 ]; then
        MARKER_FILES+=("$file")
    elif [ "$marker_status" -gt 1 ]; then
        exit "$marker_status"
    fi
done
if [ "${#MARKER_FILES[@]}" -eq 0 ]; then
    echo "   ✓ No conflict markers found"
else
    echo "   ❌ FOUND conflict markers:"
    for file in "${MARKER_FILES[@]}"; do
        printf '   - %q\n' "$file"
    done
    echo ""
    echo "   ⚠️  Please resolve these files before continuing"
    EXIT_CODE=1
fi
echo ""

# 2. Check for unresolved git conflicts
echo "2️⃣  Checking git status..."
if [ "${#UNMERGED_FILES[@]}" -eq 0 ]; then
    echo "   ✓ No unresolved conflicts in git status"
else
    echo "   ❌ Git still shows unresolved conflicts:"
    for file in "${UNMERGED_FILES[@]}"; do
        printf '   - %q\n' "$file"
    done
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
dupe_status=0
COMMON_DUPES=$(git diff HEAD | grep -c '^+.*{$') || dupe_status=$?
if [ "$dupe_status" -gt 1 ]; then
    exit "$dupe_status"
fi
COMMON_DUPES=${COMMON_DUPES:-0}
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
CHANGED_COUNT=${#CHANGED_FILES[@]}
echo "   $CHANGED_COUNT files changed:"
for file in "${CHANGED_FILES[@]}"; do
    printf '   - %q\n' "$file"
done
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
    echo "  1. Run from the repository root: make gate"
    echo "  2. For focused frontend checks, use bunx turbo run <task> --filter=<workspace> from the repository root"
    echo "  3. Run a manual smoke test if needed"
    echo "  4. Then: git rebase --continue; after push, wait for PR CI"
else
    echo "❌ Validation FAILED - Fix issues before proceeding"
    echo ""
    echo "Issues found:"
    echo "  - Conflict markers remain (resolve in files)"
    echo "  - Git shows unresolved conflicts"
    echo "  - Other validation errors above"
    echo ""
    echo "Fix and run from the repository root: bash .agents/skills/git-rebase/scripts/validate-merge.sh"
fi

exit $EXIT_CODE
