---
id: RT-gateway-operator-surface-truth
area: RT
title: Read truthful gateway posture from the operator surface
persona: Iris
journey: J-expose-and-pair-gateway
expected: Gateway settings and structured status agree on named exposure modes, desired and observed state, verified addresses, provider health and cause, refusals, and local-only recovery without presenting an unsupported control or plausible dead URL.
entry_points: Web /settings/gateway; compozy gateway status -o json; HTTP/UDS GET /api/gateway/status
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-gateway-local-only-boot; RT-connectivity-provider-route
---

Walk fresh, loading, empty, refused, pending, degraded, live, and reconfirmation-required states.
Provider inventory failure must not look like an empty catalog; a degraded row must keep its cause
visible. Each address belongs to its own observed surface and disappears when verification or
admission is withdrawn.

The settings surface must remain keyboard-operable, screen-reader legible, responsive at the
project's supported viewports, and free of color-only state. Runtime truth wins over extension
claims and optimistic mutations.

