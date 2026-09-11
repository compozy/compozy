# CH-gateway-paired-operator-surface: The paired operator sees only what the tier can execute

```yaml
charter:
  id: CH-gateway-paired-operator-surface
  mission: "As Iris working from a paired device on the private tier, run the Truthful Surface Tour through the paired operator session — every affordance shown must be one the tier can actually execute, terminal and write controls are absent (never disabled), a deep-linked loopback-only mutation lands in the truthful host-naming state, and local desktop behavior is unchanged."
  mode: charter-with-tour
  persona:
    name: Iris
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-expose-and-pair-gateway
  scenarios: [RT-gateway-paired-device, RT-gateway-operator-surface-truth]
  tour: Truthful Surface Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Pair from the second device (fragment redeem → HttpOnly cookie) and open the operator UI on the private tier: terminal and local task-lifecycle affordances are absent from shell, dock, palette, and nav; settings/extensions show read views without save bars; notification/profile enablement renders read-only states instead of toggles."
      - "Deep-link a settings/extensions write from the paired device: the 403 envelope carries loopback_mutation_required and the shell renders the truthful loopback-only strip naming the machine running CompozyOS — no generic error toast, no retry queue."
      - "Follow a logs/session stream from the paired device: fresh single-use tickets per connect, reconnection re-mints, and the session rail behaves per the touch/compact tier."
      - "On the local desktop host, walk the same surfaces: every affordance that existed before the mobile-surface-truth change is still present and functional (BR-5 regression walk)."
      - "Check the device inventory agrees across web, HTTP/UDS, and compozy device list -o json while paired; then revoke and confirm the access-ended boundary still wins over capability gating."
    must_avoid:
      - "Public-tier/Funnel legs (RT-gateway-public-ui-consent and public ingress own them; blocked-verify pending the authorized TS_AUTHKEY)."
      - "Mobile viewport ergonomics (CH-mobile-shell-touch-tier owns the 390×844 walk)."
  evidence_expectations:
    - "Screenshots of the paired private-tier shell showing absent terminal/write affordances and the truthful loopback-only strip after a deep-linked write."
    - "Local-desktop before/after spot-checks proving no affordance regression."
    - "Device inventory reads across web + structured surfaces; revocation closing live work."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
