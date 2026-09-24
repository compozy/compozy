---
id: RT-provider-full-access-mode
area: RT
title: Apply an explicit unrestricted ACP provider mode to trusted local sessions
persona: Ada
journey: J-15
expected: With permissions.provider_full_access enabled and effective approve-all permissions, new Codex sessions select agent-full-access and new Claude Code sessions select bypassPermissions. An explicit narrower agent mode wins; stricter final permissions refuse unrestricted modes; unsupported providers report an error.
entry_points: config.toml permissions; compozy agent info; compozy session prompt; Loop run-agent
qa_status: blocked-verify
bug_ids:
fix_status: proposed
retest_status: pending
fix_commits:
evidence: internal/config/provider_test.go;internal/acp/types_test.go;internal/daemon/loop_runtime_adapters_test.go
last_report:
overlaps: RT-session-sandbox-first-bind; RT-cursor-agent-mode
---

In an owned disposable workspace, enable `[permissions] provider_full_access = true` and
`mode = "approve-all"`. Inspect fresh Codex and Claude Code agents without authored ACP mode options;
their effective options must select `agent-full-access` and `bypassPermissions`, respectively.
Start each provider through a new session and verify a read-only command can reach an owned Docker
container and localhost PostgreSQL service. Confirm that an agent's explicit narrower mode wins,
that `approve-reads` cannot start with an unrestricted mode, and that an unsupported provider fails
clearly. Stop the sessions and remove the disposable services.

2026-09-24 partial evidence: an earlier Codex-specific build selected `agent-full-access` and its
provider-native command reached the owned Docker and PostgreSQL fixtures. The provider-neutral
configuration and Claude Code selection have focused Go coverage. A live walk of both providers on
the provider-neutral build remains pending; the existing active delivery run must not be disrupted.
