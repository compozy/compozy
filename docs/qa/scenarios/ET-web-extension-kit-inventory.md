---
id: ET-web-extension-kit-inventory
area: ET
title: Inspect an extension kit in Marketplace
persona: Bruno
journey: J-extension-kit-lifecycle
expected: Extension detail shows shipped and live resource counts by kind, live badges, bound environment key names, and Network confirmation state from daemon responses without inventing controls or leaking secrets.
entry_points: /marketplace/extension/$entryId?installed_name=$name; /marketplace/extensions?tab=installed; browser extension detail and confirmation dialog
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/review-kit-inventory-disabled.png; /Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/review-kit-inventory-enabled.png; /Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/ui-capture/review-kit-detail.png; /Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/inventory-before-enable.json; /Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/inventory-enabled.json
last_report: docs/qa/reports/2026-08-03-bundles-removal-review.md
overlaps: ET-ext-inventory; ET-web-extension-detail; ET-web-extensions-manage
---

QA impact 2026-08-02: new user-visible panel. Walk published global detail before and after enable,
the confirmation affordance, and a workspace dev overlay where global inventory must stay hidden.

QA impact 2026-08-02: the inventory panel now consumes the canonical change payload and refreshed
kind grouping; re-walk the installed extension detail and capture the rendered panel.
