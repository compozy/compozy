# Conflict Inspection

```bash
git status --short
git diff --name-only --diff-filter=U
git show REBASE_HEAD
git show :1:path/to/file  # merge base
git show :2:path/to/file  # rebased target state (ours)
git show :3:path/to/file  # replayed commit (theirs)
```

Inspect the actual hunks before resolving them. A marker search can find candidates in touched paths; documentation may intentionally contain marker examples. Never stage an unresolved file merely because a search found it.

Bundled helpers use `<skill-dir>/scripts/`: `pre-rebase-backup.sh`, `analyze-conflicts.sh`, and `validate-merge.sh`. Read the applicable helper's assumptions before running it; repository-specific gates remain authoritative.
