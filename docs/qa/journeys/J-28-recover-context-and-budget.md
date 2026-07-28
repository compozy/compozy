# J-28 — Recover from context pressure and budget boundaries truthfully

A delivery operator lets a Goal run long enough to encounter every ACP stop reason, context-health state, compaction/reseed path, and nested token/wall budget boundary. The user value is trust: no partial output is judged, no post-crossing effect starts without the exact grant, and every recovery action is visible.

```mermaid
flowchart TD
    A[Entry: active Goal with reporting or silent ACP profile] --> B[Dispatch one work turn after fresh budget fence]
    B --> C{Typed ACP stop reason}
    C -->|end_turn|max_turn_requests| D[Persist turn and run authoritative judge]
    C -->|max_tokens| E[Persist consumed turn; no judge]
    C -->|refusal| F[Persist no partial judge; needs approval]
    C -->|cancelled| G[Winning control or observable pause; no continuation]
    E --> H{Effective compact path?}
    H -->|yes| I[Correlated compact operation, not a human turn]
    H -->|no editor-owned| J[Fenced automatic reseed]
    H -->|no session-origin| K[UsageLimited + explicit reseed approval]
    I --> L{Newer usage sequence proves effective?}
    L -->|lower/fresh| B
    L -->|equal, stale, absent| M[Pending/ineffective recovery persists]
    M -->|second ineffective streak| K
    K -->|Approve exact rotate-binding grant| N[One successor binding; moved-session link]
    N --> B
    B --> O{Token/wall budget crosses before or during effect?}
    O -->|halt| P[BudgetLimited terminal; no new effect]
    O -->|escalate| Q[Needs approval with exact settle-current or work-and-settle scope]
    Q -->|Approve| R[One recorded turn closure/work allowance]
    R --> B
    D --> S{Judge approved?}
    S -->|yes| T[True end: complete with truthful usage/context audit]
    S -->|rejected| B
    A -.->|operator leaves while usage pending or approval required| X1[Abandon/resume: reload preserves pending baseline, grant cause, and moved binding]
```

```yaml
journey:
  id: J-28
  name: "Recover from context pressure and budget boundaries truthfully"
  value_statement: "A long-running Goal survives context and budget pressure without replaying work, judging partial output, or starting an ungranted effect."
  personas: [Bruno]
  entry_points:
    - url: "web session Goal chip and Run timeline"
      origin: in-app-nav
    - url: "CLI/HTTP/UDS Goal controls and turn audit"
      origin: direct
  actions:
    - step: 1
      verb: "Run work through all five typed ACP stop reasons"
      expected_observable: "Each reason takes its specified branch; refusal/cancelled partial output is never judged and max-turn-requests is not UsageLimited."
    - step: 2
      verb: "Observe known, unknown, pending, stale, and compacted context"
      expected_observable: "No host estimate appears; compaction is correlated and pending until newer telemetry proves effectiveness."
    - step: 3
      verb: "Approve a session-origin reseed when required"
      expected_observable: "Exactly one grant rotates one binding; the origin remains owner and the active-session link is explicit."
    - step: 4
      verb: "Cross token or wall budget before and inside a turn"
      expected_observable: "No queued effect starts after crossing; halt or the exact turn-scoped grant settles only its authorized work."
  goal:
    observable: "The Goal completes or stops at a truthful usage/budget boundary with a complete turn, context, grant, and binding audit."
    side_effects: [usage-flush, compaction-operation, binding-reseed, typed-control-grant]
  true_end_state: "Reload shows the same context state, stop reason, effective limit, grant consumption, and moved binding; no original prompt or evaluator effect ran twice."
  exit:
    natural: "The operator reaches complete, UsageLimited approval, or BudgetLimited terminal with an exact next action."
  abandonment:
    - at_step: 2
      how: "The daemon restarts while compaction usage is pending."
      resume: "The durable baseline pair reloads exactly, including canonical unknown nil/nil, and newer telemetry is classified once."
    - at_step: 4
      how: "The operator leaves while a prepared entry is fenced on budget."
      resume: "The entry remains non-dispatchable until the matching grant or terminal halt; no effect starts from stale authorization."
  crosses: [ACP-stop-reasons, context-telemetry, compaction, session-binding, budget-ledger, approval-grants, restart-recovery]

e2e_backbone:
  runtime: ["_tests.md runtime cases 5-7 plus Task 02/round-7 recovery integration"]
  web: ["_tests.md E2E-web 2 and 8"]
  integration: ["_tests.md integration 4, 10, 12-13, 18, and 26"]
  scenarios: [GL-017, GL-018, GL-019, GL-020, GL-021, GL-038, GL-040]
```
