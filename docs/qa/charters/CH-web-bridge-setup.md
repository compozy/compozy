# CH-web-bridge-setup: Finish Web setup without stale or invented truth

```yaml
charter:
  id: CH-web-bridge-setup
  mission: "As Tessa, complete Slack setup through the Web while pressing Back, closing, refreshing, and repairing a bad token, and prove every checklist item and send action remains tied to daemon-observable state."
  mode: charter-with-tour
  persona:
    name: Tessa
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-complete-web-bridge-setup
  scenarios: [NB-026, NB-028, NB-039, NB-web-bridge-setup]
  tour: Back-Button Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Before create, select Slack and confirm the manifest handoff is advertised but no manifest GET occurs; submit twice rapidly and prove exactly one disabled bridge is created."
      - "After the committed create, press Back/close/refresh during manifest fetch and clipboard failure; every recovery must reuse the returned ID and never restore a stale pre-create draft."
      - "Bind a bad token, run verify, trace each checklist/card result to provider/binding/config/verify/lifecycle/health payloads, then mutate config/bindings and prove stale green evidence disappears before re-verify."
      - "Run Check target (dry run) and Send test message; record separate endpoints, pending states, result copy, and one provider call only for the real send."
    must_avoid:
      - "Inferring verified or registered from enabled/ready; accepting a visual checkmark without independent CLI/API read-back; treating screenshots as the only behavior evidence."
  evidence_expectations:
    - "Browser screenshots at create handoff, failed remediation, refreshed complete checklist, and dry-run/send-test results; request counters and independent bridge/binding/health JSON."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
