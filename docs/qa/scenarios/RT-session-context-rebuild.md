---
id: RT-session-context-rebuild
area: RT
title: Rebuild provider context from one session's persisted transcript
persona: Théo
journey: J-11
expected: When ACP session loading is unsupported or the saved provider session is missing, AGH starts a fresh provider session, prepends the workspace checkpoint followed by only that AGH session's pruned persisted transcript to the first accepted prompt, preserves the authored message, and exposes one durable `Context rebuilt from log.` marker. A successful ACP load performs no replay and adds no recovery marker.
entry_points: daemon session reactivation; session transcript; session events
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
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
