---
id: ET-ext-network-confirm
area: ET
title: Confirm an extension Network requirement
persona: Bruno
journey: J-administer-network-live
expected: Enable and update refuse before mutation when Network consent is absent or stale, return the exact candidate digest, and succeed only when retried with that digest without enrolling an execution into Live.
entry_points: compozy extension enable|update --confirm-network-digest; extension enable/update HTTP and UDS routes; Marketplace extension confirmation dialog
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
exact retry, stale retry, unchanged digest, and post-restart confirmation truth.
