# CH-agy-native-session-continuity: Keep a conversation across native runtime replacement

```yaml
charter:
  id: CH-agy-native-session-continuity
  mission: "Continue an existing Antigravity conversation across the standalone-to-CLI route change without losing its history or explicit selections."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-15
  scenarios: [RT-agy-acp-provider]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Create a consented standalone-runtime conversation in the isolated installation before replacing its inherited runtime."
      - "Prompt the same logical conversation after replacement and independently read preserved history and explicit runtime intent."
      - "Distinguish route-change transcript replay from native restoration; restart the CLI-backed conversation and inspect its own recovery."
    must_avoid:
      - "Editing the database, importing native history, deleting provider state, or treating fake-peer coverage as actual upgrade proof."
```
