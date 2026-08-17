# CH-herdr-keyboard-navigation: Reach active work through live shortcuts and nested palette views

```yaml
charter:
  id: CH-herdr-keyboard-navigation
  mission: "As Bruno, keep both hands on the keyboard while cycling work, opening nested palette views, and landing on the intended session with the live cheatsheet telling the truth."
  mode: charter-with-tour
  cycle_tier: targeted
  cycle_role: primary
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-desktop-shell
  scenarios: [ET-editable-shell-shortcuts, ET-keyboard-navigation-actions, ET-live-shortcut-cheatsheet, ET-web-command-palette-shortcuts, ET-web-shell-shortcuts-about-dialogs, ET-palette-nested-views, ET-palette-sessions-view-switch]
  tour: Feature Tour
  time_box_minutes: 60
  adrs: [ADR-003, ADR-004, ADR-006]
  safety_invariants: [10]
  visual_contract: "docs/design/opendesign/herdr-parity/: task_05 VC-07..VC-08 and task_06 VC-01..VC-05"
  guidance:
    must_try:
      - "Open palette and New session from an editable composer, then cycle sessions, workspaces, desktops, attention, and focus history with wrap and a frozen burst order."
      - "Use root Views and ⌘E to enter Sessions, filter every state chip, produce zero matches, widen to all workspaces, exceed the row bound, and land on a closed foreign session."
      - "At three stack levels, type and erase a query, pop one level on empty Backspace, close from depth with Escape, and reopen at root with no result bleed."
      - "Rebind a displayed shortcut, open the cheatsheet from both live bindings, and confirm alternates, compact ranges, unbound rows, and the Settings link refresh immediately."
    must_avoid:
      - "Do not judge a screenshot without its reference pair or treat a Storybook fixture as proof of real workspace switching."
```

<!-- Durable targeted charter; every run debrief belongs in its dated report. -->
