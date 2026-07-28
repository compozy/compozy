# CH-long-provider-replies: Preserve long replies across every chat-provider limit

```yaml
charter:
  id: CH-long-provider-replies
  mission: "As Omar, deliver one adversarial long-form reply through all six chat providers and prove wire limits, Unicode boundaries, ordering, fence repair, dialect fidelity, and final acknowledgement without freezing a guessed chunk count."
  mode: charter-with-tour
  persona:
    name: Omar
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-deliver-long-formatted-reply
  scenarios: [NB-long-bridge-replies]
  tour: Paste Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Use a deterministic fixture longer than Discord/Telegram/WhatsApp limits containing astral emoji, combining marks, long German/RTL text, links, inline code, and a fenced block whose language tag crosses a natural split candidate."
      - "For Slack/Discord/Teams/WhatsApp count Unicode code points, Telegram UTF-16 units, and Google Chat UTF-8 bytes; assert every final wire body including markers/fences stays within its cap."
      - "Reconstruct content from ordered numbered chunks after removing transport-only fence repair/indicators; compare losslessly and verify the ACK names the last materialized remote ID."
      - "Force Telegram's typed parse rejection and confirm one plain-text fallback, not a 400 or duplicated prefix; inspect Slack mrkdwn outside and inside code."
    must_avoid:
      - "Expecting 6000 characters to produce exactly three Discord messages; measuring source text instead of the formatted wire body; accepting visual order without request-log order."
  evidence_expectations:
    - "Fixture checksum, per-provider wire bodies and length calculations, ordered request IDs, reconstruction diff, Telegram fallback trace, and final ACK payload."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
