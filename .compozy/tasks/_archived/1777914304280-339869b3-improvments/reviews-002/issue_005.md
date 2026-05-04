---
provider: coderabbit
pr: "131"
round: 2
round_created_at: 2026-04-30T16:05:39.30025Z
status: resolved
file: web/src/systems/app-shell/components/app-shell-container.tsx
line: 218
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4206542727,nitpick_hash:d27e6eedb3ae
review_hash: d27e6eedb3ae
source_review_id: "4206542727"
source_review_submitted_at: "2026-04-30T15:47:24Z"
---

# Issue 005: Consider adding recursion depth limit for safety.
## Review Comment

`extractWorkspaceId` recursively traverses objects without a depth limit. While React Query keys/variables are typically shallow, a malformed payload could cause a stack overflow.

This is low risk given the controlled input source, but a simple depth counter would provide defensive safety.

## Triage

- Decision: `VALID`
- Notes:
  - `extractWorkspaceId` recursively traverses arrays and object values from React Query sources without a maximum depth.
  - The normal query-key/error shapes are shallow, but a malformed nested value can recurse until the JavaScript stack overflows.
  - Fix approach: add a bounded depth parameter with a conservative maximum, return `null` beyond that limit, and cover the defensive behavior with an AppShellContainer regression test.
  - Resolution: implemented and verified with the repository verification pipeline.
