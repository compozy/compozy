# CH-judge-session-attribution: Does one Goal judge stay bound to its exact origin session?

```yaml
charter:
  id: CH-judge-session-attribution
  mission: "As Lea, start two neighboring Goals and prove each judge turn is attributed to the exact origin session, attempt, role, and correlation without cross-process leakage."
  mode: scenario-based
  persona:
    name: Lea
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-26
  scenarios: [GL-judge-session-contract]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Create two independent session-origin Goals with distinguishable objectives and observe their judge results through public Run/turn/session reads."
      - "Confirm every judge result remains attached to the exact origin session and correlation while adjacent work proceeds."
      - "Reload both session views and verify no verdict, issue, or evidence crosses between them."
    must_avoid:
      - "Inferring attribution from prompt wording alone or using a private diagnostic map as the user-facing verdict."
  evidence_expectations:
    - "Structured Goal Run/turn/session outputs carrying the origin identities and a fresh-read comparison for both sessions."
  truthful_ui_check: "The visible Goal timeline must never display a neighboring session's judge verdict, issue, evidence, or completion state."
```
