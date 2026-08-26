# CH-terminal-window-tabs-canary: Confirm the new terminal app did not distort the rest of the desktop

```yaml
charter:
  id: CH-terminal-window-tabs-canary
  mission: "As Théo, organize and recover ordinary tabbed desktop work with terminals present but not under test, confirming that adding a multi-instance terminal app and a third badge kind left grouping, dock cycling, closed-entry recovery, and attention counts exactly as they were."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-organize-tabbed-work
  scenarios: [ET-window-tab-deck-lifecycle, ET-window-tab-multi-instance, ET-window-tab-close-reopen, ET-web-window-routing-lifecycle, ET-web-dock-default-window-size]
  tour: Interrupt Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "With two terminals open in the background, group two non-terminal windows into a frame, pin one, reorder, and confirm the deck behaves exactly as the existing tabbed-work charters describe."
      - "Cycle instances from the dock and confirm every app still resolves to one addressable instance and that terminal instances join the cycle by terminal identity without displacing anything."
      - "Check the dock badges while a terminal has a pending request and a session has pending attention; both counts must stay separate and truthful, and neither may absorb the other."
      - "Close a tab and a group, reload the desktop, and reopen both entries newest-first; confirm routes, pins, order, and placement are restored and that terminal windows restore into their own instance rather than a shared one."
      - "Interrupt the reload midway and confirm nothing is left in a half-restored state."
    must_avoid:
      - "Do not test terminal behaviour itself — this is the adjacent canary; any terminal finding belongs to the terminal charters. Do not read implementation state or browser storage as the deciding path; use rendered state and daemon output."
```

## Selection rationale

This is the cycle's one adjacent canary, deliberately outside the terminal domain. The terminal program
registered a new multi-instance app in the desktop app catalog and widened the dock badge kinds from two
to three — both are shared surfaces the window manager owns and no terminal scenario watches. A
regression here would show up as somebody else's bug in a later cycle, which is exactly what a canary is
for. The Interrupt Tour is the right lens because the shared risk is restoration: what the desktop
rebuilds after a reload, and whether a new instance kind confuses it.

## Focus areas

- **Desktop app catalog** — a new multi-instance app joins the registry without changing how existing
  apps are addressed, cycled, or restored.
- **Dock badge projection** — a third badge kind coexists with the existing two; counts stay separate and
  each keeps its own meaning.
- **Closed-entry recovery** — restoration returns original identities, routes, navigation depth, pins,
  order, and placement, with terminal windows restoring as their own instances.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
