# CH-palette-global-summon-truth: Prove OS-global hotkeys register honestly and fail without losing the operator's chord

```yaml
charter:
  id: CH-palette-global-summon-truth
  mission: "As Bruno on the desktop shell, summon Compozy from other apps, bind a per-command global hotkey, force registration failures and relaunches, and prove a chord only ever displays active once confirmed — with the previous working binding restored on every failure and the browser told the truth."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-command-os-from-palette
  scenarios: [ET-desktop-global-summon]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Fire the summon chord from another app (window restores + palette opens) and over an open Compozy modal (focuses without executing through it)."
      - "Bind a global chord to an argument-bearing command via `cmd-palette bind --global` — firing it unfocused opens the palette in that command's argument step."
      - "Pre-claim a chord in another app and try to take it — Settings shows 'unavailable — in use by another application', the previous confirmed chord keeps working; quit/relaunch the shell and confirm registrations re-register and re-report."
      - "Open the app in a plain browser — the global section carries the 'requires desktop shell' reason (in Settings only), and the in-app ⌘K chord is unaffected; on macOS check the accessibility callout deep-links to System Settings."
    must_avoid:
      - "In-app chord conflict semantics (CH-palette-keymap-conflict-truth owns them); non-QWERTY layouts beyond reading the recorder copy (recorded limitation, not a bug hunt)."
```

## Selection rationale

Targeted tier, desktop-shell-gated: SI-10 (active only when confirmed; failure restores the
previous binding) is task 09's core honesty rule and only observable on a real shell —
E2E-027..030 are authored but unexecuted, so this session is their first live walk. 30 minutes:
narrow surface, binary observables.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
