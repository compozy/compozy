# Conflict Resolution Strategies Reference

## Strategy Decision Matrix

Choose your strategy based on your situation:

| Scenario                 | Strategy      | Complexity | Risk     | Time      | Best For                |
| ------------------------ | ------------- | ---------- | -------- | --------- | ----------------------- |
| 3+ commits, many changes | Squash First  | Medium     | Low      | Fast      | Feature branches, CI/CD |
| 1-2 commits, simple      | Simple Rebase | Low        | Low      | Fast      | Quick hotfixes          |
| Clean history needed     | Interactive   | High       | Medium   | Slow      | Production code         |
| Same conflicts repeat    | Rerere        | Low        | Very Low | Very Fast | Complex rebases         |
| Unknown complexity       | Merge instead | Very Low   | Very Low | Fast      | When in doubt           |

---

## Deep Dive: Each Strategy

### 1. Squash First (Recommended)

**How it works**:

```bash
# Squash all commits into one
git rebase -i origin/main  # Keeps your branch's history intact
# Mark commits as 's' (squash) to combine them
# Then rebase once against main
```

**When to use**:

- 3+ commits to rebase
- Multiple files changed
- Conflicts expected
- Want clean commit history on main

**Advantages**:

- ✅ Resolve conflicts ONCE (not per-commit)
- ✅ Final code is one clean commit
- ✅ Faster for many commits
- ✅ Easy to understand final state

**Disadvantages**:

- ❌ Lose intermediate commit history
- ❌ Can't debug individual commits later
- ❌ Harder to bisect if bug is introduced

**Example**:

```bash
$ git rebase -i $(git merge-base HEAD origin/main)

# Editor shows:
pick a1b2c3d feat: add authentication
pick d4e5f6g fix: handle edge case
pick h7i8j9k chore: update dependencies

# Change to:
pick a1b2c3d feat: add authentication
s d4e5f6g fix: handle edge case
s h7i8j9k chore: update dependencies

# Save - commits squash into one
# Write new commit message describing entire feature

$ git rebase origin/main
# Resolve conflicts once
$ git rebase --continue
$ git push origin feature-branch --force-with-lease
```

---

### 2. Interactive Rebase (Full Control)

**How it works**:

```bash
git rebase -i origin/main
# Reorder, edit, squash individual commits before rebasing
```

**When to use**:

- Want to keep some commits, squash others
- Need to reorder commits strategically
- Want to reword commit messages
- History preservation is important

**Advantages**:

- ✅ Full control over commit order
- ✅ Can keep important intermediate commits
- ✅ Can edit commit messages
- ✅ Excellent for code review

**Disadvantages**:

- ❌ More conflict resolution iterations
- ❌ Takes more time
- ❌ More error-prone

**Interactive rebase commands**:

```
pick    = use commit
reword  = use commit, but edit message
squash  = use commit but combine with previous
fixup   = like squash, but discard log message
drop    = remove commit
exec    = run command between commits
```

**Example**:

```bash
$ git rebase -i origin/main

# Before:
pick commit1 - feat: part A
pick commit2 - feat: part B
pick commit3 - test: add tests
pick commit4 - chore: lint

# Edit to:
pick commit1 - feat: part A
squash commit2 - feat: part B      # Combine with part A
squash commit3 - test: add tests   # Include tests
drop commit4 - chore: lint         # Remove lint-only commit

# Save and edit final message to describe everything
```

---

### 3. Simple Linear Rebase (Fastest)

**How it works**:

```bash
git rebase origin/main
# Git replays all your commits directly, no intermediate edits
```

**When to use**:

- 1-2 commits only
- Simple changes, minimal conflicts
- Don't care about reordering
- Want fastest possible rebase

**Advantages**:

- ✅ Fastest to execute
- ✅ Simplest conceptually
- ✅ Preserves commit history as-is

**Disadvantages**:

- ❌ Conflict per commit (can be slow)
- ❌ Can't reorder or edit commits
- ❌ Loses opportunity to clean up history

---

### 4. Git Rerere (Smart Replay)

**How it works**:

```bash
git config --global rerere.enabled true
# Git records your conflict resolutions
# Automatically replays them when same conflict appears
```

**When to use**:

- Rebasing many commits against target with repeated conflicts
- Same file changed in multiple of your commits
- Want to avoid re-resolving the same conflicts

**Advantages**:

- ✅ Automatic conflict replay
- ✅ Huge time savings for complex rebases
- ✅ Consistent resolutions

**Disadvantages**:

- ❌ Can hide mistakes if recorded wrong
- ❌ Requires careful first resolution
- ❌ Only works for identical conflict patterns

**How to use**:

```bash
# Enable (one-time)
git config --global rerere.enabled true

# Now when you rebase:
git rebase origin/main

# First conflict: You resolve it
# [Edit file, git add, git rebase --continue]

# Second identical conflict: Git automatically applies same resolution
# Just keep going: git rebase --continue

# Git has learned your conflict pattern and replays it
```

**Track rerere database**:

```bash
# See all recorded resolutions
ls -la .git/rr-cache/

# Review a specific resolution
cat .git/rr-cache/<conflict-hash>/preimage
cat .git/rr-cache/<conflict-hash>/postimage
```

---

### 5. Merge Instead (Safest Alternative)

**How it works**:

```bash
git merge origin/main
# Combines two branches, creates merge commit
```

**When to use**:

- Shared branches (never rebase shared branches!)
- Too complex to rebase
- Team collaboration on same branch
- Want to preserve branch history

**Advantages**:

- ✅ Preserves both histories
- ✅ Can't lose commits
- ✅ Safe for shared branches
- ✅ Clearer integration point

**Disadvantages**:

- ❌ Less clean history
- ❌ Creates merge commit
- ❌ Harder to bisect
- ❌ More verbose history

---

## Automation Strategies (Pipeline Use)

### Risky: Auto-Accept With `-X`

```bash
# Accept all INCOMING changes (main's version)
git rebase -X theirs origin/main

# Accept all YOUR changes (your version)
git rebase -X ours origin/main

# Three-way recursive merge (safest auto-strategy)
git rebase -X recursive origin/main
```

**ONLY use when**:

1. You have comprehensive tests covering all changes
2. You understand the implications
3. You've verified the strategy on a test branch first
4. Changes are non-critical (not user-facing)

**Recommendation**: Avoid auto-resolution in production pipelines.

---

## Conflict Complexity Indicators

### Low Complexity (Safe to Auto-Resolve)

- ✅ Changes in different files
- ✅ Same file, different functions/methods
- ✅ Whitespace/formatting conflicts
- ✅ Import reordering

### Medium Complexity (Manual Review Needed)

- ⚠️ Changes in same function but different branches
- ⚠️ Logic changes (if statements, loops)
- ⚠️ Return values or flow changes
- ⚠️ API changes

### High Complexity (Very Careful)

- ❌ Same line changed differently
- ❌ Critical business logic
- ❌ Security/authentication code
- ❌ Database migrations

---

## Decision Tree: Which Strategy?

```
START: Do I need to rebase?
  |
  +-> Is this a shared branch?
      |-> YES: Use MERGE instead, don't rebase
      |-> NO: Continue
  |
  +-> How many commits to rebase?
      |-> 1-2 commits:
      |   +-> Use SIMPLE LINEAR REBASE
      |   +-> Quick, straightforward
      |
      |-> 3-10 commits:
      |   +-> Are conflicts expected?
      |   +-> YES: Use SQUASH FIRST
      |   +-> NO: Use INTERACTIVE REBASE
      |
      |-> 10+ commits:
          +-> Almost always: Use SQUASH FIRST
          +-> Too many conflict iterations otherwise
  |
  +-> Any other concerns?
      |-> I've rebased this file before: Enable RERERE
      |-> Feeling uncertain: Use MERGE instead
      |-> Need clean history: Use INTERACTIVE
      |-> Just want it done: Use SQUASH FIRST

END: Choose strategy and proceed
```

---

## Post-Resolution Validation

After resolving conflicts, always validate:

```bash
# 1. Check syntax (prevents runtime errors)
npm run lint
npm run type-check  # If TypeScript

# 2. Check logic (review changes)
git diff HEAD origin/main -- <conflicted-file>

# 3. Run tests (catches integration issues)
npm test

# 4. Manual smoke test (try key features)
npm start
# Test the feature manually

# 5. Only then force push
git push origin branch --force-with-lease
```

**Rule**: Never push code you haven't tested.
