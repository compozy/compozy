---
id: RT-provider-full-access-mode
area: RT
title: Apply an explicit unrestricted ACP provider mode to trusted local sessions
persona: Ada
journey: J-15
expected: With permissions.provider_full_access enabled and effective approve-all permissions, new Codex sessions select agent-full-access and new Claude Code sessions select bypassPermissions. An explicit narrower agent mode wins; narrower session permissions drop inherited unrestricted agent modes during start and later runtime changes while explicit unrestricted session selections are refused; unsupported providers report an error.
entry_points: config.toml permissions; compozy agent info; compozy session prompt; Loop run-agent
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits:
evidence: internal/config/provider_test.go;internal/acp/types_test.go;internal/session/manager_transition_test.go;internal/daemon/loop_runtime_adapters_test.go;docs/qa/reports/2026-09-24-pr-674-provider-full-access.md
last_report: docs/qa/reports/2026-09-24-pr-674-provider-full-access.md
overlaps: RT-session-sandbox-first-bind; RT-cursor-agent-mode
---

In an owned disposable workspace, enable `[permissions] provider_full_access = true` and
`mode = "approve-all"`. Inspect fresh Codex and Claude Code agents without authored ACP mode options;
their effective options must select `agent-full-access` and `bypassPermissions`, respectively.
Start each provider through a new session and verify a read-only command can reach an owned Docker
container and localhost PostgreSQL service. Confirm that an agent's explicit narrower mode wins,
that `approve-reads` drops an inherited unrestricted agent mode and refuses an explicit unrestricted
session selection, and that an unsupported provider fails
clearly. Stop the sessions and remove the disposable services.

2026-09-24 live evidence: both Codex and Claude Code sessions reached the owned Docker container
and PostgreSQL fixture on the provider-neutral build. Narrower agent and session modes, rejection
of an explicit unrestricted override, and unsupported-provider failure were verified through the
CLI. See the dated run report and its isolated lab receipts.
