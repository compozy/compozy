---
id: ET-compozy-native-tool-invocation
area: ET
title: Invoke native tools through the Compozy namespaces
persona: Ada
journey: J-validate-compozy-hard-cut
expected: A managed session, including Codex on a macOS CGO-disabled build, and the operator CLI discover and invoke compozy__ native tools; hosted MCP advertises only compozy_host__ façade names from compozy-hosted-tools, and retired identifiers resolve as unknown without aliases.
entry_points: compozy tool list|search|info|invoke -o json; managed session tool call; compozy mcp serve; hosted MCP tools/list and tools/call
qa_status: pass
bug_ids: BUG-20260727-runtime-legacy-identity
fix_status: fixed
retest_status: pass
fix_commits: e4df8634
evidence: /home/franciscpd/dev/qa-labs/compozy-extension-agent-session-skills-20260813-122950-240954-lab/qa-artifacts/qa/provider-hosted-mcp-summary.json;/home/franciscpd/dev/qa-labs/compozy-extension-agent-session-skills-20260813-122950-240954-lab/qa-artifacts/qa/native-config-get-missing.json;/home/franciscpd/dev/qa-labs/compozy-extension-agent-session-skills-20260813-122950-240954-lab/qa-artifacts/qa/native-config-set.json;/home/franciscpd/dev/qa-labs/compozy-extension-agent-session-skills-20260813-122950-240954-lab/qa-artifacts/qa/native-config-get-reread.json;/home/franciscpd/dev/qa-labs/compozy-extension-agent-session-skills-20260813-122950-240954-lab/qa-artifacts/qa/operator-config-reread-after-provider.json
last_report: docs/qa/reports/2026-08-13-extension-agent-session-skills.md
overlaps: ET-native-tool-approval-grants;ET-workspace-host-api-mcp
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

QA verdict 2026-08-13: passed. The live extension-agent Codex session invoked `compozy__command_list`, all three skill tools, and `compozy__config_get/set` through hosted MCP. An absent `loops.inputs.batuta-deliver.auto_commit` returned `config_path_not_found`; the session persisted workspace `false`, reread `false`, preserved `todo 1.0.0` literally, and did not modify repository files.
