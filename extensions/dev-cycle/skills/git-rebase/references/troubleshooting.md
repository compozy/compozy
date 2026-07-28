# Troubleshooting Common Rebase Issues

## Problem: "CONFLICT (content): Merge conflict in X"

**What it means**: File X has conflicting changes from both branches

**Solution**:

1. `git status` to see all conflicts
2. Edit each conflicted file (look for `<<<<<<<` markers)
3. `git add <file>`
4. `git rebase --continue`

## Problem: "fatal: cannot lock ref 'refs/heads/...'"

**What it means**: Git can't write to branch (another command is running or permission issue)

**Solution**:

```bash
# Check if another git process is running
ps aux | grep git

# Kill it if safe: kill <PID>

# Or retry after a moment
# If persistent: check file permissions in .git/
```

## Problem: Rebase seems stuck (no prompt)

**What it means**: Your editor didn't open, or it's waiting for input

**Solution**:

```bash
# Check if you have an editor set
git config --global core.editor

# Set one if not configured
git config --global core.editor "nano"  # or "vim", "code", etc.

# Resume rebase
git rebase --continue
```

## Problem: "Your branch has diverged"

**What it means**: After rebase, local and remote have different history

**Solution**:

```bash
# This is EXPECTED after rebase
# Force push safely to update remote
git push origin $(git rev-parse --abbrev-ref HEAD) --force-with-lease
```

## Problem: Lost Commits After Force Push

**Recovery**:

```bash
# Your backup branch saves you
git reset --hard backup-rebase-<timestamp>

# Or use reflog to find the SHA
git reflog
git reset --hard <SHA>
```
