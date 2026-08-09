---
id: RT-gateway-operator-surface-truth
area: RT
title: Read truthful gateway posture from the operator surface
persona: Iris
journey: J-expose-and-pair-gateway
expected: Gateway settings and structured status agree on named exposure modes, desired and observed state, verified addresses, provider health and cause, refusals, and local-only recovery without presenting an unsupported control or plausible dead URL.
entry_points: Web /settings/gateway; compozy gateway status -o json; HTTP/UDS GET /api/gateway/status
qa_status: blocked-verify
bug_ids: BUG-20260807-gateway-live-config-copy;BUG-20260807-gateway-provider-cause;BUG-20260807-gateway-provider-boot
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/screenshots/05-provider-degraded-no-address.png;/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/screenshots/14-audit-no-findings.png
last_report: docs/qa/reports/2026-08-07-remote-gateway.md
overlaps: RT-gateway-local-only-boot; RT-connectivity-provider-route
---

Walk fresh, loading, empty, refused, pending, degraded, live, and reconfirmation-required states.
Provider inventory failure must not look like an empty catalog; a degraded row must keep its cause
visible. Each address belongs to its own observed surface and disappears when verification or
admission is withdrawn.

The settings surface must remain keyboard-operable, screen-reader legible, responsive at the
project's supported viewports, and free of color-only state. Runtime truth wins over extension
claims and optimistic mutations.

QA walk 2026-08-07: Web, CLI, HTTP, and UDS agreed on local-only, degraded, refusal, and remediated
states; three production defects were fixed and re-walked. A truthful live-address presentation
remains blocked because the provider account is unavailable.
