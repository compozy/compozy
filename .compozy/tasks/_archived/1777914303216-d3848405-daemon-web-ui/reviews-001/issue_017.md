---
status: resolved
file: internal/daemon/sync_transport_service_test.go
line: 103
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58gqX-,comment:PRRC_kwDORy7nkc651WIC
---

# Issue 017: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Split these into `t.Run("Should...")` subtests (table-driven by default).**

The scenarios are comprehensive, but both tests are currently monolithic. Please restructure into named subtests (and parallelize independent ones) to align with the repository’s test requirements.



<details>
<summary>♻️ Suggested structure</summary>

```diff
-func TestSyncTransportServiceResolvesTargetsAndUnavailableBranches(t *testing.T) {
+func TestSyncTransportService_ShouldResolveTargetsAndHandleUnavailableBranches(t *testing.T) {
+  cases := []struct{
+    name string
+    // inputs + expected outputs/errors
+  }{
+    {name: "Should sync by path"},
+    {name: "Should sync by workspace and slug"},
+    {name: "Should fail when target is missing"},
+    {name: "Should fail when service is unavailable"},
+  }
+  for _, tc := range cases {
+    tc := tc
+    t.Run(tc.name, func(t *testing.T) {
+      // t.Parallel() for independent cases
+      // case logic/assertions
+    })
+  }
}
```
</details>

As per coding guidelines `**/*_test.go`: "Table-driven tests with subtests (`t.Run`) as the default pattern", "Use `t.Parallel()` for independent subtests", and "MUST use `t.Run(\"Should...\")` pattern for ALL test cases".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/sync_transport_service_test.go` around lines 13 - 103, Split
the two monolithic tests into named subtests using t.Run("Should...") and
table-driven cases where appropriate: refactor
TestSyncTransportServiceResolvesTargetsAndUnavailableBranches so each scenario
(Sync by path, Sync by workspace, missing target validation, nil service
unavailable, ResolveOrRegister+resolveWorkspaceReference by id and path,
resolveWorkspaceReference nil DB) is a separate t.Run("Should ...") subtest and
call t.Parallel() inside independent subtests; keep setup calls
(newRunManagerTestEnv, newTransportSyncService, service.Sync,
resolveWorkspaceReference, env.globalDB.ResolveOrRegister) local to each subtest
or recreated in a shared setup helper to avoid races. Do the same for
TestLooksLikeWorkflowDir: make each case (empty dir, workflow dir with
task_01.md, reviews-* excluded) a t.Run("Should ...") subtest (table-driven for
file scenarios) and use t.Parallel() for independent cases; reference
looksLikeWorkflowDir, os.WriteFile, filepath.Join and t.TempDir when moving
assertions into subtests. Ensure test names follow "Should..." pattern and
preserve existing assertions and error messages.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:5d7db1b2-73a3-4278-ab63-78e2b034dbdc -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - The sync transport tests currently bundle multiple behavior branches into two monolithic functions, making failures harder to localize and leaving them out of the repo’s default subtest pattern.
  - Root cause: independent scenarios were implemented as a linear script instead of isolated `Should...` subtests with local setup.
  - Intended fix: split the scenarios into named subtests and keep setup per case so the tests remain race-safe and readable.

## Resolution

- Split the sync transport coverage into named `Should ...` subtests and adjusted the layout so environment-dependent setup remains race-safe.
- Verified with `make verify`.
