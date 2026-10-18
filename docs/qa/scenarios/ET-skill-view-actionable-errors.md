---
id: ET-skill-view-actionable-errors
area: ET
title: Recover from a skill resource or definition error
persona: Ada
journey: J-load-skill-in-managed-session
expected: A native skill_view failure distinguishes a missing resource from a malformed skill definition, preserves the operator-safe path and YAML location across hosted MCP, and gives the agent a specific recovery step while keeping the primary public message stable and safe
entry_points: managed session prompt; compozy__skill_view; hosted MCP tools/call; POST /api/tools/invoke
qa_status: pass
bug_ids: [678]
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-reported-issues-20260929-20260930-025651-581547-lab/qa-artifacts/qa/skill-resource-recovered.json
last_report: docs/qa/reports/2026-09-29-reported-issues.md
overlaps: ET-compozy-native-tool-invocation; ET-managed-session-skill-loading
---

Ask a managed agent to read one missing skill resource and one skill whose `SKILL.md` frontmatter is
malformed. The two failures must remain different after the hosted MCP hop: the missing resource is
permanent until the exact relative path is corrected, while the malformed definition identifies the
file and YAML location for the operator and tells the agent to repair the definition before retrying.
Public API output must retain stable reason codes and safe primary messages; operator diagnostics
remain separately marked as details rather than replacing the user-facing message.

After repairing the definition, the same native read must succeed without restarting the daemon.
The managed process must not fall back to the operator CLI or read the file directly.

For SQLite contention, keep the lab daemon and a real managed session running while a second
connection holds an immediate write transaction. Request `compozy__skill_view` with
`name="compozy"` and `file="references/terminal.md"` while session health/supervision writes
run concurrently. Release a transient lock and require the same daemon and session to complete
the resource read and persist health. Then hold the lock beyond the bounded retry policy and
require an actionable failure rather than an opaque process-health result. Release the lock and
retry the read successfully without a daemon restart.

Diagnostics must distinguish `tool_registry_resolve`, `tool_event_write`,
`skill_catalog_resolve`, and `skill_resource_load`. A `tool_event_write` failure is not proof
that the filesystem resource loader failed. Contention diagnostics include code-owned operation,
attempt count, elapsed wait, and the local transaction identity/held duration when known. A lock
held by another database handle or process is explicitly unknown; never infer a historical owner
from temporal overlap. Logs must not include SQL arguments, skill contents, credentials, or raw
backend error text. Waiting local writers must leave pool capacity for the active transaction's
commit-fence read.

The concurrent ACP subprocess/SQLite integration is covered by
`TestHarnessContextIntegrationMeasuresDeliveredSkillCatalogs/Should_load_a_managed_skill_resource_while_session_health_writes_recover_from_contention`;
`TestExecuteWrite` owns pool starvation, bounded persistent external busy, safe writer provenance,
and recovery on the same handle. Live-provider hosted-tool evidence: `skill-resource-real.json` records two bounded
`tool_event_write` failures during a 20-second external writer;
`skill-resource-recovered.json` records the same daemon/session retry succeeding with
`# Terminal` after release. `writer-transient.jsonl` records the isolated writer interval.
This validates failure within the managed caller bound and same-daemon recovery; it does not
assert exhaustion of all 15 default helper attempts or a filesystem loader failure.

QA impact 2026-10-06: a deadline arriving during a successful SQLite COMMIT must not turn
its acknowledgement into a retryable error or duplicate the completed tool event. Cancellation
observed before commit must still roll back. TestExecuteWrite owns this transaction boundary;
TestDaemonToolEventSink retains the real writer-contention and exactly-one-event assertions.
Focused and runtime re-walk evidence: `docs/qa/reports/2026-10-05-dependency-upgrades.md`.

QA result 2026-10-06: the owning managed-skill contention integration passed with a real ACP
subprocess and SQLite under the race detector (5.869s). The unchanged tool-event contention
suite passed 30 repetitions each on macOS and Linux; transaction cancellation regressions
passed. Broader release delivery remains tracked in the dependency-upgrade report.
