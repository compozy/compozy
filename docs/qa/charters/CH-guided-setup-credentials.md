# CH-guided-setup-credentials: Survive pasted credentials in the three guided setup flows

```yaml
charter:
  id: CH-guided-setup-credentials
  mission: "As Tessa, abuse then complete the WhatsApp, Telegram, and Discord guided setup flows, measuring actions and wall time for each provider and comparing Telegram with the Hermes ≈4-action baseline."
  mode: charter-with-tour
  persona:
    name: Tessa
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-connect-bridge-provider
  scenarios: [NB-bridge-provider-setup]
  tour: Paste Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "WhatsApp: paste a visible phone number into phone_number_id, then fake OpenAI/Slack/GitHub-shaped tokens into access_token; expect product-specific remediation before any Meta call or binding write."
      - "Telegram: use one truncated and one valid-shape BotFather token, prove daemon setWebhook needs zero operator-run curl, and record action count plus wall time against the Hermes Telegram ≈4-action baseline."
      - "Discord: supply a malformed Ed25519 public key, then a valid 64-hex fake; inspect the invite URL for bot + applications.commands and permission bits, and verify the Interactions Endpoint handoff is explicit."
      - "For WhatsApp and Discord, record action counts and wall time with baseline marked unavailable; never invent a Hermes comparison. Confirm every successful setup ends disabled with exact verify instructions and masked secrets."
    must_avoid:
      - "Real production credentials; counting daemon-performed setWebhook as a user action; omitting Discord after listing its scenario; treating validation as proof the provider dashboard is configured."
  evidence_expectations:
    - "Per-provider TTFM/action worksheet, structured setup output, validator failures, write-only binding reads, fake-provider registration/request logs, and final verify instructions."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
