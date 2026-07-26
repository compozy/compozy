# J-08 — Watch and maintain: a self-correcting watch-source Loop

The identity case (PRD F8, use-cases §1, `reviews-watch`). A Loop is driven by an external signal — new review comments on an open change — cycling fetch → fix → verify each time the source wakes it, and concluding on its own (`done`, a clean `no-op` tick, or `stalled` if the source goes silent past its window). Between events it rests in the live, zero-cost `watching` state. This is also the extensibility surface: the `coderabbit_pr_review` watch source, the fetch/resolve/push tools, and the `review_fixer` agent all arrive from the default-enrolled `dev-cycle` extension (ADR-024/ADR-016), and CodeRabbit behavior **requires `gh` installed and authenticated** — a missing/unauthenticated `gh` must surface as an actionable availability error, never a silent hang or a fake `watching` state (LP-039).

```mermaid
flowchart TD
    A[Entry: run reviews-watch manual/cli/agent] --> B[Loop arms its watch-source]
    B --> C[Live watching state: dormant, ZERO cost, zero lease]
    C --> D{Watch event?}
    D -->|new review lands| E[Wake → wait a beat → confirm the reviewer finished]
    E --> F[Round: fetch new comments → fan out one fix per comment → rejoin]
    F --> G{Resolve-check gate: anything unresolved?}
    G -->|unresolved remain| C
    G -->|zero unresolved, work done| H[True end: terminal done]
    D -->|clean tick, nothing new| N[Terminal no-op — a clean watch tick is NEVER fake done]
    D -->|source silent past the window| S[True end: terminal stalled + escalate ping]
    B --> W{Watch-source provider}
    W -->|dev-cycle extension: coderabbit_pr_review, gh authenticated| C
    W -->|gh missing or unauthenticated| WG[Actionable availability error — no silent hang, no fake watching]
    W -->|extension WITH loop.watch_source over watch/poll| C
    W -->|extension WITHOUT the capability| WE[Cannot serve the source — gated, surfaced in InitializeResponse]
    C -.->|operator stops a watching run| X1[Abandon: terminal failed(operator_stop), never coerced]
    S -.->|never re-armed| X2[Abandon: run ended stalled and pinged — no infinite silent wait]
```

```yaml
journey:
  id: J-08
  name: "Run a watch-source Loop that self-corrects and concludes on its own"
  value_statement: "A recurring, externally-triggered Loop keeps a change clean without a babysitter and concludes truthfully — done, a clean no-op, or stalled when the source goes quiet."
  personas: [Bruno, Marina]
  entry_points:
    - url: "web /loops (loops-catalog) reviews-watch › Run  /  web run-detail (rounds)"
      origin: in-app-nav
    - url: "CLI: agh loop run --name reviews-watch --input pr=123 (until-clean IS the default: iteration_cap 0; max-rounds via loop_config)"
      origin: direct
  actions:
    - step: 1
      verb: "Start reviews-watch on an open change"
      expected_observable: "Run enters the live watching state (zero cost, zero lease) between events; catalog shows the watch tag + iteration cap ∞"
    - step: 2
      verb: "A new review lands"
      expected_observable: "The Loop wakes, waits a quiet-period beat, confirms, then runs a round: fetch → fan out fixes → resolve-check"
    - step: 3
      verb: "Repeat until clean / silent"
      expected_observable: "Zero-unresolved → done; a clean tick with nothing new → no-op; silence past the window → stalled + escalate"
  goal:
    observable: "The Loop concludes with a truthful terminal outcome (done / no-op / stalled) appropriate to what actually happened"
    side_effects: [watch-source-wakes, fan-out-fix-runs, escalate-ping-on-stalled]
  true_end_state: "A clean watch tick is reported as no-op (never done-with-fake-work); a silent source ends stalled with an escalation, not an infinite wait; a resolved change ends done — each verifiable on reload."
  exit:
    natural: "Operator/evaluator lands on the truthful terminal run; the change is clean or the escalation is visible."
  abandonment:
    - at_step: 3
      how: "The reviewer goes silent and never posts again."
      resume: "The source's no-progress guard ends the run stalled and pings — no infinite silent wait."
    - at_step: 1
      how: "Operator stops a watching run."
      resume: "Terminal failed(operator_stop), never coerced to done."
  crosses: [watch-source-poll, extension-capability-gate, fan-out/collect, resolve-gate, escalation]

design_reference:
  screens:
    - "docs/design/opendesign/run-detail.html (LOOPS-DESIGN-SPEC §4.4 — rounds/generations timeline)"
    - "docs/design/opendesign/loops-catalog.html (§4.1 — watch tag, cap ∞; the watch-source tag is a body-node concept, never a start-binding badge)"
  truthful_ui_checks:
    - "watching is a LIVE zero-cost dormant state — not terminal, not running (ADR-013)."
    - "no-op renders as its own neutral terminal — a clean tick is NEVER shown as done-with-fake-work (ADR-022 inv5)."
    - "stalled (silent source) renders as itself, never as done."
    - "The reviews-watch watch-source is a body-node concept and never gets the catalog start-binding badge (§4.1)."
    - "loops-refac (2026-07-08): reviews-watch's fix_batch run-agent sessions are now policy-gated (resolved sandbox/permission + subset-only allowed_tools) — the wake/remediate/done behavior is unchanged, but LP-029 is reset to verify the new session posture (CH-005 gating bullet). NOTE this is the ADR-016 extension watch_source; the new daemon-internal watch-events source class is J-11."

e2e_backbone:
  runtime:
    - "E2E-runtime-2: wake reviews-watch on fake review events, remediate, and conclude (incl. stalled on silence)."
    - "E2E-runtime-9: drive a loop end-to-end from an extension-provided watch-source over watch/poll (ADR-016)."
    - "E2E-runtime-7: no-progress → stalled, fan-out ceiling → exhausted/escalate (guardrail side)."
  web:
    - "E2E-web-9: all 11 states render truthfully (covers watching / no-op / stalled pills)."
  integration:
    - "Integration-2: watch-source poll→ready→settle→confirm with a fake source; end stalled on silence past the window."
    - "Integration-18: gate watch/poll — an extension WITHOUT loop.watch_source cannot serve it; one WITH it serves; watch_source_kinds surfaced in InitializeResponse."
  unit:
    - "Unit-6 (zero lease/token for a watching loop); Unit-15 (no false done, exhausted/stalled/needs-approval) + §7-15 (the precise no-op/blocked terminal-not-coerced owner); Unit-2 (stalled on no-progress window)."
  followups:
    - "LP-039 — gh missing/unauthenticated availability path is owned by the dev-cycle provider-failure suite (`_changes_spec.md` v2 R9); walk it with a lab shell whose PATH hides gh (or logged-out gh) — distinct from LP-038's mid-run blocked classification."
    - "AB-001 — the loop e2e-web seed harness must include a watch-source seed (fake review events) to exercise watching/round/stalled in Playwright; daemon rich SSE emission exists, but browser seeds still do not drive this journey."
    - "AB-004 — LP-038 owns the truthful-`blocked` guarantee (external dependency impossible, ADR-022), but it needs a behavioral seed (a watch-source missing a credential, or a refused run-loop cycle) to walk a run into `blocked` — not just render the pill; qa-execution seeds it via AB-004."
    - "Watch-source PUSH path (watch/subscribe + loops/watch/notify) and MCP-backed passive sources are v1-deferred (not in scope for this cycle) — recorded, not tested."
```
