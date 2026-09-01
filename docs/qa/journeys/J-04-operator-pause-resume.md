# J-04 — Operator pause, resume, or cancel a running Loop

The operator control journey (ADR-016/017, PRD "Observing"). An operator suspends a running Loop
at a generation boundary (`paused` — a live, resumable state distinct from system attention), resumes
it later, or immediately cancels it and its owned sessions. Pause is
**intent-decoupled from status**: it never flips to `paused` while a node is still executing.

```mermaid
flowchart TD
    A[Entry: run-detail of a Running loop] --> B{Pause available?}
    B -->|status = running| C[Press Pause → pause_requested=1, status STAYS running]
    B -->|status = watching / needs-approval / terminal| B2[Pause hidden/disabled — cannot pause a non-running run]
    C --> D[In-flight generation's claimed nodes finish]
    D --> E[Generation-finisher reads pause_requested at the boundary]
    E --> F{Boundary verdict}
    F -->|terminal or needs-approval reached first| G[Truthful outcome preempts pause — run is NEVER reported paused]
    F -->|still in progress| H[Transition running→paused at the boundary; zero lease, zero token, state held]
    H --> I{Operator action}
    I -->|Resume| J[paused→running, coordinator re-enqueued, resumes from snapshot → terminal]
    I -->|Cancel| K[Commit canceled operator_cancel + fence new work]
    K --> L[Concurrently stop every run-owned session; retry failed cleanup durably]
    L --> M[Borrowed origin session stays active; Resume is unavailable; Rerun can start new work]
    H -.->|operator never resumes| X1[Abandon: run rests LIVE in paused indefinitely at zero cost; resumable on demand]
    C -.->|daemon crashes after pause requested, before boundary| X2[Recovery: pause_requested durable → still yields paused on restart, no mid-node kill]
```

```yaml
journey:
  id: J-04
  name: "Pause, resume, or cancel a running Loop"
  value_statement: "An operator can suspend work safely or stop it immediately, with terminal state and session ownership always telling the truth."
  personas: [Bruno]
  entry_points:
    - url: "web /loops/:name/runs/:id (run-detail) Pause / Resume / Cancel controls"
      origin: in-app-nav
    - url: "CLI: compozy loop pause|resume|cancel <run>"
      origin: direct
    - url: "native tool: compozy__loop_pause / compozy__loop_resume / compozy__loop_cancel"
      origin: in-app-nav
  actions:
    - step: 1
      verb: "Press Pause on a running run"
      expected_observable: "pause_requested set; status STAYS running while a node executes; Pause is only offered from running"
    - step: 2
      verb: "Wait for the generation boundary"
      expected_observable: "Claimed nodes finish; at the boundary the run transitions running→paused (or a terminal/needs-approval verdict preempts the pause)"
    - step: 3
      verb: "Resume or Cancel"
      expected_observable: "Resume continues from the snapshot; Cancel immediately commits canceled(operator_cancel), fences new work, and stops every owned session without stopping the borrowed origin"
  goal:
    observable: "A paused run resumes to a truthful terminal, or Cancel ends canceled(operator_cancel) before owned-session cleanup — never coerced to done or failed"
    side_effects: [pause-requested, terminal-cancellation, work-fence, owned-session-cleanup, status-change-events]
  true_end_state: "After resume, the run reaches its genuine terminal outcome from where it paused; after Cancel, fresh reads show canceled, every owned session is stopped or queued for retry, the borrowed origin remains active, Resume is unavailable, and Rerun starts a new generation."
  exit:
    natural: "Operator lands on a resumed terminal run or a canceled run with the chosen cause."
  abandonment:
    - at_step: 2
      how: "Operator pauses and never resumes."
      resume: "Run rests LIVE in paused at zero cost; the durable loop_run row holds full state and resumes on demand."
    - at_step: 1
      how: "Daemon crashes after pause is requested but before the boundary."
      resume: "pause_requested is durable — recovery still yields paused at the next boundary, never a mid-node kill."
  crosses: [run-detail-controls, loop.Service.Pause/Resume/Cancel, node-control-ledger, session-cleanup-outbox, generation-finisher-tx, crash-recovery]

design_reference:
  screens:
    - "docs/design/opendesign/run-detail.html (LOOPS-DESIGN-SPEC §4.4 — run lifecycle controls; ADR-017 §9.3)"
  truthful_ui_checks:
    - "Status stays running while a node executes; it flips to paused ONLY at the generation boundary (ADR-013 inv5: running = a node is executing)."
    - "Pause is hidden/disabled outside running (watching / needs-approval / terminal cannot be paused) — N-003."
    - "Cancel commits canceled(operator_cancel) immediately, exposes no Kill action, and is never coerced to done/failed/exhausted/stalled."
    - "A terminal / needs-approval verdict at the boundary preempts a pending pause — the run is never reported paused when it actually finished."

e2e_backbone:
  runtime:
    - "E2E-runtime-6: pause a running loop at a generation boundary, hold state, resume to terminal (ADR-017)."
  web:
    - "E2E-web-8: pause to paused at a generation boundary, resume or cancel, with SSE live updates and no Kill action."
  integration:
    - "Integration-6: yield running→paused at a boundary with no orphaned nodes; resume to terminal; a terminal/needs-approval boundary verdict preempts the pause (never reported paused)."
    - "Integration-30: pause_requested durable across a crash before the boundary; idempotent Pause / Resume clears intent (race resolves deterministically)."
  unit:
    - "Unit-16: permit running⇄paused and reject illegal transitions; Cancel suites prove immediate terminal truth, owned-session interruption, borrowed-session preservation, and idempotency."
  component:
    - "Web-unit-2 (apply status_changed running→paused / paused→running to the run store via use-loop-stream)."
  followups:
    - "AB-001 — real-daemon pause/resume in Playwright depends on the loop e2e seed harness + rich-frame SSE emission."
```
