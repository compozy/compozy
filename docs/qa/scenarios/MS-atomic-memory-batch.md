---
id: MS-atomic-memory-batch
area: MS
title: Commit one agent memory batch without intermediate state
persona: Ada
journey: J-11
expected: An agent can submit ordered add, replace, and remove operations for one workspace memory document through agh__memory_propose. AGH rejects missing or ambiguous old_text without changing bytes, validates only the final configured body size, records one decision for a successful batch, keeps repeated prompt assembly byte-stable until a committed memory mutation, and recalls the committed fact in the next session.
entry_points: hosted MCP agh__memory_propose; workspace Memory v2 files; memory decision history; next-session prompt recall
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-workspace-checkpoint-continuity
---

Start an agent session with hosted native tools and submit a workspace-scoped batch containing at
least two ordered operations. Confirm no intermediate document bytes are visible, one decision is
recorded, and an identical retry reports `already_applied`. Repeat with a missing and a duplicated
`old_text`; confirm the document and decision count stay unchanged. Fill a document to its
configured limit, then remove stale text and add a replacement in one batch; confirm final-state
validation permits it. Compare three assembled prefix hashes before and after one committed write,
then start another session and confirm recall contains the committed fact.

QA impact 2026-07-15: new agent-visible native batch shape, final-body limit enforcement, and
memory-prefix generation behavior. Planning flag only; no QA session ran in this implementation
slice.

Phase C planning 2026-07-19: persona normalized to Ada; companion to US-004 (EC-4 atomic batches,
EC-5 prefix stability). Forensic contract (SD-006): timestamped command + observed output for the
atomic batch (zero applied on injected mid-batch failure), the ambiguous `old_text` rejection, and
the three-turn prefix-hash trace changing exactly once after one committed write.

src: .compozy/tasks/hermes-comparison/_user_stories.md#us-004-compaction-under-pressure-crash-safe
