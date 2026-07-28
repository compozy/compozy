# CH-background-session-switch: Keep live sessions visible across workspace switches

```yaml
charter:
  id: CH-background-session-switch
  mission: "As Théo, leave a Cursor/Grok 4.5 session working, switch among workspaces, use active-session badges to find it again, and return to the current transcript without stopping the work."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-11
  scenarios: [RT-workspace-active-session-badge, RT-041, RT-045]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Keep one real provider turn in flight while switching to a second workspace."
      - "Read the exact active-session badge, open the background session from the owning workspace, and compare its transcript."
      - "Open a second session in the second workspace and verify counts remain workspace-scoped."
      - "Let one session finish while away; the count and lifecycle must update without manual repair."
```
