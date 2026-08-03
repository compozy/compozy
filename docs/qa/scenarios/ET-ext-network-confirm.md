---
id: ET-ext-network-confirm
area: ET
title: Confirm an extension Network requirement
persona: Bruno
journey: J-extension-kit-lifecycle
expected: Enable and update refuse before mutation when Network consent is absent or stale, return the exact candidate digest, and succeed only when retried with that digest without enrolling an execution into Live.
entry_points: compozy extension enable|update --confirm-network-requirement <digest> -o json|jsonl|toon; POST /api/extensions/:name/enable and PUT /api/extensions/:name with confirm_network_digest over HTTP and UDS; compozy__extensions_enable confirm_network_digest; Marketplace extension confirmation dialog; extension dev reload
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/enable-missing-consent.txt; /Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/enable-stale-consent.txt; /Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/enable-http-missing-consent.json; /Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/status-after-refusals.json; /Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/review-kit-network-confirmation.png; /Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/status-enabled.json
last_report: docs/qa/reports/2026-08-03-bundles-removal-review.md
overlaps: ET-network-participation-hooks; ET-web-extension-detail
---

QA impact 2026-08-02: new lifecycle consent gate. Walk initial enable, changed-digest update refusal,
exact retry, stale retry, unchanged digest, dev-instance reload, true operator/agent actor identity,
and post-restart confirmation truth.

QA impact 2026-08-02: the Marketplace confirmation dialog and lifecycle rollback changed; re-walk
the refusal, exact-digest retry, and rendered confirmation state.
