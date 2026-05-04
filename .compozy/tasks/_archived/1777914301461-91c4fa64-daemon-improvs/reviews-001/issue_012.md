---
status: resolved
file: internal/cli/daemon_commands_test.go
line: 1087
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148016854,nitpick_hash:25ff6de015c9
review_hash: 25ff6de015c9
source_review_id: "4148016854"
source_review_submitted_at: "2026-04-21T13:29:50Z"
---

# Issue 012: Missing t.Parallel() for independent test.
## Review Comment

Per coding guidelines, independent tests should use `t.Parallel()` for parallel execution.

---

## Triage

- Decision: `invalid`
- Root cause analysis: `TestDaemonStartCommandInternalChildUsesDetachedRunMode` is not an isolated pure function test. It acquires the suite-wide CLI override lock and mutates the package-global `runCLIDaemonForeground` hook.
- Why the finding does not apply: adding `t.Parallel()` here would let this test contend with other daemon-command cases that intentionally serialize global CLI overrides. The current non-parallel execution is required to keep those global mutations deterministic.
- Resolution: no code change. The test should remain serialized.
