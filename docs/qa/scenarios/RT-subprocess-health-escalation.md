---
id: RT-subprocess-health-escalation
area: RT
title: Escalate a task run after ACP subprocess health failure
persona: Ada
journey: J-diagnose-task-session-health
expected: Active failed ACP health verdicts produce the same bounded evidence through HTTP, UDS, agh status, and runtime.subprocess_health doctor output; the configured threshold moves the exact linked nonterminal run to needs_attention once, an unexpected process exit escalates immediately, terminal runs remain terminal, and threshold 0 preserves diagnostics without task mutation.
entry_points: daemon.subprocess_health_escalation_threshold; GET /api/status; GET /api/doctor; agh status; agh doctor --only runtime.subprocess_health; agh task run recover
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-033; RT-002
---

Start a task-bound session with a provider fixture that exposes ACP health checks. Confirm failed
verdict counts and redacted reasons match across HTTP, UDS, CLI JSON, and doctor. Cross the configured
threshold and verify the exact run reaches `needs_attention` with one event under repeated checks.
Repeat with a terminal run, then crash a linked nonterminal subprocess. Finally set the threshold to
`0`, restart, and confirm diagnostics remain visible without a task-run transition. Recover the
parked run only after repairing the provider cause.

QA impact 2026-07-16: new restart-required config, status/doctor evidence, and automatic task-run
escalation. Planning flag only; no QA session ran in this implementation slice.

Phase C planning 2026-07-19: settles defect D5 (ADR-010 §4, ADR-011) in the coverage map.

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- Matching failed-verdict evidence across HTTP, UDS, `agh status`, and doctor output.
- The single `needs_attention` transition (one canonical event with correlation keys) for the exact
  linked nonterminal run, plus the immediate-escalation crash run.
- The threshold-0 run preserving diagnostics without task mutation, and the terminal-run
  precedence capture.

src: .compozy/tasks/hermes-comparison/_techspec.md#35-reliability-adr-010-fixes-d5
