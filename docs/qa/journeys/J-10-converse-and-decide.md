# J-10 — Converse and decide: observe a channel-harvest run

The differentiator and the hero screenshot (PRD F4/F10, use-cases §3, ADR-021). A Loop step drives a live multi-agent conversation in a team channel and harvests the agreed decision before advancing. The run page **embeds the channel conversation itself** — the user watches the agents converse, sees the harvested result highlighted, then watches the fan-out execute. The converse-and-decide template ships docs-only, but the runtime capability (`agh__network_send` + `channel_result` harvest) is live and rendered.

```mermaid
flowchart TD
    A[Entry: run a Loop whose step posts to a team channel] --> B[Step: agh__network_send posts the problem into #channel]
    B --> C[Run-detail embeds the live channel conversation: agents discuss]
    C --> D{Result reached in the window?}
    D -->|designated result signal posted default / responder / content-rule| E[Harvest channel_result payload — highlighted in the timeline]
    E --> F[Fan out over the agreed task list → rejoin → definition-of-done gate]
    F --> G[True end: terminal done — the agreed work executed and verified]
    D -->|no decision within the window| S[True end: terminal stalled + escalate — no decision harvested]
    C -.->|user watches, decision never comes, leaves| X1[Abandon: run ends stalled on its own; no infinite wait]
    B -.->|no configured channel/agents| X2[Abandon: no installed template — needs a hand-built seed to exercise (docs-only)]
```

```yaml
journey:
  id: J-10
  name: "Observe a converse-and-decide run: channel discussion → harvested decision → execute"
  value_statement: "A user watches agents converse to a decision inside the run and sees the harvested result drive the next step — the multi-agent capability no plain orchestrator has."
  personas: [Bruno]
  entry_points:
    - url: "web /loops/:name/runs/:id (run-detail — embedded channel panel)"
      origin: in-app-nav
    - url: "CLI: agh loop run --name <converse-and-decide-fork> (hand-built; docs-only template)"
      origin: direct
  actions:
    - step: 1
      verb: "Run a Loop whose step posts to a team channel"
      expected_observable: "The step posts via agh__network_send with a channel_result harvest declared (window, optional responder/content-rule)"
    - step: 2
      verb: "Watch the embedded conversation"
      expected_observable: "The run-detail timeline embeds the live channel (implementer/reviewer/decision messages) using the existing network conversation components"
    - step: 3
      verb: "See the harvested decision drive the fan-out"
      expected_observable: "The designated result is harvested and highlighted; the Loop fans out over the agreed task list → gate → done"
  goal:
    observable: "The harvested decision is shown and drives execution to a verified terminal done; a windowed no-decision ends stalled"
    side_effects: [channel-post, channel_result-harvest, fan-out-execute]
  true_end_state: "The harvested result payload is visible in the timeline, the fan-out executed the agreed tasks, and the run reached done — OR, with no decision in the window, the run ended stalled with an escalation (never a fabricated decision)."
  exit:
    natural: "User lands on the terminal run with the harvested decision and its downstream execution visible."
  abandonment:
    - at_step: 2
      how: "The agents never reach a decision; the user leaves."
      resume: "The run ends stalled on its own window guard — no infinite wait for a decision."
    - at_step: 1
      how: "No channel/agents are configured (there is no installed converse-and-decide template)."
      resume: "The journey needs a hand-built seed (agh__network_send + channel_result node) to exercise — flagged as an E2E follow-up, not an installed path."
  crosses: [network-channel, agh__network_send, channel_result-harvest, fan-out/collect, done-gate]

design_reference:
  screens:
    - "docs/design/opendesign/run-detail.html (LOOPS-DESIGN-SPEC §4.4 — embedded channel + harvested decision)"
  truthful_ui_checks:
    - "The result-reached convention is honest: the default is a designated result signal whose payload is harvested; responder/content-rule are opt-ins (PRD F4). The UI must not fabricate a decision."
    - "No decision within the window ends stalled, never a coerced done."
    - "channel posting renders as agh__network_send with harvest: {kind: channel_result, ...}; the retired channel-post kind must not appear (ADR-021)."

e2e_backbone:
  runtime: []
  web:
    - "E2E-web-6: run page channel — render the embedded converse-and-decide conversation with the harvested result."
  integration:
    - "Integration-21: harvest the channel result on the happy path and reach stalled with no result in the window, over a fake conversation store independent of the docs-only template (N-003)."
  unit:
    - "Unit-26: harvest the designated result coordination message within the window and stall on silence; §7-10 (agh__network_send + channel_result; retired channel-post rejected)."
  followups:
    - "AB-002 — converse-and-decide has NO installed template (docs-only). A hand-built seed loop (agh__network_send + channel_result node + configured channel/agents) is required to exercise the run-detail channel panel end-to-end (E2E-web-6). Highest-effort follow-up in this cycle."
```
