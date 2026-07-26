# CH-prune-missing-workspace: Remove a local folder and recover from its ghost

```yaml
charter:
  id: CH-prune-missing-workspace
  mission: "As Bruno, add a temporary local workspace, remove its folder outside AGH, return through the switcher and old permalink, and continue without a ghost workspace."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-prune-missing-workspace
  scenarios: [RT-missing-workspace-pruned]
  tour: Interrupt Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Register the lab-owned temporary folder through the workspace modal and refresh before removal."
      - "Remove the folder through a normal user-accessible filesystem command, not AGH internals."
      - "Return by switcher, refresh, and old permalink; verify Web and one structured list agree."
      - "Confirm another valid workspace remains selected and usable."
```
