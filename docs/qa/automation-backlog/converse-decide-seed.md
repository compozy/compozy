# Converse-and-decide run seed (no installed template)

- Legacy ID: AB-002
- Source: J-10 / LP-036, LP-037 / `_tests.md` E2E-web-6, Integration-21, Unit-26
- Why automate: high-value differentiator with no runtime-installed template (docs-only, PRD F10); the run-detail channel panel and `channel_result` harvest cannot be exercised end to end without a hand-built seed loop. Blocks CH-010 from running.
- Suggested layer: E2E browser (channel panel render) + integration (harvest happy path + windowed stall) over a fake conversation store.
- Spec sketch: build a seed loop with an `agh__network_send` node declaring `harvest: {kind: channel_result, window, responder?}`; drive a conversation to a designated result and assert the highlighted payload drives the fan-out to `done`; a no-result window must end `stalled`. True end state: the harvested decision is visible and executed, or the run stalls with escalation.
- Status: proposed
