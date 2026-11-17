---
id: RT-provider-full-access-mode
area: RT
title: Full access runs the ACP provider in its unrestricted mode
persona: Ada
journey: J-15
expected: With effective approve-all permissions alone, new Codex sessions select agent-full-access, new Claude Code sessions select bypassPermissions, agy negotiates its advertised unrestricted mode (auto on the verified CLI, never an injected yolo), and Copilot selects allow_all=on; under auto, Codex keeps its reviewer mode. An explicit narrower agent mode wins; narrower session permissions drop inherited unrestricted agent modes during start and later runtime changes while explicit unrestricted session selections are refused; providers without a mapping keep their own approve-all negotiation. Native agy acp refuses all narrower host policies before launch without widening them. A config that still carries the retired permissions.provider_full_access key starts normally: the key is ignored, removed from writable config files after a backup, and permissions.mode remains the canonical control.
entry_points: config.toml permissions; compozy agent info; compozy session prompt; Loop run-agent
qa_status: blocked-verify
bug_ids:
fix_status: fixed
retest_status: blocked-verify
fix_commits: issue #1473
evidence: internal/config/provider_test.go;internal/config/provider_agy_test.go;internal/acp/types_test.go;internal/session/manager_start_test.go;internal/session/manager_transition_test.go;internal/daemon/loop_runtime_adapters_test.go;docs/qa/reports/2026-11-10-issue-1473-full-access.md
last_report: docs/qa/reports/2026-11-17-agy-native-acp-consented.md
overlaps: RT-cursor-agent-mode
---

In an owned disposable workspace, set `[permissions] mode = "approve-all"` with no other permission
key. Inspect fresh Codex and Claude Code agents without authored ACP mode options; their effective
options must select `agent-full-access` and `bypassPermissions`, respectively, and an agent at
`auto` must report no unrestricted option. Start a Codex session at `approve-all` and one at `auto`:
the first runs without the Codex sandbox, the second runs in the Codex reviewer mode with its
sandbox and `permission_mode_degraded: false`. Confirm that an agent's explicit narrower mode wins,
that `approve-reads` drops an inherited unrestricted agent mode, and that an explicit unrestricted
session selection below `approve-all` is refused. Stop the sessions.

Upgrade from v0.6.0 (#1618): in a disposable home, write a global `config.toml` with comments,
`[permissions] mode = "approve-all"` and `provider_full_access = false`, as a v0.6.0 home may hold.
Start the runtime: it must become ready without manual edits, `approve-all` must still select the
unrestricted mode, the file must lose only the retired line, `config.toml.retired-keys.bak` must hold
the original bytes, and the daemon log must name the removed key once. Restart and confirm nothing
changes. Repeat with `true`, with the key in a workspace `.compozy/config.toml`, and with the global
config directory read-only: the runtime must still start and leave that file unchanged.
`TestLoadUpgradesRetiredProviderFullAccessKey` owns the config-layer invariant in CI; the live
desktop upgrade walk is unexecuted.

Historical 2026-11-10 live evidence (#1473, before v0.6.1 key removal): with only `mode = "approve-all"` plus a deprecated
`provider_full_access = false`, a fresh Codex session reported `agent-full-access` and its shell had
no Codex sandbox variables; an `auto` session kept the `agent` reviewer mode (`CODEX_SANDBOX=seatbelt`,
network disabled, not degraded). A `read-only` agent kept its mode, and an explicit
`mode=agent-full-access` on the `auto` session was refused. See the dated report.

Historical 2026-09-24 evidence (opt-in era, before #1473): both Codex and Claude Code sessions reached the owned Docker container
and PostgreSQL fixture on the provider-neutral build. Narrower agent and session modes, rejection
of an explicit unrestricted override, and unsupported-provider failure were verified through the
CLI. See the dated run report and its isolated lab receipts.

## agy negotiates its advertised mode under approve-all

With effective `approve-all`, a new `agy` session selects a mode the CLI actually advertises: the
verified 1.5.9 runtime lists `default`, `accept-edits`, `auto`, and `plan`, so approve-all negotiates
`auto` and no `yolo` selection or permission-bypass launch argument is injected. An explicit narrower
agent mode wins under that host policy. Native `agy acp` refuses all narrower host policies before
launch because live default and acknowledged plan both bypassed host write approval. Authored
standalone/custom commands retain their own approval behavior. `agy` is never added to generic
automatic mode candidates. Unit ownership is UT-002; `TestAgyACPBuiltInEntry` covers effective-command
refusal and retained legacy metadata/transcript through the real ACP driver.

The 2026-11-16 live walk observed `auto`, an allowed write and hosted native read, then final HTTP 422
refusal with an absent fresh target and unchanged `approve-reads`. Turn cancellation and explicit
detached-task termination were verified as separate operations. See
`docs/qa/reports/2026-11-16-agy-native-acp.md`; its raw lab evidence was lost on environment restart
before the strict audit/teardown. These observations do not establish a full scenario pass.
The historical Codex/Claude branches remain previously verified; retired-key upgrade is untested.

QA 2026-11-17 (AGY-only consented re-walk): independent durable receipts confirm native
`approve-all` acknowledged `auto`, completed an authorized write and hosted workspace-info read,
and subsequently remained usable after canceling observed active work. A fresh `approve-reads`
session refused before launch with unchanged permissions, no processes, and no forbidden file.
This is refusal, not a claim that `plan` enforces read-only behavior. Explicit cleanup found no
remaining task. See the current report and `native-authority-cancellation-verdict.json`.
No Codex/Claude/retired-key or whole-scenario PASS is claimed by this targeted run.

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

ACP provenance regression coverage (#1520, #1526): in the existing run-agent action suite,
request `fast_mode=false` and let binding add `mode=agent-full-access`; the applied sources
must be exactly `fast_mode=node` and `mode=agent`. Then request both options and skip
`mode`: only `fast_mode=node` survives. Skipping every option must omit option provenance.
Requested intent remains unchanged. `TestReservedActionExecutorsShouldRunAgentLoopAndTransform`
owns this invariant; `TestLoopRuntimeSelectionIntegration` owns persisted/read projections
and `runtime_applied` replay. Historical events remain unchanged.

Focused verification 2026-11-11: action/precedence regressions and scoped Loop lint pass;
`go test -tags=integration ./internal/daemon -run '^TestLoopRuntimeSelectionIntegration$' -count=1`
passes with an isolated daemon, SQLite and subprocess ACP peer, including restart/read/replay.
This is automated integration evidence, not a new live-provider or browser walk.
