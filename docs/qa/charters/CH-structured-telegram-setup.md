# CH-structured-telegram-setup: Complete Telegram setup with structured surfaces only

```yaml
charter:
  id: CH-structured-telegram-setup
  mission: "As Ada, complete Telegram setup twice with no TTY or browser—strict CLI JSON in one lane and HTTP/UDS operations in the other—while measuring time to first message (TTFM) as user-equivalent actions and wall time against the Hermes ≈4-action baseline."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-connect-bridge-provider
  scenarios: [NB-024, NB-025, NB-026, NB-036, NB-037, NB-038, NB-039, NB-bridge-provider-setup]
  tour: Feature Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "CLI lane: submit exactly one strict JSON object, reject an unknown field and a trailing object, then complete create → bindings → daemon webhook registration → verify → enable → send-test using parseable output only."
      - "HTTP/UDS lane: issue the equivalent create, secret-binding, webhook/register, verify, enable, and send-test calls; compare typed status/body semantics and confirm there is no invented /api/bridges/setup route or native-tool substitute."
      - "Count one user-equivalent action per structured command/request, count daemon setWebhook as zero, record wall time, and compare both lanes with the Hermes Telegram ≈4-action baseline."
      - "Prove failed verify returns structured records before a non-zero exit, test-delivery remains a zero-send target check, and every operation remains scoped to the chosen workspace bridge."
    must_avoid:
      - "Interactive prompts, Web UI, parsing human log prose, direct database/vault inspection, or using a CLI-only shortcut in the HTTP/UDS parity lane."
  evidence_expectations:
    - "JSON/HTTP/UDS request-response corpus, exit codes/status codes, action and wall-time table, fake Telegram setWebhook/send logs, and cross-surface semantic diff."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
