---
id: ET-compozy-native-tool-invocation
area: ET
title: Invoke native tools through the Compozy namespaces
persona: Ada
journey: J-validate-compozy-hard-cut
expected: A managed session, including Codex on a macOS CGO-disabled build, and the operator CLI discover and invoke compozy__ native tools; hosted MCP advertises only compozy_host__ façade names from compozy-hosted-tools, and retired identifiers resolve as unknown without aliases.
entry_points: compozy tool list|search|info|invoke -o json; managed session tool call; compozy mcp serve; hosted MCP tools/list and tools/call
qa_status: pass
bug_ids: BUG-20260727-runtime-legacy-identity;BUG-20260910-idle-session-high-cpu
fix_status: fixed
retest_status: pass
fix_commits: e4df8634;ed2f523b8c5e85b54f0be70accf81ff011255ff3;4368ae4a6078fe16f50a528a687a70726fba9a61;0308fe9617f08ce8c6de8ace9ffca4659ee00b6f;b75ab2c2bb54ba7a7ce5b6c6cafe73a6b0b337a4;e016417aac4fbef9f79a025743b54d98b57b55dd;3b5a22ee3287316afd213bad3afc7dd6e0a2f852;66100544f88d6ce9b0c2b5a8affec78a2a2fb9c9
evidence: /Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/tool-info-skill-list.json;/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/tool-skill-list.json;/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/tool-skill-view.json;/Users/pedronauck/dev/qa-labs/compozy-beta24-cpu-20260910-162426-604899-lab/qa-artifacts/qa/logs/final-policy-go1264-clean-state-canary-canary.json;/Users/pedronauck/dev/qa-labs/compozy-beta24-cpu-20260910-162426-604899-lab/qa-artifacts/qa/logs/final-policy-go1264-clean-state-representative-metrics.json;/Users/pedronauck/dev/qa-labs/compozy-beta24-cpu-20260910-162426-604899-lab/qa-artifacts/qa/qa-audit-report.json;/Users/pedronauck/dev/qa-labs/compozy-beta24-cpu-20260910-162426-604899-lab/qa-artifacts/qa/teardown.json;/Users/pedronauck/dev/qa-labs/compozy-beta24-cpu-review-20260910-181815-323329-lab/qa-artifacts/qa/logs/watcher-final-clean-state-canary-canary.json;/Users/pedronauck/dev/qa-labs/compozy-beta24-cpu-review-20260910-181815-323329-lab/qa-artifacts/qa/logs/watcher-final-clean-state-representative-metrics.json
last_report: docs/qa/reports/2026-09-10-beta24-daemon-cpu.md
overlaps: ET-native-tool-approval-grants; ET-workspace-host-api-mcp
---

Passed in the 2026-07-30 final rerun: a real Codex-managed task-role session bound nine hosted
native descriptors in 4 ms, claimed run `run-d24b7bddfc76b2d4`, maintained its lease with two
heartbeats, and completed it through `compozy__task_run_complete`. The resulting Go canary service
passed three tests; Python and shell verification also passed. This directly exercises managed
session calls through `compozy-hosted-tools` without a CLI lease substitute.

QA impact 2026-07-26: native ToolIDs, ToolsetIDs, and the hosted MCP façade now
use the Compozy namespaces. Planning flag only; the next QA cycle owns real
managed-session invocation plus explicit legacy-identifier rejection.

QA impact 2026-08-13: reset because `compozy__config_get` now distinguishes an absent key with `config_path_not_found`, and extension-agent sessions share one skill resolver across native calls.

QA evidence correction 2026-08-13: the prior pass is not valid evidence for PR #372 because its build predates this PR head. It is historical only and does not set this scenario status.

QA verdict 2026-08-13 (fresh native-CLI lab): passed. The real operator-home Codex reviewer invoked hosted skill-list, empty skill-search, all ten skill views, and `compozy__config_get`; the missing `loops.inputs.batuta-deliver.auto_commit` path returned `config_path_not_found`. This is a substantive persona-walk verdict only: the QA report remains blocked on C14 until a successful final gate exists.

QA impact 2026-08-25 (skill sources): reset because the skill native-tool descriptors changed in this cycle. `compozy__skill_list` and `compozy__skill_search` now require `origin` and `owner_scope`, `compozy__skill_view` additionally requires `exposures[]`, the descriptions were rewritten, and the recorded native-tool catalog and schema digests were regenerated. Re-walk discovery and invocation across the managed session, the operator CLI, and hosted MCP against the new digests. Charter: `CH-skill-sources-agent-plane`.

QA impact 2026-09-10 (daemon CPU): keep two hosted MCP clients connected to a
workspace with skills and extensions. Compare catalog output, repeated discovery,
and daemon CPU before and after the fix. Native availability and policy must
remain live; extension workspace aliases must resolve to the same registered
identity, and invalid identity or missing roots must remain rejected. Use
`docs/qa/reports/2026-09-10-beta24-daemon-cpu.md` for this bounded performance pass;
it does not replace the historical provider-specific journeys above.

QA verdict 2026-09-10: passed. Two real hosted MCP clients on the Go 1.26.4,
CGO-disabled production build retained the exact 237-tool catalog. Config and
extension changes updated both connected clients, nested skill changes converged
within the existing watcher contract, native calls and UDS succeeded, and daemon
CPU fell from 186.867% to 21.157% on the controlled representative fixture.
The final canary also covers directory membership/type changes and in-place
invalid-to-valid and valid-to-invalid definition edits with preserved metadata.
The original local gate and strict QA audit passed; final delivery gates run
exclusively in CI at the user's request. The owning report records both lab runs.

QA impact 2026-10-09 (Subagents tools slice): verify the four `compozy__subagent_*` tools in the
`sessions` toolset, active-turn rejection, narrowing-only budgets, and status acknowledgement.
Scoped evidence: `TestNativeSubagentBindings`, `TestNativeSubagentPermissionBoundary`, and
`TestHostedSubagentAnnotations`. The integrated provider-backed walk remains owned by the spec QA slice.

QA impact and focused verification 2026-10-23 (harness dogfood): call `monitor_create` with
an overlong description through hosted MCP. Expect stable `tool_invalid_input` / `schema_invalid`
and safe `operator_cause` / `operator_recovery` naming description and its 200-character bound,
without echoing its submitted value. Missing required schema fields likewise identify their
declared path; arbitrary unknown keys and backend details stay masked. Existing native validation,
shared transport, and real hosted-stdio daemon tests pass. Separately, the inline skill router
requires Task guidance only when exercising daemon Task authority: ordinary delegated provider
coding/review work can continue without unavailable native manuals, while capability bypass
remains forbidden. Static instruction review is not a model-compliance score. See
[focused verification](../reports/2026-10-23-harness-dogfood.md).

## Native error parity — 2026-10-28

Native error parity (#1012): exercise archive/worktree refusals through both the CLI and native gateway. Registered domain codes must survive generic adapter wrapping, existing specialized reasons/partial results must remain compatible, and backend failures must retain a bounded sanitized cause. Do not infer a transport outage from `tool_backend_failed` alone.

The [targeted report](../reports/2026-10-28-native-tool-errors.md) records the real isolated-daemon re-walk, automated evidence, and limits. Broader scenario status is unchanged.


QA impact 2026-10-30 (Claude hosted MCP startup): verify both a fresh Claude ACP
session and a resumed session. Capture only redacted `mcpServers` shape: stdio has
no `type` discriminator and preserves the executable, arguments, and environment.
Require an established hosted bind and a successful native tool invocation on each
runtime. A controlled missing executable must fail the later `mcp_bind` startup
stage before native-tool guidance is delivered; `mcp_activation` alone only arms
the launch. Never restart the operator daemon for this walk. WSL2 ENOENT behavior
requires separate platform evidence when the local lab is macOS.

The [targeted report](../reports/2026-10-30-claude-hosted-mcp-startup.md) records fresh/load payload evidence, a controlled bind failure, and the unreproduced WSL2 limit.

QA impact 2026-11-01 (responsiveness audit): a toolset-list request now discovers
the scoped tool universe once and expands every toolset against that snapshot.
Re-walk toolset discovery with multiple toolsets and dynamic MCP tools, change the
available tools, and repeat discovery. All toolsets in each response must agree;
the later request must reflect the changed catalog while preserving workspace,
profile, authorization, and unknown-tool diagnostics.

Focused evidence: `TestRuntimeRegistryToolsets` and the canonical registry tests
pass with the race detector. The
[performance report](../reports/2026-11-01-performance/events-settings.md) records
before/after registry measurements without synthetic network delay. Real
CLI/HTTP/hosted-MCP verification and current-head CI remain pending for this
audit; this entry does not change historical passing verdicts above.
