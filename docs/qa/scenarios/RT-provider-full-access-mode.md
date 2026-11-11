---
id: RT-provider-full-access-mode
area: RT
title: Full access runs the ACP provider in its unrestricted mode
persona: Ada
journey: J-15
expected: With effective approve-all permissions alone, new Codex sessions select agent-full-access, new Claude Code sessions select bypassPermissions, agy selects yolo, and Copilot selects allow_all=on; under auto, Codex keeps its reviewer mode. An explicit narrower agent mode wins; narrower session permissions drop inherited unrestricted agent modes during start and later runtime changes while explicit unrestricted session selections are refused; providers without a mapping keep their own approve-all negotiation. The deprecated permissions.provider_full_access key loads with a warning and has no effect.
entry_points: config.toml permissions; compozy agent info; compozy session prompt; Loop run-agent
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits: issue #1473
evidence: internal/config/provider_test.go;internal/config/provider_agy_test.go;internal/acp/types_test.go;internal/session/manager_start_test.go;internal/session/manager_transition_test.go;internal/daemon/loop_runtime_adapters_test.go;docs/qa/reports/2026-11-10-issue-1473-full-access.md
last_report: docs/qa/reports/2026-11-10-issue-1473-full-access.md
overlaps: RT-cursor-agent-mode
---

In an owned disposable workspace, set `[permissions] mode = "approve-all"` with no other permission
key. Inspect fresh Codex and Claude Code agents without authored ACP mode options; their effective
options must select `agent-full-access` and `bypassPermissions`, respectively, and an agent at
`auto` must report no unrestricted option. Start a Codex session at `approve-all` and one at `auto`:
the first runs without the Codex sandbox, the second runs in the Codex reviewer mode with its
sandbox and `permission_mode_degraded: false`. Confirm that an agent's explicit narrower mode wins,
that `approve-reads` drops an inherited unrestricted agent mode, and that an explicit unrestricted
session selection below `approve-all` is refused. Add `provider_full_access = false` to the global
config: it loads with one deprecation warning naming `permissions.mode`, and `approve-all` still
selects the unrestricted mode. Stop the sessions.

2026-11-10 live evidence (#1473): with only `mode = "approve-all"` plus a deprecated
`provider_full_access = false`, a fresh Codex session reported `agent-full-access` and its shell had
no Codex sandbox variables; an `auto` session kept the `agent` reviewer mode (`CODEX_SANDBOX=seatbelt`,
network disabled, not degraded). A `read-only` agent kept its mode, and an explicit
`mode=agent-full-access` on the `auto` session was refused. See the dated report.

Historical 2026-09-24 evidence (opt-in era, before #1473): both Codex and Claude Code sessions reached the owned Docker container
and PostgreSQL fixture on the provider-neutral build. Narrower agent and session modes, rejection
of an explicit unrestricted override, and unsupported-provider failure were verified through the
CLI. See the dated run report and its isolated lab receipts.

## agy prefers yolo under approve-all

With effective `approve-all`, a new `agy` session
prefers mode `yolo`; an explicit narrower agent mode wins, and narrower session permissions reject
`yolo`. `agy` is never added to generic automatic mode candidates. Unit ownership is UT-004
(config negotiation cases); the live `agy` walk is unexecuted and this scenario remains `pass`
only on its previously verified Codex/Claude branches.

## Providers without a unique mode option

Under `approve-all`, start providers that omit `mode` or advertise it more than once. Startup must
skip the unrestricted-mode config selection and preserve the negotiated mode; model and reasoning
negotiation and the first prompt must still complete. A unique `mode` option must still select and
confirm the unrestricted value, and an unsupported value must fail. Change the model and reasoning
on the same live process: the unchanged skipped mode must not prevent the change. A new explicit
live mode, other options, and narrower modes still require unique advertisements. Loop applied runtime and provenance
must omit skipped options while the session retains requested intent. Triage must refuse ambiguous
mode options when no independently advertised read-only session mode is available.
Rotate a Goal that requested both a skipped mode and a confirmed option: rebinding must preserve
the full requested creation profile while its applied projection keeps only confirmed options.

Regression ownership: `TestStartUnrestrictedModeAdvertisement` in the ACP startup contract suite.
Real daemon/process-boundary coverage: `TestAgyACPBuiltInEntry`,
`TestDaemonE2EProviderReasoningNegotiatesThroughAdvertisedACPOptions`, and
`TestLoopRuntimeSelectionIntegration` (integration tag). These checks use disposable homes and
subprocess ACP providers; they do not establish live-provider or new browser evidence.
