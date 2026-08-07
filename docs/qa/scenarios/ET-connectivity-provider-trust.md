---
id: ET-connectivity-provider-trust
area: ET
title: Govern a connectivity provider under extension trust rules
persona: Vera
journey: J-extension-policy-admin
expected: A connectivity.provider manifest builds only with its declared gateway tier scopes, global source, and required service methods; enable and boot re-derive live source and digest, exact confirmation gates provider code, and updates, workspace sources, out-of-role claims, or a second provider fail closed without exposure.
entry_points: compozy extension init --template connectivity-provider-go|connectivity-provider-ts; compozy extension build|validate; extension manifest; POST /api/gateway/providers/{name}/enable; compozy extension secrets set connectivity-tailscale --env TS_AUTHKEY
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-extension-manifest-v2-surfaces; ET-extension-code-first-authoring; ET-ext-network-confirm
---

Own third-party authoring, consent, and the real-subprocess protocol walk: initialization and
service negotiation, malformed or unimplemented methods, crash and teardown bounds, output
redaction, digest re-consent, hidden `TS_AUTHKEY` binding, and no Host API addition.

QA impact 2026-08-06: added for remote-gateway Task 03. Flag only; Tasks 08–09 own the walk.
