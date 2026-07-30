# CH-nested-skill-groups: Group workspace skills without leaking them

```yaml
charter:
  id: CH-nested-skill-groups
  mission: "As Ada, create and mutate nested workspace skill groups while proving that every public catalog resolves only runnable leaves from the active workspace."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-offer-runnable-capabilities
  scenarios: [ET-nested-skill-groups]
  tour: Feature Tour
  time_box_minutes: 45
  guidance:
    must_try:
      - "Create an empty group plus nested leaves, then compare CLI, HTTP, UDS, native-tool, prompt-catalog, and resource projections."
      - "Add, edit, and remove one nested leaf without restarting the daemon; the next resolution must expose the current catalog."
      - "Use two workspaces and a same-name declaration in different groups to exercise isolation, precedence, and skill where diagnostics."
    must_avoid:
      - "Treating a directory without SKILL.md as a capability or accepting source inspection as workspace-isolation evidence."
```
