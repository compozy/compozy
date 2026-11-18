---
id: RT-daemon-drain-admission
area: RT
title: Drain new-work admission without interrupting admitted work
persona: Dora
journey: J-drain-daemon-safely
expected: Draining the daemon through CLI, HTTP, or UDS returns the same stable draining state, projects informational status and doctor evidence, refuses new session, prompt, enqueue, and claim work with HTTP 503, lets admitted prompts and claimed runs finish, and restores admission after undrain or restart.
entry_points: compozy drain; compozy undrain; POST /api/drain; POST /api/undrain; compozy status; compozy doctor; session and task admission surfaces
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-rt-current-source-20260730-20260730-061631-252740-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: MS-daemon-memory-reporting; TA-daemon-lifecycle-command-guard
---

Start one prompt and claim one task run, then drain Compozy through one control transport. Confirm the
same state through the other transports, verify new work receives the stable temporary refusal, and
finish the admitted prompt and run. Undrain and confirm new work succeeds. Repeat drain, restart the
daemon, and confirm the in-memory state returns to active only after boot reconciliation.

With three sessions still streaming, request `compozy daemon restart`. During the restart drain (the
smaller of `daemon.shutdown_grace` and 5 seconds, skipping turns blocked on the operator), admitted
work may finish but new work is refused. Remaining turns retain
their interruption checkpoints; provider stop ladders run concurrently inside the total shutdown
budget. Replacement boot refuses new work while reconciliation runs, then admits at most one
continuation per interrupted turn before queued inputs drain. Repeat with a slow/cooperatively
unresponsive provider and confirm all old processes are gone before replacement dispatch.
Repeat using `compozy daemon stop`: it cancels work and the next start sends no continuation.

QA impact 2026-10-23 (#939): grace and total shutdown budgets, concurrent stops, boot admission,
and explicit-stop semantics changed. Prior evidence is historical; this version requires re-walk.

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

Targeted automated verification 2026-10-23 (#939): the changed runtime contracts passed in the
canonical session/daemon suites and isolated real-process probes. Restart/stop detail and persisted
events were independently read over HTTP and UDS after replacement and a third boot; no false failed
badge or duplicate continuation was observed. See
[the scoped report and public readback evidence](../reports/2026-10-23-issue-939-restart-continuation.md).
The full persona/Web/CLI journey remains untested; this automated evidence does not replace that verdict.

QA impact 2026-11-04 (#1241 follow-up): repeat shutdown with a core or telemetry
PASSIVE checkpoint stalled in filesystem I/O. Database close must return at its
caller deadline while owned cleanup retains both connections until the stalled
worker exits, then closes them. The canonical `TestStoreSQLiteHelpers` suite
verifies this at the driver I/O boundary with real SQLite. The full daemon walk
remains unrun in this slice under the host-load restriction.

CI follow-up (#1274): under the thirty-session SQLite transport load, a bounded background
checkpoint execution deadline defers maintenance and retains its idle retry. Other I/O errors
remain failures. Re-run the existing load lane with unchanged contention assertions and confirm
accepted transcripts survive restart without passive-checkpoint failure warnings.

QA impact 2026-11-04 (SQLite acceptance): while 30 sessions stream, periodic
checkpoint I/O may outlive its 200 ms admission budget on its dedicated connection.
Verify it completes after a delayed sync, foreground persistence continues,
errors remain visible, and close cancels/joins maintenance before releasing files.
Idle maintenance must also return while passive I/O is stalled; only its TRUNCATE
attempt uses the writer with the bounded reader wait.
`TestSessionDBPassiveCheckpoint/Should_complete_periodic` covers real SQLite I/O
beyond admission expiry; the existing blocked-I/O cases own writer/shutdown
isolation. The paired-load and thirty-session daemon jobs own the CI walk.
