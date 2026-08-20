# CH-palette-view-session-isolation: Prove programmable view sessions never leak across clients or survive their generation

```yaml
charter:
  id: CH-palette-view-session-isolation
  mission: "As Bruno with the shell and a second browser tab attached, open the same programmable extension view in both, race searches, restarts, and slow handlers between them, and prove sessions stay per-client, stale generations never overwrite fresh frames, and a broken program can never take down the palette."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-command-os-from-palette
  scenarios: [ET-extension-palette-contributions, ET-palette-nested-views]
  tour: Multi-Tab Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Open the TS fixture's program view in two attached clients; type different queries in each — neither list ever moves for the other's input; close one palette — the other session survives."
      - "Restart the extension (dev reload and a forced kill) with both sessions open — every session drops, the host reopens fresh, and no ghost frame from the old generation ever lands."
      - "Drive the slow-mode fixture past the soft budget (previous rows + busy), the hard ack (degraded + retry), and three misses (circuit-broken) — throughout, Esc/⌫, the root palette, and a built-in view must keep working in BOTH clients."
      - "While a program view is open, walk the stack contract around it: push/pop with parent state intact, breadcrumb ≤ 3, reopen-at-root; then type fast during a pending round-trip — the echo is instant and a stale controlled-value echo can never revert typing."
      - "In a lab copy of the fixture, replace a handler while its event is in flight and force a patch-revision gap/reconnect — the disposed handler and stale echo drop silently, the host requests a full payload instead of applying the gap, and the current frame wins."
      - "Trigger one host effect, acknowledge it, then reconnect/resync — the effect never repeats; a file-picker result returns only to its matching effect id. Try a dual Action+Handler row, a destructive action without confirmation, and an ungranted daemon call — validation or policy must reject each before extension code gains a host effect."
    must_avoid:
      - "Editing the fixture beyond the provided slow/crash modes; walking the membership/health lifecycle (CH-palette-membership-vs-health owns it)."
```

## Selection rationale

Targeted tier: SI-2 (patch gaps resync), SI-12 (validated typed vocabulary), SI-13 (server-side
session ownership), SI-14/SI-15 (controlled-value counters and handler quarantine), SI-16
(last-good degradation), SI-18 (closed, policy-mediated action/handler boundary), SI-19/SI-20
(causal generations and late-frame disposal), and SI-21 (at-most-once effects) are ADR-007,
ADR-008, and ADR-009's riskiest runtime consequences. Cross-client bleed, stale-frame overwrite,
or replayed effects are invisible to a static render and deadly to trust in third-party views.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
