---
description: Fix issues for a given PR
---

<type>--type</type>
<pr>--pr</pr>
<from>--from</from>

## Helper Commands

Before starting work on fixing issues, use the `read_pr_issues.sh` script to review what needs to be addressed:

```bash
# Read all issues for a PR
scripts/read_pr_issues.sh --pr 277 --type issue --all

# Read a specific range of issues
scripts/read_pr_issues.sh --pr 277 --type issue --from 1 --to 10

# Read duplicated comments
scripts/read_pr_issues.sh --pr 277 --type duplicated --all

# Read outside-of-diff comments
scripts/read_pr_issues.sh --pr 277 --type outside --all

# Read nitpicks
scripts/read_pr_issues.sh --pr 277 --type nitpick --all
```

This script displays issues in a clean, readable format with:

- Issue numbers and titles
- File locations
- Current status (resolved/unresolved)
- Issue descriptions
- Thread IDs for GitHub reference

<critical>
- **YOU NEED** to fix the <type> from <from> in the ai-docs/reviews-pr-<pr>, and only finish when ALL THESE ISSUES are addressed;
- This should be fixed in THE BEST WAY possible, not using workarounds;
- **YOU MUST** follow project standards and rules from .cursor/rules, and ensure all parameters are addressed;
- If, in the end, you don't have all issues addressed, your work will be **INVALIDATED**;
- After making all the changes, you need to update the progress in the _summary.md file and all the related <type>.md files.
- **MUST DO:** If <type> is `issue`, after resolving every issue run `scripts/resolve_pr_issues.sh --pr-dir ai-docs/reviews-pr-<pr> --from <start> --to <end>` so the script calls `gh` to close the review threads and refreshes the summary.
</critical>

<after_finish>
- **MUST COMMIT:** After fixing ALL issues in this batch and ensuring make lint && make test pass,
  commit the changes with a descriptive message that references the PR and fixed issues.
  Example: "git commit -am "fix(repo): resolve PR #%s issues [batch]"
  Note: Commit locally only - do NOT push. Multiple batches will be committed separately.
</after_finish>
