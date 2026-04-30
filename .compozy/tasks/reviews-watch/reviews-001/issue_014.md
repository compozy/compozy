---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: internal/daemon/review_watch_git_test.go
line: 192
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4208391746,nitpick_hash:8058947b80f5
review_hash: 8058947b80f5
source_review_id: "4208391746"
source_review_submitted_at: "2026-04-30T20:37:25Z"
---

# Issue 014: Missing t.Parallel() in subtest.
## Review Comment

Per coding guidelines, independent subtests should use `t.Parallel()`. The subtest at line 193 is missing this call.

```diff
for _, failingCall := range requiredReads {
t.Run(failingCall, func(t *testing.T) {
+ t.Parallel()
wantErr := errors.New("read failed")
```

## Triage

- Decision: `valid`
- Root cause: The per-command failure subtests are independent and currently miss `t.Parallel()`, which violates the repository guidance for isolated subtests.
- Fix plan: Add `t.Parallel()` inside each affected subtest while preserving the existing command-failure assertions.
