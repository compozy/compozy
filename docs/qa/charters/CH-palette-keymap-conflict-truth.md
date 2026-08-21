# CH-palette-keymap-conflict-truth: Abuse the whole-registry keymap and alias surface until a conflict lies

```yaml
charter:
  id: CH-palette-keymap-conflict-truth
  mission: "As Bruno in the Settings shortcut table and the palette, throw hostile input at chords and aliases — conflicting records, bare letters, invalid grammar, rapid re-records, concurrent tabs — to prove every block names its culprit, every overwrite is explicit and atomic, and every surface shows the same effective chord afterward."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-desktop-shell
  scenarios: [ET-web-command-palette-shortcuts]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Record a chord already owned by another command — the block names the owner; overwrite explicitly — the loser is unbound and flagged; reset-one and reset-all restore daemon defaults; re-record the same chord rapidly (double-press, held keys) hunting for half-applied states."
      - "Feed the alias cell garbage: whitespace, 33 chars, emoji, an alias equal to another command's full title, the same alias on two commands — every rejection states the rule inline, and the title-collision case renders both rows with the alias owner first."
      - "Record a bare single letter on a non-exempt command (must reject with the typing-guard reason) and a chord shadowed by a surface-local binding (must warn as shadowed)."
      - "Edit bindings from two tabs concurrently — the later write wins, both tabs converge on the effective keymap without reload, and palette rows, menubar, and cheatsheet all show the surviving chord."
    must_avoid:
      - "Global-hotkey registration states (CH-palette-global-summon-truth owns them); personalization surfaces (CH-palette-make-it-mine owns them)."
```

## Selection rationale

Targeted tier: BR-5/BR-6 (keys are the operator's; daemon owns binding truth), SI-4 (mutations only
through the settings path), and SI-10 (a chord displays active only when confirmed) fail worst
under malformed and racing input — exactly what the Garbage lens produces. The whole-registry
table is new surface area (open id space, ext tier, alias column) shipped by tasks 04–05.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
