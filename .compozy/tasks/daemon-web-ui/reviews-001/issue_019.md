---
status: resolved
file: internal/daemon/transport_service_test.go
line: 195
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58gqYT,comment:PRRC_kwDORy7nkc651WIZ
---

# Issue 019: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Refactor these transport tests to table-driven `t.Run("Should...")` subtests.**

Current tests combine many branches in single functions, which makes failures harder to isolate and doesn’t match the mandated test pattern.



As per coding guidelines `**/*_test.go`: "Table-driven tests with subtests (`t.Run`) as the default pattern" and "MUST use `t.Run(\"Should...\")` pattern for ALL test cases".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/transport_service_test.go` around lines 14 - 195, The tests
TestWorkspaceTransportServiceCRUDAndUnavailableBranches,
TestTaskTransportServiceWorkflowReadsStartRunAndUnavailableBranches and
TestTransportSyncResultMapsStructuredFields bundle multiple assertions/branches
in single functions; refactor each into table-driven subtests using
t.Run("Should...") for each logical case (e.g., workspace
register/list/get/resolve/update/delete and nil-service errors in
TestWorkspaceTransportServiceCRUDAndUnavailableBranches; workflow
list/get/start/run/archive/list-after-archive and nil-db/nil-runmanager errors
in TestTaskTransportServiceWorkflowReadsStartRunAndUnavailableBranches; and
structured field checks in TestTransportSyncResultMapsStructuredFields). For
each case create a table entry with name like "Should <behavior>" and execute
the specific setup/assertion inside a t.Run closure, keeping helpers like
newTransportWorkspaceService, newTransportTaskService, transportSyncResult and
env.* calls intact but moved into each subtest's body.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:5d7db1b2-73a3-4278-ab63-78e2b034dbdc -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - The transport service tests mix multiple CRUD/start/archive/unavailable branches inside large single tests, which weakens failure isolation and diverges from the repository’s test contract.
  - Root cause: scenario coverage accumulated inside monolithic tests instead of isolated `Should...` subtests.
  - Intended fix: split the cases into table-driven subtests with local setup and preserved assertions.

## Resolution

- Refactored the transport service tests into isolated `Should ...` subtests so CRUD, workflow-read, and unavailable-branch behavior fail independently and remain readable.
- Verified with `make verify`.
