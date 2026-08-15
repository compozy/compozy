# CH-workspaces-command-switcher: Switch workspace identity without losing keyboard context

```yaml
charter:
  id: CH-workspaces-command-switcher
  mission: "As Ada, open the Workspaces overview from its configurable shortcut, move through live workspace and worktree rows, and complete each action with visible focus and canonical scope."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-worktree-management
  scenarios: [RT-workspace-overview-command-tab, RT-worktree-web-create-adopt]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Open from the menubar and shortcut, traverse tiles and worktree rows with arrows, typeahead, Home, End, Tab, and Shift+Tab, then close each keyboard layer with Escape."
      - "Select a ready worktree, adopt one discovered checkout, open New worktree, and use Copy path and Delete only where the daemon state permits them."
      - "Change the live catalog while focus is in the strip and menu; require focus to stay on the nearest surviving identity and return to the workspace menubar trigger after close."
      - "Verify the empty state with Enter and Space and confirm pending, missing, failed, and discovered rows never appear as the current ready scope."
    must_avoid:
      - "Do not infer selection from styling alone; confirm the resulting workspace and worktree scope through a structured public read."
      - "Do not use default Compozy state, ports, or browser sessions."
  coverage:
    tier: targeted
    surfaces: [web-S2, keyboard, settings, CLI]
    hot_spots:
      - "Live list changes must update both DOM focus and the identity activated by the next keypress."
      - "Terminal creation failure must stop Creating and leave the preserved draft retryable."
    adjacent_canary: CH-add-workspace-from-root / J-add-workspace-by-browsing
    expected_evidence: "Screenshots of the strip, worktree menu, empty state, shortcut reference, and create failure, plus structured scope reads."
    exit_criteria: "Every keyboard and pointer action reaches the intended live identity, unsupported actions remain absent, and focus returns to the Workspaces trigger."
```

<!-- Immutable charter: each run's debrief belongs in its dated report. -->
