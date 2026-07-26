# Loop web E2E seed harness (real-daemon Playwright)

- Legacy ID: AB-001
- Source: J-01, J-03, J-04, J-06, J-08, J-09 / LP-001..LP-005, LP-008..LP-016, LP-021..LP-024, LP-029..LP-035 / `_tests.md` E2E-web-2..9, 12..17
- Why automate: high-value stable journeys blocked from real-daemon browser E2E — `web/e2e/fixtures/*` has no loop seed flow that drives the rich Loop SSE states the run page binds. Covered today at daemon/runtime tests, Vitest/component layer, and `agh-ui-screenshot` visual parity.
- Suggested layer: E2E browser (`make test-e2e-web`) + a daemon-side loop seed fixture emitting the enumerated SSE kinds.
- Spec sketch: seed a running/needs-approval/paused/watching loop run with generation, gate, and meter frames; drive catalog → run form → run detail; assert contract header, meters, timeline, approval routing, pause at boundary, and truthful terminal banner. True end state: the browser view matches the seeded run and reloads without an optimistic-UI lie.
- Status: proposed
