# All-11-status observation seeds

- Legacy ID: AB-004
- Source: J-03, J-07, J-08 / LP-012, LP-026, LP-030, LP-038 / `_tests.md` E2E-runtime-10, E2E-web-9, Web-unit-5/8
- Why automate: the truthful-outcome guarantee needs seeds that produce each of the 11 states, including `no-op`, `blocked`, `queued`, and `paused`, which today's daemon rarely emits richly.
- Suggested layer: E2E runtime (produce each state) + web component (map each state to its distinct pill, reduced-motion-gated pulse, no coercion).
- Spec sketch: drive or seed a run into each state; assert a distinct truthful pill and that `exhausted`/`stalled`/`needs-approval`/`no-op`/`blocked` are never rendered as `done`/`failed`. The terminal `blocked` state must be produced behaviorally, not only rendered. True end state: all 11 states remain distinct across web and structured surfaces.
- Status: proposed
