---
id: ET-compozy-native-tool-invocation
area: ET
title: Invoke native tools through the Compozy namespaces
persona: Ada
journey: J-answer-agent-requests
expected: A managed session and operator CLI discover and invoke compozy__ native tools, hosted MCP advertises only compozy_host__ façade names from compozy-hosted-tools, and the equivalent compozy__ and agh_host__ identifiers resolve as unknown without aliases.
entry_points: compozy tool list|search|info|invoke -o json; managed session tool call; compozy mcp serve; hosted MCP tools/list and tools/call
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-native-tool-approval-grants;ET-workspace-host-api-mcp
---

QA impact 2026-07-26: native ToolIDs, ToolsetIDs, and the hosted MCP façade now
use the Compozy namespaces. Planning flag only; the next QA cycle owns real
managed-session invocation plus explicit legacy-identifier rejection.
