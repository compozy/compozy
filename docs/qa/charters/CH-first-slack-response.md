# CH-first-slack-response: Reach the first Slack response from a generated manifest

```yaml
charter:
  id: CH-first-slack-response
  mission: "As Tessa, connect Slack from an installed extension to one visible in-thread AGH response, measure time to first message (TTFM) as operator actions and wall-clock minutes against the Hermes ≈7-action baseline, and stop on any undocumented or ambiguous handoff."
  mode: scenario-based
  persona:
    name: Tessa
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-connect-bridge-provider
  scenarios: [NB-025, NB-029, NB-036, NB-037, NB-038, NB-bridge-provider-setup]
  tour: Feature Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Start at extension installed; count one action for each CLI command and one action for each provider-dashboard screen submission/paste from manifest generation through the first provider-visible response. Record total actions, wall time, and delta from the Hermes Slack ≈7-action baseline."
      - "Create disabled, generate the daemon-owned manifest for the persisted ID, validate its scopes/events/URLs, simulate the api.slack.com paste/install, and bind bot_token plus signing_secret without reading either value back."
      - "Run verify while disabled, enable, verify reachability, create one inbound route, then run a real send-test in that route; confirm the response is in-thread exactly once."
      - "At one credential step, stop and reopen the disabled instance; existing bindings must stay masked and the flow must resume without creating a duplicate bridge."
    must_avoid:
      - "Using raw database state as success evidence; treating dry-run test-delivery as the first real message; claiming a real Slack dashboard action when the lab only simulates it."
  evidence_expectations:
    - "Action-count ledger with command/dashboard action, timestamp, and observable; structured manifest/verify/route/send-test payloads; provider-fake request log."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
