# CH-onboarding-global-skip: Finish onboarding without racing workspace changes

```yaml
charter:
  id: CH-onboarding-global-skip
  mission: "As Lea on first run, finish onboarding in Global scope and prove Skip and Continue remain unavailable while a workspace add or removal is still resolving."
  mode: scenario-based
  persona:
    name: Lea
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-19
  scenarios: [RT-onboarding-skip-to-global]
  tour: Interrupt Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Reach Workspaces with zero projects and finish through Skip."
      - "Begin adding and removing a project, interrupt once, and confirm completion actions stay blocked until the operation settles."
      - "Reload the desktop and confirm Global remains the truthful active scope."
    must_avoid:
      - "Do not seed onboarding state or workspace rows through internal APIs."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
