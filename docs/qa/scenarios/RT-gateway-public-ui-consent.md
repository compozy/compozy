---
id: RT-gateway-public-ui-consent
area: RT
title: Consent to public operator access without weakening pairing
persona: Iris
journey: J-expose-and-pair-gateway
expected: Every public operator-UI enable requires fresh concrete consent, cancel leaves it off, only an already-paired device can enter, public pairing remains absent, and revocation replaces the open view with access-ended and no residual data.
entry_points: Web /settings/gateway; public Gateway address; POST /api/gateway/surfaces
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-gateway-paired-device; RT-gateway-browser-stream-reconnect
---

The disclosure must name the operator UI and the full management API, state that pairing is never
minted or redeemed publicly, and require an acknowledgement on every enable. Refresh, cancel,
disable, and re-enable must not preserve consent. An unauthenticated probe must not reveal workspace,
version, route, or device details.

