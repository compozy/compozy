---
name: git-rebase
description: Intelligently handle git rebase operations and resolve merge conflicts while preserving features and maintaining code quality. Use when rebasing feature branches, resolving conflicts across commits, and ensuring clean linear history without losing changes.
---

# Git Rebase with Intelligent Conflict Resolution

## Quick Start

For most rebases with multiple commits, use the squash-first strategy to resolve conflicts only once:

```bash
# Step 1: Backup current state
bash scripts/pre-rebase-backup.sh

# Step 2: Squash commits (interactive rebase on current branch)
git rebase -i $(git merge-base HEAD origin/main)

# Step 3: Rebase onto target
git rebase origin/main

# Step 4: If conflicts, resolve them once (see workflow below)
# Then continue: git rebase --continue

# Step 5: Force push safely
git push origin $(git rev-parse --abbrev-ref HEAD) --force-with-lease
```

This approach resolves conflicts once instead of per-commit, saving time and mental overhead.

## Core Workflow: Conflict Analysis & Resolution

Copy this checklist and mark progress:

```
Rebase Workflow:
- [ ] Step 1: Create safety backup
- [ ] Step 2: Fetch latest from target branch
- [ ] Step 3: Analyze conflict scope
- [ ] Step 4: Choose resolution strategy
- [ ] Step 5: Apply conflict resolutions
- [ ] Step 6: Validate merged code
- [ ] Step 7: Run tests
- [ ] Step 8: Force push safely
```

### Step 1: Create Safety Backup

ALWAYS do this first. If rebase goes wrong, you can recover:

```bash
# Use the bundled script
bash scripts/pre-rebase-backup.sh

# Or manually create timestamped backup branch
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
git branch backup-rebase-$TIMESTAMP

# Alternative: Create temporary ref to your current commit
git reflog  # Note your current HEAD SHA for manual recovery
```

This costs nothing and saves hours of work if something goes wrong.

### Step 2: Fetch Latest Changes

Ensure you have the most recent remote state:

```bash
# Fetch without modifying local branches
git fetch origin

# View the divergence
git log --oneline origin/main..HEAD  # Your commits
git log --oneline HEAD..origin/main  # New commits on main
```

Understand how many commits you're rebasing and how much main has changed.

### Step 3: Analyze Conflict Scope

Before starting the rebase, predict conflicts:

```bash
# See which files you changed
git diff --name-only origin/main...HEAD

# See which files main changed
git diff --name-only origin/main HEAD

# Likely conflict areas: files changed in both
```

**Key insight**: If you changed `auth.ts` and so did main, you WILL get conflicts in `auth.ts`.

Anticipating conflicts helps you understand how to resolve them.

### Step 4: Choose Resolution Strategy

**For detailed strategy comparison and decision matrix**, see [references/strategies.md](references/strategies.md).

#### Strategy A: Squash First (Recommended for 3+ commits)

**When to use**: Multiple feature commits with many conflicts expected

**Why**: Reduces conflicts to one resolution phase instead of per-commit

```bash
# Interactive rebase on current branch first
git rebase -i $(git merge-base HEAD origin/main)

# In editor, change all "pick" to "squash" (or 's') except first commit
# Save and exit - commits are squashed into one
# Edit commit message to describe the entire feature

# Now rebase the squashed commit
git rebase origin/main

# Resolve conflicts once, then git rebase --continue
```

**Tradeoffs**: Lose individual commit history, but simpler conflict resolution

#### Strategy B: Interactive Rebase with Conflict Awareness

**When to use**: 1-2 commits, clean history, or complex per-commit logic

```bash
git rebase -i origin/main

# In editor, you can:
# - Reorder commits to isolate conflict-prone ones
# - Drop commits that are already in main (git detects this)
# - Combine related commits before rebasing

# Save and exit - rebase proceeds, stopping at conflicts
```

**Tradeoffs**: More control, but more conflict-resolution iterations

#### Strategy C: Simple Linear Rebase (Fastest, Auto-Resolution)

**When to use**: Simple cases, no critical decisions, or in automated pipelines

```bash
# Rebase all commits at once
git rebase origin/main

# If no conflicts, done
# If conflicts, you resolve each one
```

**Warning**: Not recommended for complex scenarios. Use Strategies A or B instead.

### Step 5: Apply Conflict Resolutions

When `git rebase` pauses with conflicts, use the analysis script:

```bash
# Analyze conflicts
bash scripts/analyze-conflicts.sh

# See which files conflict
git status

# For each conflicted file:
# - RECOMMENDED: Use merge tool for visual clarity
git mergetool --no-prompt

# - ALTERNATIVE: Manual edit in your editor
# Search for conflict markers: <<<<<<, ======, >>>>>>
```

#### Conflict Marker Anatomy

```javascript
<<<<<<< HEAD
// Your current feature code
function authenticate(token) {
  validateToken(token);
  return true;
}
=======
// Main branch code (incoming)
function authenticate(token) {
  if (!token) throw new Error("No token");
  validateToken(token);
  setSession(token);
  return true;
}
>>>>>>> origin/main
```

**Decision framework** (before deleting markers):

1. **Can you keep both?** YES → Merge them intelligently

   ```javascript
   function authenticate(token) {
     if (!token) throw new Error("No token"); // Keep main's validation
     validateToken(token);
     setSession(token); // Keep main's session setup
     return true; // Keep feature's return
   }
   ```

2. **Conflicting logic?** Understand WHY they differ, then decide
   - Did main add critical security checks? → Keep main's version
   - Did your feature add essential functionality? → Keep feature's version
   - Are they trying to do different things? → Combine intentionally

3. **Lost features?** NEVER let a feature silently disappear
   - If you added authentication logic, ensure it's in final version
   - If main improved database access, ensure that's preserved

**For detailed resolution patterns**, see [references/resolution-patterns.md](references/resolution-patterns.md).

#### Key Resolution Principles

**✅ DO**:

- Keep both versions' important functionality when possible
- Use the merge tool for visual representation
- Add comments explaining merged conflicts: `// Merged from both versions: main's validation + feature's session setup`
- Test each file after resolution

**❌ DON'T**:

- Mindlessly pick one version without understanding both
- Delete conflict markers without understanding the conflict
- Keep duplicate code - merge intelligently
- Skip testing before continuing

### Step 6: Validate Merged Code

After resolving conflicts, validate the merged code:

```bash
# Use the validation script
bash scripts/validate-merge.sh

# Manual checks:
# 1. Check syntax
npm run lint  # or eslint, pylint, etc.

# 2. Check types (if TypeScript)
npm run type-check  # or tsc --noEmit

# 3. Spot-check key files
git diff HEAD origin/main -- <conflicted-file>

# If validation fails:
# 1. Fix the issue in the file
# 2. git add <file>
# 3. git rebase --continue
```

**Important**: Validation catches mistakes BEFORE you commit them.

### Step 7: Run Tests

This is your safety net:

```bash
# Run full test suite
npm test

# Or specific tests for changed areas
npm test -- --testPathPattern=auth  # If auth.ts was changed

# If tests fail:
# 1. Understand what broke
# 2. Fix in the files
# 3. git add <files>
# 4. git rebase --continue
```

**Rule**: Never force-push code that fails tests.

### Step 8: Force Push Safely

Use `--force-with-lease` instead of `--force`. It protects against accidentally overwriting others' work:

```bash
# SAFE: Protects others' commits
git push origin $(git rev-parse --abbrev-ref HEAD) --force-with-lease

# UNSAFE: Can overwrite others' work
git push origin your-branch -f  # Don't do this

# If force-with-lease fails:
# Someone else pushed to your branch
# Coordinate with them before forcing
```

## Common Scenarios & Strategies

### Scenario 1: Many Small Conflicts Across 5+ Commits

**Use**: Squash-first strategy

```bash
git rebase -i $(git merge-base HEAD origin/main)
# Mark all but first commit as 's' (squash)
# Save - commits squash into one

git rebase origin/main
# Resolve conflicts once
git rebase --continue

# Only one conflict-resolution phase!
```

**Why**: Each commit might have conflicts. Squashing before rebasing means one pass.

### Scenario 2: One Specific Commit Has Conflicts

**Use**: Target that commit with interactive rebase

```bash
git rebase -i origin/main

# In editor, move the problematic commit to the end
# Save - rebase proceeds, stopping at that commit

# When it stops, you know exactly which commit conflicts
git status  # See what changed in this commit

# Resolve, then continue
git rebase --continue
```

**Why**: Isolating the commit helps you understand what it's trying to do.

### Scenario 3: Conflicts Keep Repeating (Same File, Different Commits)

**Use**: Git rerere (reuse recorded resolution)

```bash
# Enable rerere globally (one-time setup)
git config --global rerere.enabled true

# Now when you hit the same conflict in a second commit,
# Git automatically applies the first commit's resolution

git rebase origin/main

# Git remembers your first conflict resolution
# and replays it automatically for similar conflicts
```

**Why**: When rebasing and hitting the same file repeatedly, rerere saves manual work.

### Scenario 4: Rebase Conflicts Are Too Complex

**Emergency escape plan**:

```bash
# Abort the rebase - return to original state
git rebase --abort

# Fall back to merge (safer for complex scenarios)
git merge origin/main

# Or try a different approach:
# - Squash your entire feature branch first
# - Cherry-pick main's critical changes selectively
```

**Important**: It's okay to abort and rethink. Better than a broken rebase.

## Bundled Scripts

This skill includes helper scripts in `scripts/`:

- **`pre-rebase-backup.sh`**: Creates a safety backup before rebasing
- **`analyze-conflicts.sh`**: Analyzes current conflicts and provides detailed information
- **`validate-merge.sh`**: Validates that merge/rebase is clean and ready for testing

Run scripts with `bash scripts/<script-name>.sh`.

## Reference Files

For detailed information on specific topics:

- **Strategies**: [references/strategies.md](references/strategies.md) - Complete strategy comparison and decision matrix
- **Resolution Patterns**: [references/resolution-patterns.md](references/resolution-patterns.md) - Common conflict resolution patterns and heuristics
- **Troubleshooting**: [references/troubleshooting.md](references/troubleshooting.md) - Solutions to common rebase problems
- **Automation**: [references/automation.md](references/automation.md) - Automated conflict resolution for CI/CD
- **Scripts & Tools**: [references/scripts-tools.md](references/scripts-tools.md) - Additional git commands and tool usage

## Checklist: Before You Commit to Rebasing

- [ ] Understand why you're rebasing (cleaner history, syncing with main, etc.)
- [ ] Backup your current branch: `bash scripts/pre-rebase-backup.sh`
- [ ] Run tests on current branch - they should pass
- [ ] Fetch latest: `git fetch origin`
- [ ] Understand what you'll be rebasing (5 commits? 50?)
- [ ] Understand main's recent changes (1 commit? Major refactor?)
- [ ] Choose your strategy (see [references/strategies.md](references/strategies.md))
- [ ] Have your merge tool ready (if using GUI)
- [ ] Block 30 mins - don't rush conflict resolution
- [ ] Have tests ready to run after rebase

## When NOT to Rebase

- Shared branches (use merge instead: `git merge origin/main`)
- Critical production code without comprehensive tests
- When multiple people are pushing to the same branch
- If you don't understand the conflicts you're seeing

**Default to merge if uncertain.** Merge is safer for collaborative work.

**Use rebase when**: Solo feature branch, clean history matters, no shared dependencies.

## Summary: The Rebase Philosophy

Rebasing is a tool for creating clean, linear commit history. **Used well**, it makes debugging and code review easier. **Used poorly**, it loses work.

**The key principle**: Understand every conflict before resolving it. Don't automate away the thinking.

With this Skill, you can rebase with confidence, understanding each decision and protecting your code throughout the process.
