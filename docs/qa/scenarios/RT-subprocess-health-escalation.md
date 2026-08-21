---
id: RT-subprocess-health-escalation
area: RT
title: Escalate a task run after ACP subprocess health failure
persona: Ada
journey: J-diagnose-task-session-health
expected: Active failed ACP health verdicts produce the same bounded evidence through HTTP, UDS, compozy status, and runtime.subprocess_health doctor output; the configured threshold moves the exact linked nonterminal run to needs_attention once, an unexpected process exit escalates immediately, terminal runs remain terminal, and threshold 0 preserves diagnostics without task mutation.
entry_points: daemon.subprocess_health_escalation_threshold; GET /api/status; GET /api/doctor; compozy status; compozy doctor --only runtime.subprocess_health; compozy task run recover
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits: 75ce57f2;ed93a4b3
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-rt-current-source-20260730-20260730-061631-252740-lab/qa-artifacts/qa;internal/daemon/subprocess_health_escalator_test.go
last_report: docs/qa/reports/2026-08-20-pr-447-runtime-recovery.md
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

- Matching failed-verdict evidence across HTTP, UDS, `compozy status`, and doctor output.
- The single `needs_attention` transition (one canonical event with correlation keys) for the exact
  linked nonterminal run, plus the immediate-escalation crash run.
- The threshold-0 run preserving diagnostics without task mutation, and the terminal-run
  precedence capture.

src: .compozy/tasks/hermes-comparison/_techspec.md#35-reliability-adr-010-fixes-d5

QA impact 2026-08-20: a confirmed managed-session crash now leaves Loop worker recovery to the Loop lifecycle owner instead of also parking the linked task run through generic subprocess escalation. Reset for crash classification and single-owner recovery verification.

QA 2026-08-20: blocked because no public surface injects a confirmed managed-session crash while preserving the Loop ownership link. Human rerun: start a checkpointing Loop node with a killable provider fixture, terminate that provider process, and compare `compozy loop status --run-id <run-id> -o json` with `compozy task run list -o json`; the Loop continuation must advance once and generic subprocess escalation must not park the task run.
