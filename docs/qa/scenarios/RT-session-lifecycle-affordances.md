---
id: RT-session-lifecycle-affordances
area: RT
title: Preserve session identity, interrupted intent, and unresolved file-mutation evidence
persona: Théo
journey: J-11
expected: An unnamed user session receives one durable generated title after its first persisted assistant response; explicit names remain unchanged. A dedicated interrupt followed by steer submits the canceled prompt plus the correction once under the new generation, while a plain interrupt replacement excludes canceled text. A failed edit with no later successful edit for the same path adds one verifier marker to the durable timeline; a later successful edit suppresses it.
entry_points: Web session list and timeline; session prompt/interrupt/steer HTTP and UDS routes; config CLI/native tools; session metadata and event history
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-session-context-rebuild; RT-pressure-context-compaction
---

Create unnamed and explicitly named user sessions, complete two turns, and compare session metadata
with HTTP, UDS, CLI, and Web catalogs. Interrupt an active prompt through the dedicated endpoint,
then steer it; inspect the next persisted user input and generation. Repeat with a plain interrupt
replacement. Finally, emit failed edit events with and without a later successful mutation for the
same path and inspect the durable Web timeline and raw session events.

QA impact 2026-07-15: new automatic title, interrupt-salvage, and file-mutation verifier behavior.
Planning flag only; no QA session ran in this implementation slice.

Phase C planning 2026-07-19: persona normalized to Théo; companion to US-004 (EC-6: auto-title,
interrupt salvage, verifier markers). Forensic contract (SD-006): timestamped commands with observed
output for one title spawn after the first assistant response (and none after the second), the
composed interrupt→steer salvage input under the new generation, and the verifier marker present
without a later successful edit and absent with one — plus `agh-ui-screenshot` captures for the
title and marker.

src: .compozy/tasks/hermes-comparison/_user_stories.md#us-004-compaction-under-pressure-crash-safe
