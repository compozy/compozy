# CH-message-actions-copy-timestamp: Keep assistant output actions focused on transcript use

```yaml
charter:
  id: CH-message-actions-copy-timestamp
  mission: "As Rafa, review a settled assistant response and confirm its toolbar offers only transcript-relevant copy and timestamp information while Goal creation stays in the composer command path."
  mode: charter-with-tour
  persona:
    name: Rafa
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-14
  scenarios: [RT-053]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Hover and keyboard-focus a settled assistant answer; confirm copy and timestamp are present and no Goal action exists."
      - "Copy the markdown source, then use `/goal` from the composer to confirm Goal creation remains available at its intended entry point."
      - "Inspect a streaming turn and a pure tool-work turn to confirm the copy action stays hidden where it is not valid."
    must_avoid:
      - "Reintroducing response-to-Goal prefill behavior or treating historical reports as the current product contract."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report. -->
