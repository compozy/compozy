# CH-profile-scoped-loop-tools: Keep lifecycle schemas inside the acting Loop scope

```yaml
charter:
  id: CH-profile-scoped-loop-tools
  mission: "As Bruno, validate a Loop lifecycle action in its enabled workspace and Profile while peer scopes reject the same action."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-complete-partial-loop
  scenarios: [LP-extension-action-schema-scope]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Validate a Loop using a lifecycle-loaded extension action through CLI and HTTP/UDS in the owning workspace and Profile."
      - "Repeat validation from the default Profile and a peer workspace; both must return unknown_action_kind."
      - "Restart the daemon, confirm the extension placement remains healthy, and repeat the scoped validation."
    must_avoid:
      - "Using database reads, internal endpoints, or mocked providers as behavior-first pass evidence."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
