---
id: MS-reject-operational-memory-state
area: MS
title: Reject operational state from durable memory
persona: Ada
journey: J-store-durable-memory-safely
expected: Operational Memory v2 identifiers return a rejected controller decision without creating a file, while a nearby durable write persists and survives a fresh read.
entry_points: compozy memory write -o json; POST /api/memory; compozy memory list -o json; compozy memory show
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-memory-operational-state-20260814-225633-763701-lab/qa-artifacts/qa/memory-operational-state-session.md
last_report: docs/qa/reports/2026-08-14-pr-396-memory-safety.md
overlaps: MS-003
---

The targeted walk covers one qualified native tool identifier, one dotted operation/event identifier, and the Memory policy namespace through public write surfaces. The controller unit suite remains the canonical owner of collision identity; the HTTP walk also exercises its documented origin field as production-like confirmation.

QA 2026-08-14: CLI and HTTP rejected operational identifiers without creating a file. The HTTP surface also exercised all three autonomous origins through its documented origin field: generated collisions returned noop, while an explicit target filename updated the intended file. Fresh list/show reads confirmed only the two intended safe files.
