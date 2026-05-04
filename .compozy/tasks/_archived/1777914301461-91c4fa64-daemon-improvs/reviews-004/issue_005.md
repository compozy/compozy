---
status: resolved
file: internal/store/globaldb/registry_test.go
line: 132
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58kNf-,comment:PRRC_kwDORy7nkc656GNF
---

# Issue 005: _🛠️ Refactor suggestion_ | _🟠 Major_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_

**Wrap the new cases in `t.Run("Should...")`.**

These additions are standalone tests, but the repo requires subtests as the default test shape.


As per coding guidelines, `**/*_test.go`: MUST use `t.Run("Should...")` pattern for ALL test cases.


Also applies to: 235-267

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/store/globaldb/registry_test.go` around lines 88 - 132, Wrap the
test body of TestGetByPathPrefersResolvedCanonicalWorkspaceRow in a subtest
using t.Run with a "Should..." description (e.g. t.Run("Should prefer resolved
canonical workspace row", func(t *testing.T) { ... })) and move the existing
t.Parallel() and all setup/assertions inside that subtest; do the same for the
other added test case referenced at lines 235-267 so both follow the
repository's t.Run("Should...") subtest pattern while keeping function name and
helper calls (db.Register, db.Get, db.now, etc.) unchanged.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:3b83986d-d641-4b98-9c1f-3d955d92a465 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Notes:
  - This file’s established pattern is standalone top-level tests with `t.Parallel()`. It does not use one-off wrapper subtests for single scenarios, and the surrounding tests at the cited lines follow that same shape.
  - Root cause of the review comment is an over-broad style interpretation: the repository guidance says subtests are the default pattern when they add structure, not that every standalone test must wrap its entire body in a single `t.Run("Should...")`.
  - Resolution: no code change was required. The current tests already match the local file’s established single-scenario pattern, and forcing one-case wrapper subtests would add ceremony without improving behavior, diagnostics, or coverage.
  - Verification: `go test ./internal/store/globaldb -run 'TestRegistryValidationBranches|TestCanonicalizeExistingPathCaseWithUsesOnDiskNames|TestCanonicalizeExistingPathCaseWithFallsBackToCleanPathWhenParentsCannotBeRead|TestGetByPathPrefersResolvedCanonicalWorkspaceRow' -count=1` passed. `make verify` also passed with `2548` tests, `2` skipped helper-process tests, and a successful `go build ./cmd/compozy`.
