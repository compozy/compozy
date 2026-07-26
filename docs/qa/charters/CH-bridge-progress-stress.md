# CH-bridge-progress-stress: Stress progress without leaking or flooding the channel

```yaml
charter:
  id: CH-bridge-progress-stress
  mission: "As Maya, stress one tool-heavy bridged turn until throttling, deduplication, routing, redaction, and transcript purity would fail, then confirm mode off is completely silent until the final answer."
  mode: strategy-based
  persona:
    name: Maya
    device: laptop
    network: wifi-slow
    locale: en-US
  journey: J-watch-agent-work-channel
  scenarios: [NB-028, NB-bridge-tool-progress, NB-provider-progress-rendering]
  tour: Garbage Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Run repeated and distinct tools faster than the 1.5s edit window, include one long completion and one failure, and use parallel users/threads; hunt message-per-tool spam, 429 loops, missing terminal states, and cross-thread/cross-user edits."
      - "Embed fake sk-, Bearer, xoxb-, agh_claim_, PKCE-verifier, and *_secret values in tool arguments and descriptor previews; every channel rendering must use [REDACTED] while remaining useful."
      - "Compare default-on Slack/Telegram/Discord, opted-in Teams/GChat, append-only WhatsApp separate grouping, and issue-provider no-side-effect acknowledgements."
      - "Switch progress to off during pending work, finish the turn, and inspect a fresh session transcript: zero progress provider calls after opt-out and zero progress chrome in ACP/session history."
    must_avoid:
      - "Using real credentials; accepting provider request-count evidence without checking the human-visible thread; switching tours mid-session."
  evidence_expectations:
    - "Timestamped fake-provider request log with edit intervals/targets, channel checkpoint at terminal state, mode-off negative call count, and transcript/ACP payload proving purity."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
