# Scripts & Tools Reference

## bash: Analyze Current Conflicts

```bash
# See all conflicts at a glance
git diff --name-only --diff-filter=U

# Count conflicts
git diff --name-only --diff-filter=U | wc -l

# Show conflicts with context
git status | grep 'both modified'
```

## bash: Verify No Conflict Markers Remain

```bash
# Check for leftover conflict markers
git grep -l '<<<<<<'

# If any files show up, you missed editing them
git add <missing-file>
git rebase --continue
```

## bash: Compare Your vs Incoming Changes

```bash
# See only YOUR changes in this conflict
git show :1:<filename>  # Base version
git show :2:<filename>  # Your version
git show :3:<filename>  # Their version

# Compare two versions:
git show :2:<filename> > yours.txt
git show :3:<filename> > theirs.txt
diff yours.txt theirs.txt
```

## Using Bundled Scripts

This skill includes helper scripts in `scripts/`:

- **`pre-rebase-backup.sh`**: Creates a safety backup before rebasing
- **`analyze-conflicts.sh`**: Analyzes current conflicts and provides detailed information
- **`validate-merge.sh`**: Validates that merge/rebase is clean and ready for testing

See SKILL.md for usage instructions.
