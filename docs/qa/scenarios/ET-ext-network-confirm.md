---
id: ET-ext-network-confirm
area: ET
title: Confirm an extension Network requirement
persona: Bruno
journey: J-extension-kit-lifecycle
expected: Enable and update refuse before mutation when Network consent is absent or stale, return the exact candidate digest, and succeed only when retried with that digest without enrolling an execution into Live.
entry_points: compozy extension enable|update --confirm-network-requirement <digest> -o json|jsonl|toon; POST /api/extensions/:name/enable and PUT /api/extensions/:name with confirm_network_digest over HTTP and UDS; compozy__extensions_enable confirm_network_digest; Marketplace extension confirmation dialog; extension dev reload
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-network-participation-hooks; ET-web-extension-detail
---

QA impact 2026-08-02: new lifecycle consent gate. Walk initial enable, changed-digest update refusal,
exact retry, stale retry, unchanged digest, dev-instance reload, true operator/agent actor identity,
and post-restart confirmation truth.
