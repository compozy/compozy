# J-03 — Observe a run and approve the human gate

The evaluator's journey (PRD secondary persona, F5 human gate, ADR-005/017 §9.8). A run reaches a human-approval gate and rests in the **live `needs-approval` state** — a pause, never a finish. The evaluator finds it via the global Runs "Awaiting you" queue, reads the contract, and routes the decision: approve & resume, request changes, or reject & halt.

```mermaid
flowchart TD
    A[Entry: global Runs runs.html] --> B[KPI strip: Active / Awaiting you / Done today / Needs a look]
    B --> C{Outcome filter}
    C -->|Awaiting you = needs-approval queue| D[Open the run → run-detail]
    D --> E[Live needs-approval gate card: 'Approve merge to main?' + facts branch/diff/tests/verifier]
    E --> F{Human decision}
    F -->|Approve & resume| G[Side effect: gate approved → coordinator resumes → next generation/verify → done]
    F -->|Request changes| H[Side effect: revise next generation; run returns to running, remediation loop]
    F -->|Reject & halt| I[True end: terminal halt — NOT coerced to done]
    G --> K[True end: terminal banner done, decision recorded]
    E -.->|evaluator never acts| X1[Abandon: run stays LIVE in needs-approval (not terminal, zero cost); resumable when she returns]
    E -.->|reads on phone, connection drops mid-approve| X2[Abandon: no double-approve; decision idempotent or clearly failed, gate still actionable]
```

```yaml
journey:
  id: J-03
  name: "Observe a run and route the human-approval gate"
  value_statement: "An evaluator trusts a run truly completed and was verified, and can approve/decline the merge gate — from the global Runs queue, on desktop or phone."
  personas: [Marina, Sol]
  entry_points:
    - url: "web /loops/runs (runs.html) › Awaiting you"
      origin: in-app-nav
    - url: "web /loops/:name/runs/:id (run-detail) approval card"
      origin: in-app-nav
    - url: "CLI: agh loop approve <run> --decision approve|request_changes|reject --gate <id>"
      origin: direct
  actions:
    - step: 1
      verb: "Scan the global Runs KPIs and open the 'Awaiting you' queue"
      expected_observable: "KPI strip (Active / Awaiting you / Done today / Needs a look); outcome filter with per-status counts; Awaiting-you = the needs-approval queue"
    - step: 2
      verb: "Open the run and read the approval gate"
      expected_observable: "Live needs-approval gate card with the merge question + facts (branch, diff, tests, verifier); status shown as WAITING, not done/failed"
    - step: 3
      verb: "Route the decision"
      expected_observable: "Approve & resume → run resumes; Request changes → revise next generation; Reject & halt → terminal halt"
  goal:
    observable: "The routed decision takes effect: approve resumes to a truthful terminal, reject halts to a terminal that is NOT done"
    side_effects: [gate-decision-recorded, coordinator-resume-or-halt, status-change-event]
  true_end_state: "Reload: an approved run reads done with the decision recorded; a rejected run reads its terminal halt (never done); a still-un-actioned gate reads needs-approval (live, resumable), not a terminal."
  exit:
    natural: "Evaluator lands on the terminal (approved/rejected) run, or leaves the gate live for later."
  abandonment:
    - at_step: 2
      how: "Evaluator never acts on the gate."
      resume: "Run rests LIVE in needs-approval at zero cost (coordinator yielded); resumes exactly where it paused when she returns and approves (ADR-013)."
    - at_step: 3
      how: "On 4G the approve request drops mid-submit."
      resume: "No double-approve / ghost decision; the gate stays actionable or fails visibly — never a silent partial approval."
  crosses: [global-runs-index, run-detail, gate-evaluator, coordinator-resume, SSE-stream]

design_reference:
  screens:
    - "docs/design/opendesign/runs.html (LOOPS-DESIGN-SPEC §4.5)"
    - "docs/design/opendesign/run-detail.html (LOOPS-DESIGN-SPEC §4.4 — approval gate)"
  truthful_ui_checks:
    - "needs-approval renders as a LIVE waiting state, never as done or failed (ADR-013 inv5; PRD 'Truthful outcomes')."
    - "Reject & halt lands on a terminal that is NOT done (no success coercion)."
    - "Runs KPI 'Awaiting you' equals the needs-approval queue; outcome segments are data-driven (Paused/Queued appear only once such runs exist)."
    - "A11y (Sol): the gate status and the 11 run-status pills are announced/labelled, not color-only; the approval dialog traps focus and is escapable."

e2e_backbone:
  runtime:
    - "E2E-runtime-8: approve capability gate end-to-end — an agent cannot approve its own gate; operator/another agent can (also J-07)."
    - "E2E-runtime-10: observe all 11 status states across the lifecycle, none coerced."
  web:
    - "E2E-web-7: approval gate renders Approve & resume / Request changes / Reject & halt and routes each correctly."
    - "E2E-web-10: Runs workspace-wide KPIs + outcome filter with counts + Active/Past tables + row → run detail."
    - "E2E-web-9: all 11 states render with distinct truthful pills (no coercion)."
  integration:
    - "Integration-5: route approval decisions approve→resume / request_changes→revise / reject→terminal halt (ADR-005/017 §9.8)."
  component:
    - "Web-unit-5 (11 statuses → pill + pulse, reduced-motion gated); Web-unit-8 (never render a terminal coercion)."
  followups:
    - "AB-004 — observing all 11 states (incl. no-op/blocked/queued/paused) needs seeds that actually produce each state; rich SSE emission alone does not create those run states."
    - "AB-001 — real-daemon approval flow in Playwright depends on the loop e2e seed harness."
```
