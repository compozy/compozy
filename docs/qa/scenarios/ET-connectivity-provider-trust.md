---
id: ET-connectivity-provider-trust
area: ET
title: Govern a connectivity provider under extension trust rules
persona: Vera
journey: J-extension-policy-admin
expected: A connectivity.provider manifest builds only with its declared gateway tier scopes, global source, and required service methods; enable and boot re-derive live source and digest, exact confirmation gates provider code, and updates, workspace sources, out-of-role claims, or a second provider fail closed without exposure.
entry_points: compozy extension init --template connectivity-provider-go|connectivity-provider-ts; compozy extension build|validate; extension manifest; POST /api/gateway/providers/{name}/enable; compozy extension secrets set connectivity-tailscale --env TS_AUTHKEY
qa_status: blocked-verify
bug_ids: BUG-20260807-gateway-provider-cause;BUG-20260807-gateway-provider-boot;BUG-20260729-public-extension-sdks-unpublished
fix_status: partial
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/12-private-provider-degraded-status.json;/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/33-provider-restart-rewalk.json;/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/41-extension-template-discovery.json
last_report: docs/qa/reports/2026-08-07-remote-gateway.md
overlaps: ET-extension-manifest-v2-surfaces; ET-extension-code-first-authoring; ET-ext-network-confirm
---

Own third-party authoring, consent, and the real-subprocess protocol walk: initialization and
service negotiation, malformed or unimplemented methods, crash and teardown bounds, output
redaction, digest re-consent, hidden `TS_AUTHKEY` binding, and no Host API addition.

QA impact 2026-08-06: added for remote-gateway Task 03. Flag only; Tasks 08–09 own the walk.

QA walk 2026-08-07: missing provider binding failed closed with an actionable redacted cause, and
restart retained local-only readiness. Real digest consent and verified route establishment remain
blocked without an authorized Tailscale account; the public SDK also lacks the new scaffold API.
