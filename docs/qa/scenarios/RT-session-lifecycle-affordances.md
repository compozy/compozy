---
id: RT-session-lifecycle-affordances
area: RT
title: Preserve session identity and unresolved file-mutation evidence
persona: Théo
journey: J-11
expected: An unnamed user session receives one durable generated title after its first persisted assistant response; explicit names remain unchanged. A failed edit with no later successful edit for the same path adds one verifier marker to the durable timeline; a later successful edit suppresses it. Durable interrupt and steer replacement behavior is owned by RT-019.
entry_points: Web session list and timeline; config CLI/native tools; session metadata and event history
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-rt-current-source-20260730-20260730-061631-252740-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: RT-session-context-rebuild; RT-pressure-context-compaction
---

Create unnamed and explicitly named user sessions, complete two turns, and compare session metadata
with HTTP, UDS, CLI, and Web catalogs. Emit failed edit events with and without a later successful
mutation for the same path and inspect the durable Web timeline and raw session events.

QA impact 2026-07-15: new automatic title and file-mutation verifier behavior.
Planning flag only; no QA session ran in this implementation slice.

Phase C planning 2026-07-19: persona normalized to Théo; companion to US-004 (EC-6: auto-title and
verifier markers). Forensic contract (SD-006): timestamped commands with observed output for one
title spawn after the first assistant response (and none after the second), and the verifier marker
present without a later successful edit and absent with one — plus `eng-ui-screenshot` captures for
the title and marker.

QA impact 2026-08-03: removed obsolete interrupt-salvage expectations. RT-019 now owns durable,
fenced interrupt and steer replacements; this scenario retains only title and file-verifier behavior.

src: .compozy/tasks/hermes-comparison/_user_stories.md#us-004-compaction-under-pressure-crash-safe
