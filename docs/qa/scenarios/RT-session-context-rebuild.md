---
id: RT-session-context-rebuild
area: RT
title: Rebuild provider context from one session's persisted transcript
persona: Théo
journey: J-11
expected: When ACP session loading is unsupported or the saved provider session is missing, Compozy starts a fresh provider session, prepends the workspace checkpoint followed by only that Compozy session's pruned persisted transcript to the first accepted prompt, preserves the authored message, and exposes one durable `Context rebuilt from log.` marker. A successful ACP load performs no replay and adds no recovery marker.
entry_points: daemon session reactivation; session transcript; session events
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-rt-current-source-20260730-20260730-061631-252740-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: RT-015; RT-session-message-reload
---

Use two workspaces with deliberately similar transcript content. Stop and reactivate one session
through a provider fixture that advertises no `session/load` support, then send a prompt that depends
on a unique fact from its earlier transcript. Confirm the provider receives the workspace checkpoint
before the pruned local replay exactly once, the visible authored prompt remains unchanged, and the
transcript contains one typed recovery marker. Repeat with a valid provider session load and confirm
that no replay or marker is added.

QA impact 2026-07-15: new runtime recovery behavior. Planning flag only; no QA replay ran in this
implementation slice.

QA impact 2026-07-15: degraded replay now prepends the workspace checkpoint before the session-local
transcript. Status remains untested; this is a planning flag, not fresh QA evidence.

Phase C planning 2026-07-19: persona normalized to Théo (session hero); settles US-003 (D4,
ADR-002).

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- Kill + resume command sequence with timestamps and the marker event in the transcript.
- The agent response demonstrating a pre-restart fact after degraded resume.
- Byte-identical event-store hash before/after the prune pass, and a successful `session/load` run
  with no replay and no marker.

QA impact 2026-09-06 sessions-stability task_06 (final walk owned by task_10): compact
settled history during an active watched turn. Compare the remaining events and
transcript with the reset snapshot, verify the generation advances once and the active
message keeps its identity/content without a visible jump. Reconnect from a retention-
erased cursor and verify a stated cursor_expired snapshot. Focused store/manager checks
are recorded in sessions-stability/memory/task_06.md; this browser walk has not run.

QA 2026-09-06 — sessions-stability selected scope: PASS for the selected pressure-compaction/recovery branch: profile-bound resume succeeds; first and repeated archives advance generation0→1→2, preserve raw event contents and current authored/assistant identity, and reset old-generation/expired cursors. The actual Find query invalidates without losing focus. The full historical degraded-provider-load forensic charter is not rerun or newly claimed. Evidence: docs/qa/reports/2026-09-06-sessions-stability.md.
