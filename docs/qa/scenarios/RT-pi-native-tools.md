---
id: RT-pi-native-tools
area: RT
title: Pi sessions discover native tools and receive monitor wakes
persona: Ada
journey: J-15
expected: Pi ACP sessions discover and call session-bound Compozy tools; monitor creation wakes the same session. A failed bridge emits a typed runtime warning and CLI guidance without claiming native tools are available.
entry_points: compozy session new; compozy session prompt; compozy session history
qa_status: untested
bug_ids:
fix_status: pending
retest_status: pending
fix_commits:
evidence: internal/session/provider_runtime_test.go;internal/acp/client_start_contract_test.go;internal/daemon/composed_assembler_test.go
last_report: docs/qa/reports/2026-10-29-issue-1020-pi-native-tools.md
overlaps: RT-provider-full-access-mode
---

In an isolated home, configure a real `pi_acp` provider and a model with tool support.
Use both `native_cli` and `bound_secret` routes. Preserve the operator's pi settings
and native login; do not copy secrets into evidence.

Ask the agent to inspect its current session and run a background terminal command,
then notify it through a daemon monitor. Verify discovery and invocation of
`compozy__tool_search`, `compozy__session_status`, `compozy__terminal_exec`, and
`compozy__monitor_create` in persisted history. After the first turn ends, confirm a
new monitor-origin turn in the same session and the command result.

Repeat with a pi executable that cannot load the bridge. Confirm a typed
`pi_native_tools_*` warning and startup CLI guidance without the native-tool router
or native skill-reader instructions. Existing Claude/Codex launch behavior remains
covered by the provider runtime and ACP suites.
