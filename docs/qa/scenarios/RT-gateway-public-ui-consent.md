---
id: RT-gateway-public-ui-consent
area: RT
title: Consent to public operator access without weakening pairing
persona: Iris
journey: J-expose-and-pair-gateway
expected: Every public operator-UI enable requires fresh concrete consent, cancel leaves it off, only an already-paired device can enter, public pairing remains absent, and revocation replaces the open view with access-ended and no residual data.
entry_points: Web /settings/gateway; public Gateway address; POST /api/gateway/surfaces
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/screenshots/12-public-consent-dialog.png;/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/screenshots/13-public-provider-refusal.png;/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/36-public-operator-consent.json
last_report: docs/qa/reports/2026-08-07-remote-gateway.md
overlaps: RT-gateway-paired-device; RT-gateway-browser-stream-reconnect
---

The disclosure must name the operator UI and the full management API, state that pairing is never
minted or redeemed publicly, and require an acknowledgement on every enable. Refresh, cancel,
disable, and re-enable must not preserve consent. An unauthenticated probe must not reveal workspace,
version, route, or device details.

QA walk 2026-08-07: the dialog named every exposed surface, required fresh acknowledgement, and a
provider-less submit left public UI off with a truthful refusal. Authenticated entry, revocation,
and refresh through a real public address remain blocked without an authorized provider.
