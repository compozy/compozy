# CH-mobile-shell-touch-tier: The shell is operable at phone size without lying about state

```yaml
charter:
  id: CH-mobile-shell-touch-tier
  mission: "As Marina checking Compozy from her phone between meetings, run the Touch Tier Tour through the operator shell at 390×844 — every control reachable with a thumb per the frozen T1–T8 artboard, keyboard-open and landscape stay usable, streams stay readable, and the desktop rendering shows no drift."
  mode: charter-with-tour
  persona:
    name: Marina
    device: phone
    network: wifi-fast
    locale: en-US
  journey: J-operate-desktop-shell
  scenarios: [APP-mobile-touch-tier]
  tour: Touch Tier Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Walk the menubar, dock, and command palette at 390×844: 44px touch floor holds (T1), palette stays top-anchored with 44px rows and the grown results well (T5), compact win-layer reserves the tab-bar height so stream content gains the dock band (T3)."
      - "Open a logs/session stream, scroll it, and open the on-screen keyboard: the layout viewport shrinks (T4), the composer/input stays above the keyboard, and the keyboard-open board in the artboard matches."
      - "Rotate to landscape: the same chrome flexes height — verify the height-flex story, not 44px chrome in landscape (T7 recorded residual)."
      - "Open and dismiss the session rail: it overlays the transcript at ≤760px (T6) and restores desktop docking above the breakpoint."
      - "Trigger the loopback-only strip on a remote tier and check its placement and touch floor (T8); then spot-check desktop >1024px for pixel identity."
      - "Confirm or bounce the recorded residuals: profile-switcher 28px well, log bodies' horizontal scroll wells."
    must_avoid:
      - "The loop visual editor canvas (recorded desktop-only skip)."
      - "Remote-tier capability gating (CH-gateway-paired-operator-surface owns it); pairing/redeem flows (owned by the gateway scenarios)."
  evidence_expectations:
    - "Screenshots per artboard board (portrait, keyboard-open, landscape, palette) compared against `docs/design/opendesign/mobile-surface-truth/mobile-surface-truth-shell-390.html`."
    - "Touch-target measurements on the menubar/dock/palette rows at the tier; keyboard-open and landscape captures."
    - "Desktop spot-check captures proving no drift."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
