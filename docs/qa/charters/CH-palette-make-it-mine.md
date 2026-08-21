# CH-palette-make-it-mine: Abuse the personalization and execution surfaces the operator shapes

```yaml
charter:
  id: CH-palette-make-it-mine
  mission: "As Bruno making the palette his own — pinning, aliasing, teaching it queries, feeding typed arguments — throw garbage and repetition at every input: pasted junk in argument fields, double-pins, held Enter on confirmations, password values that must never resurface, and a reset that must actually forget."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-command-os-from-palette
  scenarios: [ET-palette-action-panel, ET-palette-inline-args-confirmation, ET-palette-personalization-lifecycle]
  tour: Garbage Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Paste hostile input into inline argument fields (10k chars, emoji, type-invalid values, regex metacharacters in queries) — validation blocks with the field message, search never crashes, and Esc always discards drafts clean."
      - "Run a password-arg command, then hunt the value everywhere it must not be: recents, query learning, `personalization show`, rank-signals — only the pre-selection query may exist."
      - "Hold and double-press ⏎ through a destructive confirmation (repeat guard), double-pin from two surfaces quickly (idempotent), and fire a panel action's chord with focus drifted (single fire, no repeat)."
      - "Teach a query→command association, verify the learned boost and deterministic ordering on repeat, then compare a second workspace before and after reset: pins, recents, and learning clear only in the targeted workspace, every read remains workspace-filtered, and the other workspace stays byte-identical; flip the master switch off and prove recording stops while search still works."
    must_avoid:
      - "Chord/alias grammar abuse in the Settings table (CH-palette-keymap-conflict-truth owns it); extension views (isolation and health charters own them)."
```

## Selection rationale

Targeted tier: SI-6 (no argument/password/vault values in personalization) and SI-7 (every row and
read is workspace-scoped, with deletion cleanup) are ADR-003's privacy and isolation boundaries.
BR-9/BR-10 (workspace-scoped daemon state, deterministic total order) and BR-16's repeat guard live
exactly where garbage input and repetition land. 90 minutes because three dense scenarios share one
journey walk; break at the hour.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
