---
id: ET-agent-permission-override
area: ET
title: Operator overrides a bundled agent's permission level without forking its prompt
persona: Dora
journey: J-31
expected: Setting `[agents.overrides.<agent>].permissions` in config.toml (or `compozy config set agents.overrides.<agent>.permissions <mode>` from the operator shell) changes the level a new session of that agent starts at, while the agent keeps its packaged prompt, appearance and other fields. `compozy agent info <agent>` shows the authored `Permissions` unchanged and `Effective permissions: <mode> (config_override)`; `-o json` reports `effective_runtime.permissions` and `effective_runtime.sources.permissions = "config_override"`. A new session reports `effective_permissions` at the override level. Removing the override returns new sessions to the authored level (`(agent)` source). A workspace `.compozy/config.toml` override merges per field over the global one. From an agent session, `compozy config set|unset agents.overrides.*` fails with `agent_override_operator_only`, and `compozy__config_set`/`compozy__config_unset` refuse the path with `config_trust_root_forbidden`. An invalid level fails config load naming `agents.overrides.<agent>.permissions`.
entry_points: "[agents.overrides.<agent>] in $COMPOZY_HOME/config.toml and <workspace>/.compozy/config.toml; compozy config set|unset agents.overrides.<agent>.<field>; compozy agent info <agent> (+ -o json); compozy session new --agent <agent>; compozy__config_set"
qa_status: partial
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: "2026-11-04 isolated acpmock lab (COMPOZY_HOME=/tmp/aov/home, providers.claude routed to acpmock-driver, issue #1265 branch): bundled `orchestrator` with authored `permissions: auto`; override approve-all → agent info `Effective permissions: approve-all (config_override)` with the packaged prompt; `session new --agent orchestrator` → `effective_permissions: approve-all`; prompt completed (`override lab ok`, end_turn); override unset → new session `auto` and `auto (agent)`; agent-session `config set` → `agent_override_operator_only`. Owning tests: internal/config/provider_test.go TestResolveAgentAppliesOperatorOverride; internal/config/tool_surface_test.go TestToolConfigPathPolicy; internal/cli/config_test.go TestConfigSetAgentOverrideIsOperatorOnly; internal/api/core/agent_contracts_test.go TestAgentPayloadEffectiveRuntimeUsesRequestedWorkspaceConfig; internal/daemon/role_resolver_test.go TestRoleResolver."
last_report:
overlaps: MS-web-agent-permissions, ET-web-agents-team-profile
---

Issue #1265. In an isolated lab, pick a bundled or extension agent whose definition sets
`permissions` (for example the bundled `orchestrator`). Add
`[agents.overrides.orchestrator] permissions = "approve-all"` to `$COMPOZY_HOME/config.toml`, run
`compozy agent info orchestrator`, and confirm the authored level, the effective level with source
`config_override`, and the packaged prompt. Start a session with `--agent orchestrator`, confirm
`effective_permissions`, and send one prompt. Remove the key and confirm the next session starts at
the authored level. Add a workspace override that changes only `permissions` over a global one that
also sets `model`, and confirm both apply. From a shell with `COMPOZY_SESSION_ID` set, confirm
`compozy config set agents.overrides.orchestrator.permissions approve-all` fails with
`agent_override_operator_only`.

Not walked 2026-11-04: the workspace per-field merge and the native `compozy__config_set` refusal in
the lab (both covered by the owning config suites), an always-on instance started under the
override, and a delegation that requests more than the overridden level. The Web Permissions page
still shows the authored level, not the override (see MS-web-agent-permissions).
