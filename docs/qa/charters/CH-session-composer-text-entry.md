# CH-session-composer-text-entry: Enter an exact session draft without character loss

```yaml
charter:
  id: CH-session-composer-text-entry
  mission: "As Bruno, create a session and write a normal multi-word prompt without losing spaces before deciding whether to send it."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-17
  scenarios: [ET-web-session-composer-text-entry]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Enter from the agent detail New session action, select the agent, and create one durable session without a first prompt."
      - "Type a multi-word draft one key at a time, open and close Next prompt, and compare the visible draft with the intended text."
      - "Refresh, return through the session deep link, and type a second multi-word draft to prove the input remains reliable after remount."
      - "Try leading, repeated, and trailing spaces plus one non-Latin word without sending the draft."
    must_avoid:
      - "Do not send a provider prompt or grade the product in the composer; runtime dispatch belongs to the overlapping runtime scenarios."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
