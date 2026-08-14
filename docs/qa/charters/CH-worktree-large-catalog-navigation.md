# CH-worktree-large-catalog-navigation: Navigate a crowded workspace without losing actions

```yaml
charter:
  id: CH-worktree-large-catalog-navigation
  mission: "As Bruno, navigate a workspace with many linked worktrees and keep switching and creation actions reachable without the submenu taking over the desktop."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-worktree-management
  scenarios: [RT-worktree-web-nested-navigation, RT-worktree-web-create-adopt]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Open the overview submenu with eighteen worktrees, scroll to both boundaries, and confirm the desktop itself does not move."
      - "Reach the submenu by pointer and keyboard, then return focus with ArrowLeft."
      - "Keep New worktree visible while the catalog scrolls and compare the visible catalog with structured CLI output."
      - "Refresh the desktop and repeat the journey from the normal workspace entry point."
    must_avoid:
      - "Using Storybook or internal state as the session verdict; they are supporting visual evidence only."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report. -->
