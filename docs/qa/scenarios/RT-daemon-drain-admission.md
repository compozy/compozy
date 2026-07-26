---
id: RT-daemon-drain-admission
area: RT
title: Drain new-work admission without interrupting admitted work
persona: Dora
journey: J-drain-daemon-safely
expected: Draining the daemon through CLI, HTTP, or UDS returns the same stable draining state, projects informational status and doctor evidence, refuses new session, prompt, enqueue, and claim work with HTTP 503, lets admitted prompts and claimed runs finish, and restores admission after undrain or restart.
entry_points: agh drain; agh undrain; POST /api/drain; POST /api/undrain; agh status; agh doctor; session and task admission surfaces
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-daemon-memory-reporting; TA-daemon-lifecycle-command-guard
---

Start one prompt and claim one task run, then drain AGH through one control transport. Confirm the
same state through the other transports, verify new work receives the stable temporary refusal, and
finish the admitted prompt and run. Undrain and confirm new work succeeds. Repeat drain, restart the
daemon, and confirm the in-memory state returns to active.

QA impact 2026-07-15: new daemon-global admission control, status/doctor projection, and public
CLI/HTTP/UDS behavior. Planning flag only; no QA session ran in this implementation slice.

Phase C planning 2026-07-19: persona normalized to Dora (runtime administrator) and journey moved
J-11 → J-drain-daemon-safely; settles US-006 (ADR-010 §3).

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- Drain command, the refused-admission error, in-flight completion, and undrain restore — all
  timestamped in sequence.
- Identical drain state read over UDS and HTTP, plus the doctor payload capture showing `draining`.
- The idempotent second drain (no-op, same status) and the post-restart active state.

src: .compozy/tasks/hermes-comparison/_user_stories.md#us-006-graceful-drain
