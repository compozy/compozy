# CH-palette-sessions-landing-canary: Re-walk the Sessions view and prove session landing stays single-path

```yaml
charter:
  id: CH-palette-sessions-landing-canary
  mission: "As Bruno, re-walk the palette Sessions view end-to-end in an isolated live lab — attention order, truthful chips, globe round-trip, and above all the shared landing semantics (restore, workspace-switch-first, done-seen) — as the adjacent canary proving the registry rewrite regressed nothing the palette shares with the sessions surface."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-desktop-shell
  scenarios: [ET-palette-sessions-view-switch]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "⌘E straight into the Sessions view and the same view via the root Views entry; attention-first order with exact state words; needs-you/working/finished/idle chips with truthful counts; zero-match chip names its filter and clears with one ⌫."
      - "Globe round-trip: palette ↔ sessions sidebar ↔ `compozy config get shell.sessions.scope` all report the same persisted breadth, surviving a second tab."
      - "Land on a closed session (restores), a foreign-workspace session (workspace switches FIRST, then focus, with the switch named), and a done session (finished marker clears on arrival) — and confirm the Ask-agent fallback's session-open uses the same landing behavior (BR-20: one path, not two)."
      - "Confirm the 'showing N of M' note appears only when matches exceed the bound."
    must_avoid:
      - "Settling the fallback scenario itself (CH-palette-catalog-revision-truth owns ET-palette-agent-fallback); skipping the isolated lab — the standing blocked-verify exists precisely because past cycles substituted unit evidence for a live persona walk."
```

## Selection rationale

The cycle's adjacent canary (task_11 requirement 4): session landing / attention semantics — BR-20
deleted the divergent root `jumpToSession` path, so a regression here is exactly what a
palette-scoped walk would miss. This charter also discharges the standing `blocked-verify` debt on
`ET-palette-sessions-view-switch` (spec § High-Level Technical Constraints names it a P8 gate):
two prior cycles recorded blocked-verify for want of an isolated live-daemon lab, so this session's
non-negotiable is running inside one (eng-qa-bootstrap manifest, isolated COMPOZY_HOME).

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
