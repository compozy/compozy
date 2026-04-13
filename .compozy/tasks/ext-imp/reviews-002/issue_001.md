---
status: resolved
file: internal/cli/form_test.go
line: 367
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56V4Ze,comment:PRRC_kwDORy7nkc627G2v
---

# Issue 001: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Run this overlay test serially.**

This test mutates process-global overlay registries via `agent.ActivateOverlay(...)` and `provider.ActivateOverlay(...)`, so `t.Parallel()` makes it race with any other test touching the same catalogs. That can leak temporary providers across tests and produce order-dependent failures.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/form_test.go` around lines 342 - 367, The test
TestFormSelectOptionsIncludeExtensionCatalogEntries calls t.Parallel() but
mutates global overlay registries via agent.ActivateOverlay and
provider.ActivateOverlay, causing races; remove the t.Parallel() invocation (or
convert it to a serial-only test) so the test runs serially and does not race
with other tests that touch the same catalogs, ensuring overlays restored by
restoreIDE() and restoreProvider() don't leak across tests.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:d73c2a5b-b595-4db2-9fe8-f643a58586e0 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Root cause: `TestFormSelectOptionsIncludeExtensionCatalogEntries` calls `t.Parallel()` before mutating the process-global IDE and provider overlay catalogs with `ActivateOverlay(...)`.
  - Impact: concurrent tests can observe transient overlay registrations or restores, creating order-dependent failures.
  - Fix approach: make the parent test serial so the overlay activation and deferred restore stay isolated for the whole test body.
  - Implemented: removed the parent `t.Parallel()` so the overlay catalog mutation stays serial for the duration of the test.
  - Verification: targeted Go tests passed and the final `make verify` run passed cleanly.
