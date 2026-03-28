# Lesson Learned

Extract software engineering takeaways from your recent code changes.

## Purpose

Most developers ship code and move on. But the fastest way to grow is to pause and reflect: what principle did I just apply? What trade-off did I make? What would I do differently?

This skill analyzes recent git history and surfaces the engineering lesson embedded in the work you just did.

## When to Use

- After finishing a feature or PR
- After a debugging session
- After a refactor
- When you want to reflect on recent work
- Trigger phrases: "what is the lesson here?", "what can I learn from this?", "engineering takeaway"

## How It Works

1. **Determine scope** -- Detects whether you're on a feature branch (analyzes branch vs main) or main (analyzes last 5 commits). You can also specify a commit range or specific SHA.
2. **Gather changes** -- Reads git log, diffs, and commit messages. For large diffs, selectively reads the most-changed files.
3. **Analyze patterns** -- Identifies structural decisions, trade-offs, and problems solved. Maps observations to named SE principles.
4. **Present the lesson** -- Delivers 1-3 specific, code-grounded takeaways with the principle name, how it manifested, why it matters, and an actionable next step.

## Key Features

- **Git-history-driven** -- Works from actual code changes, not hypotheticals
- **Principle-grounded** -- Maps observations to named SE principles (SOLID, DRY, KISS, YAGNI, etc.)
- **Specific, not generic** -- Every insight references actual files and lines from your diff
- **Balanced** -- Recognizes good patterns, not just areas for improvement
- **Configurable scope** -- Feature branch, last N commits, specific SHA, or working changes

## Output Format

```markdown
## Lesson: Separation of Concerns

**What happened in the code:**
The `AuthController` was split into `AuthController` and `UserService` across
commits abc123 and def456. Authentication logic stayed in the controller while
user CRUD operations moved to a dedicated service.

**The principle at work:**
Separation of Concerns -- different responsibilities should live in different
modules so each can change independently.

**Why it matters:**
Before the split, any change to user management risked breaking authentication.
Now each module can evolve independently with a clear interface between them.

**Takeaway for next time:**
When a file handles two distinct responsibilities, split early -- it's cheaper
than untangling them later.
```

## Prerequisites

- Must be in a git repository with commit history
- Works best with descriptive commit messages

## Related Files

- `references/se-principles.md` -- Curated SE principles catalog
- `references/anti-patterns.md` -- Common anti-patterns to detect in diffs
