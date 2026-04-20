---
status: resolved
file: internal/core/run/ui/remote_test.go
line: 234
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc57_ipA,comment:PRRC_kwDORy7nkc65JlV2
---

# Issue 003: _🛠️ Refactor suggestion_ | _🟠 Major_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_

**Reshape the new owner-session test to the required `t.Run("Should...")` form.**

The coverage is good, but this new case still lands as a one-off test body rather than the repo’s required subtest style.


As per coding guidelines, `**/*_test.go`: MUST use t.Run("Should...") pattern for ALL test cases.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/ui/remote_test.go` around lines 191 - 234, Wrap the test
body in a subtest using t.Run("Should keep owner sessions cancelable from local
quit", func(t *testing.T) { ... }) while keeping the existing setup/teardown and
logic intact: keep originalSetup := setupRemoteUISession and its deferred
restore outside the t.Run, then move the detachOnly declaration, the
setupRemoteUISession override function, the call to
AttachRemote(RemoteAttachOptions{OwnerSession: true, ...}), and all subsequent
assertions into the t.Run closure so the test conforms to the repository's
required t.Run("Should...") subtest pattern without changing behavior of
TestAttachRemoteKeepsOwnerSessionsCancelableFromLocalQuit, AttachRemote,
setupRemoteUISession, or the recordingUISession usage.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:4ce4a286-5e86-4ca3-8906-61b651e999ec -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Reasoning: the current repository guidance does not require wrapping each standalone Go test case in `t.Run("Should...")`, and the surrounding `internal/core/run/ui` package consistently uses top-level `Test...` functions for distinct behaviors. `remote_test.go` follows that package convention already: `TestAttachRemoteKeepsOwnerSessionsCancelableFromLocalQuit` is a standalone behavior check beside other standalone attach tests, not a table-driven subcase that needs shared `t.Run` structure. The review comment is therefore a style preference from the provider, not a repository rule violation or correctness defect in the scoped file.
- Resolution: no code change is required in `internal/core/run/ui/remote_test.go`; only the issue artifact needs to record that the finding is stale/invalid against the current branch standards.
- Verification: `make verify` passed after triage with formatting and lint clean (`0 issues`), test suite passing (`DONE 2416 tests, 1 skipped in 41.165s`), and build success (`All verification checks passed`).
