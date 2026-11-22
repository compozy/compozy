---
id: ET-agent-fallback-chain-override
area: ET
title: Operator gives a packaged agent a fallback chain without forking its AGENT.md
persona: Dora
journey: J-31
expected: "`[[agents.overrides.<agent>.fallback_chain]]` in config.toml replaces the effective agent's whole fallback chain while its prompt and authored chain stay untouched. `compozy agent info <agent>` keeps the authored `fallback_chain` and adds a `Fallback Chain (config override)` table (`fallback_chain_override` in TOON); `-o json` reports `effective_runtime.fallback_chain` and `effective_runtime.sources.fallback_chain = \"config_override\"` (`agent` for an authored chain without override). A new session whose first route is refused binds on the override route, and `compozy session continue <id> --route N` resolves N against the override chain. A workspace layer that sets `fallback_chain` replaces the global chain; `fallback_chain = []` clears it. A route without `model` fails config load naming `agents.overrides.<agent>.fallback_chain[0].model`. `compozy__config_set`/`compozy__config_unset` refuse `agents.overrides.*` with `config_trust_root_forbidden`, and `compozy config set` does not accept the path."
entry_points: "[[agents.overrides.<agent>.fallback_chain]] in $COMPOZY_HOME/config.toml and <workspace>/.compozy/config.toml; compozy agent info <agent> (+ -o json, -o toon); compozy session new --agent <agent>; compozy session continue <id> --route N; GET /api/agents/{name}"
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: "Owning tests: internal/config/provider_test.go TestResolveAgentAppliesOperatorOverride (fallback_chain subtests); internal/api/core/agent_contracts_test.go TestAgentPayloadEffectiveRuntimeUsesRequestedWorkspaceConfig; internal/session/manager_start_test.go TestCreateFallbackChain."
last_report:
overlaps: ET-agent-permission-override, RT-session-fallback-chain
---

Issue #1928. In an isolated acpmock lab, pick a bundled agent without a chain (for example the
spec-cycle `reviewer`). Add a `[[agents.overrides.reviewer.fallback_chain]]` route on another seat
or provider to `$COMPOZY_HOME/config.toml`, run `compozy agent info reviewer` (human, TOON and JSON),
and confirm the override chain and its `config_override` source next to the empty authored chain.
Make the primary route refuse `session/new` and confirm `session new --agent reviewer` binds on the
override route with one `session.fallback.used` event. Run `session continue <id> --route 1` and
confirm the child binds on the override route. Add `fallback_chain = []` to the workspace
`.compozy/config.toml` and confirm the effective chain is empty.
