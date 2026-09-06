# CH-session-history-navigation: Find and revisit exact work in a long session

```yaml
charter:
  id: CH-session-history-navigation
  mission: "As Théo, locate old instructions and exact tool content in a long session without losing the reading position or live output."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-14
  scenarios: [ET-web-session-transcript-calm-grammar]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Find text outside the loaded tail in a session with at least 3,000 entries; advance matches and inspect the exact expanded tool or reasoning content."
      - "Keep find focused while new output arrives; try no matches, Unicode and a match beyond a truncated output preview."
      - "Hover message-trail previews, jump to an earlier operator message, inspect compressed ticks and return to live output."
      - "Reload and repeat after history compaction; compare the destination and message identity with public transcript/search reads."
    must_avoid:
      - "Treating a turn-level jump as an exact tool-field match, or using synthetic component data as evidence of persisted history navigation."
```
