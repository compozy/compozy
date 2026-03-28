#!/bin/bash

# pre-rebase-backup.sh
# Creates a safe backup before starting a rebase
# Usage: bash pre-rebase-backup.sh [branch-name]

echo "🔐 Pre-Rebase Safety Backup"
echo "======================================"
echo ""

# Get current branch
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)

if [ "$CURRENT_BRANCH" = "HEAD" ]; then
    echo "❌ You're in detached HEAD state"
    echo "Checkout a branch first: git checkout your-branch"
    exit 1
fi

# Check if clean
if [ -n "$(git status --porcelain)" ]; then
    echo "⚠️  Warning: You have uncommitted changes"
    echo "Commit or stash them first:"
    echo "  git add . && git commit -m 'work in progress'"
    echo "  or"
    echo "  git stash"
    exit 1
fi

echo "Current branch: $CURRENT_BRANCH"
echo ""

# Create timestamped backup
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_BRANCH="backup-rebase-${CURRENT_BRANCH}-${TIMESTAMP}"

echo "Creating backup branch..."
git branch "$BACKUP_BRANCH"

# Get current commit
CURRENT_SHA=$(git rev-parse HEAD)

echo ""
echo "✅ Backup created successfully!"
echo ""
echo "📋 Backup Details:"
echo "   Branch name: $BACKUP_BRANCH"
echo "   Commit SHA: $CURRENT_SHA"
echo ""

# Save backup info to file for easy recovery
BACKUP_INFO_FILE=".rebase-backup-info"
cat > "$BACKUP_INFO_FILE" << EOF
# Rebase Backup Information
# Created: $(date)

BACKUP_BRANCH=$BACKUP_BRANCH
CURRENT_BRANCH=$CURRENT_BRANCH
COMMIT_SHA=$CURRENT_SHA
TIMESTAMP=$TIMESTAMP

# To recover from this backup:
# git reset --hard backup-rebase-${CURRENT_BRANCH}-${TIMESTAMP}
EOF

echo "💾 Backup info saved to: $BACKUP_INFO_FILE"
echo ""

# Show git reflog for additional recovery options
echo "📚 Backup Recovery Instructions:"
echo ""
echo "If something goes wrong, you can recover using:"
echo ""
echo "  Option 1 (Recommended): Use the backup branch"
echo "    git reset --hard $BACKUP_BRANCH"
echo ""
echo "  Option 2: Use git reflog"
echo "    git reflog  # Find your commit SHA"
echo "    git reset --hard <SHA>"
echo ""

# List all backup branches
echo "🗂️  All Backup Branches:"
git branch | grep "backup-rebase" | tail -5 | sed 's/^/  - /'
echo ""

echo "✅ You're ready to rebase safely!"
echo ""
echo "Next steps:"
echo "  1. git fetch origin"
echo "  2. git rebase origin/main"
echo "  3. Resolve any conflicts"
echo "  4. git rebase --continue"
echo "  5. git push origin $CURRENT_BRANCH --force-with-lease"
