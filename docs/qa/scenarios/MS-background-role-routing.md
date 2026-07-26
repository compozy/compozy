---
id: MS-background-role-routing
area: MS
title: Route background work by global and workspace role
persona: Dora
journey: J-route-background-work
expected: New coordinator, dream, extractor, auto-title, checkpoint-summary, and memory-controller work resolves the configured global or workspace identity and model without changing role policy or leaking across workspaces.
entry_points: config.toml; agh config set roles.<role>.<key> <value>; agh config set roles '<table-json>'; agh config set --scope workspace --workspace <root> roles '<table-json>'; agh__config_list|get|set|unset over exact roles.* leaves or the structured roles table (agh__config_path proves the selected scope target only); docs runtime/core/configuration/config-toml [roles]
qa_status: untested
bug_ids: BUG-20260724-coordinator-config-list-path;BUG-20260724-inherited-role-provider-resolution
fix_status: fixed
retest_status: pass
fix_commits: 69b2099f3cada66395ced4c8ae862b21b5ebc996;a9a8fcad63f4354505e4c9a0701a6d0f559cc991
evidence: /Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/config-list-coordinator-after-fix.json; /Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/config-get-coordinator-enabled-after-fix.json; /Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/native-tools-session-2-history.json; /Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/inherit-provider-fix-auto-title-child.json
last_report: docs/qa/reports/2026-07-24-agent-roles.md
overlaps: MS-026; RT-033; RT-session-auto-title; MS-workspace-checkpoint-continuity
---

QA impact 2026-07-23: the six daemon-owned background roles moved to the live `[roles]` routing model with global defaults, workspace overlays, builtin identities, and strict rejection of the deleted role-bearing keys. Planning update only; the next QA cycle owns the real-user walk.

Planning 2026-07-24 (Task 05): persona reconciled Bruno → Dora — role routing is runtime administration (config keys, daemon ownership), which is Dora's definition. Entry points widened to the full write plane: workspace-scoped `agh config set`, the native `agh__config_list|get|set|unset` tools over exact `roles.*` leaves (removed paths must reject deterministically; `agh__config_path` only proves the selected global/workspace config file target and scope), and the docs config reference as the entry origin real users start from. Session charter: CH-background-role-routing-scopes.

QA 2026-07-24: global/workspace overlays, strict removed-key rejection, bounded validation, native config lifecycle, ghost diagnostics, hidden builtin sessions, and live invocation all passed. Two contained defects were fixed and retested: the flattened coordinator config path and model-only inherited-provider resolution.

QA impact 2026-07-24 (final review remediation): CLI and native config mutation now accept the
complete structured `[roles]` table at global and workspace scope, including fallback chains. The
next QA cycle owns this broader agent-manageable write path.

QA impact 2026-07-25 (Roles panel redesign): routing is still configured through
`PATCH /api/settings/roles`, but the Web path now writes provider, model and reasoning effort
together from one `RuntimeSelector` (a model pick pins both provider and model) with a Clear
override action for inherit. Split provider-only / model-only overrides remain expressible through
`config.toml`, the CLI, and the API. The next QA cycle owns Web-versus-CLI parity for those partial
routes.

QA impact 2026-07-25 (deep-review remediation): disabled roles now return a disabled routing
decision before catalog or provider resolution, structured JSON config writes preserve integer
values, and direct ACP roles accept an empty model. Flag only; the next QA cycle owns live
global/workspace and CLI/native-tool parity retesting.
