# CH-archive-projects: Retire and restore a project without losing its work

```yaml
charter:
  id: CH-archive-projects
  mission: "As Bruno, hide a completed project, restore its history, and safely remove a disposable registration."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-retire-workspace
  scenarios: [RT-workspace-overview-command-tab, RT-008]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Archive from the picker; verify exclusion and restore from Settings with the same identity."
      - "Attempt archive with active work and read the named blockers."
      - "Cancel removal, choose Archive instead, and remove only a disposable project; inspect its disk files afterward."
      - "Compare CLI, HTTP, UDS and native-tool results and explicit historical reads."
    must_avoid:
      - "Never use the operator's real daemon or project data."
      - "Do not include pinning or recent ordering."
```
