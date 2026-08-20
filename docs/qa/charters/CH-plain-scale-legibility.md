# CH-plain-scale-legibility: Prove the new type scale and warm ramp stay legible and announced without sight

```yaml
charter:
  id: CH-plain-scale-legibility
  mission: "As Sol, re-walk the desktop shell after the type-scale, warm-ramp, and de-uppercase pass and prove every label, status, and focus ring is still reachable, announced, and legible without color or sharp sight."
  mode: charter-with-tour
  persona:
    name: Sol
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-desktop-shell
  scenarios: [ET-web-geist-wght-medium-510, ET-web-ui-resilience]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Probe the type contract on the running surface: 15px body at 1.55, --text-small-body 13.5px, and the eyebrow as sentence-case 12px/510 — uppercase only where <Eyebrow variant=\"caps\"> was opted into."
      - "Tab the whole shell — dock, menubar, palette, window chrome — and confirm the focus ring still clears 3:1 against the lifted surface ramp on every stop, including rows and inputs whose glaze doubled."
      - "Read status with a screen reader across the touched surfaces: the eyebrow losing uppercase must not have removed a label's meaning, and no state may be carried by tint alone."
      - "Check the pairs the ramp lift put closest together — muted text on elevated, accent on canvas, hairline against the new canvas — and measure rather than eyeball any pair that looks thin."
      - "Confirm reduced-motion still holds after the duration and easing change (fast 120ms, base 180ms, slow 260ms): nothing may pulse or slide when the OS asks it not to."
      - "Watch for the de-mono regression specifically: meta, counters, and ids that lost font-mono must still read as meta and not blend into content."
    must_avoid:
      - "Nav label wording (Connections, Permissions) — ET-web-catalog-navigation and CH-untested-068-operate-desktop-shell-bruno own it."
      - "The session transcript and the composer decision dock — CH-session-calm-transcript and CH-session-permission-dock own those surfaces, including their own a11y reads."
      - "The docs site; its ramp and eyebrow moved too, but it is out of this cycle and gated by --audit-site in codegen-check."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
