---
status: resolved
file: pkg/compozy/runs/run_test.go
line: 62
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148016854,nitpick_hash:1c316f2b12c1
review_hash: 1c316f2b12c1
source_review_id: "4148016854"
source_review_submitted_at: "2026-04-21T13:29:50Z"
---

# Issue 031: Wrap this case in a Should... subtest and run it in parallel.
## Review Comment

The assertions are good; this is just to keep test structure aligned with enforced project standards.

As per coding guidelines `**/*_test.go`: `MUST use t.Run("Should...") pattern for ALL test cases` and `Use t.Parallel() for independent subtests`.

## Triage

- Decision: `valid`
- Root cause: `TestAdaptRemoteRunSnapshotPreservesIncompleteReasons` is a single self-contained case that does not follow the repository's preferred `t.Run("Should...")` test-case shape.
- Fix approach: wrap the assertions in a named subtest and run that subtest with `t.Parallel()` because it does not mutate shared package-level state.
- Resolution: the snapshot adaptation assertions now run inside a named `Should...` subtest with `t.Parallel()`.
- Verification: `go test ./pkg/compozy/runs` and `make verify`
