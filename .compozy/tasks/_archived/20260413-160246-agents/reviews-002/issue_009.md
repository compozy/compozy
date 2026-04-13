---
status: resolved
file: internal/setup/reusable_agents.go
line: 221
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4092828776,nitpick_hash:6416c1966f22
review_hash: 6416c1966f22
source_review_id: "4092828776"
source_review_submitted_at: "2026-04-10T22:56:04Z"
---

# Issue 009: Consider logging instead of failing when backup cleanup fails.
## Review Comment

After line 220, the new agent is successfully installed in `targetPath`. If `removeReusableAgentPath(backupPath)` fails here, the function returns an error, causing the install to be reported as failed even though the new content is in place.

This leaves orphaned backup directories and misleads the caller about the install status. Consider either:
1. Logging the cleanup error and returning `nil` (install succeeded)
2. Collecting cleanup errors separately from install errors

## Triage

- Decision: `invalid`
- Notes:
  - `replaceReusableAgentInstallTarget` is modeling an atomic replace plus cleanup. If backup removal fails after the rename, the new agent content exists, but cleanup did not complete and operator-visible filesystem debt remains on disk.
  - The current API already reports this as a per-agent failure in the `failures` slice without aborting unrelated installs. Silently downgrading that cleanup failure would hide a real filesystem problem and overstate the install result.
  - There is no warning channel in this function signature today that preserves the cleanup error while still marking the install as fully successful, so I am keeping the current failure semantics.
  - Resolution: analysis complete; no code change required.
