---
id: ET-web-extension-kit-inventory
area: ET
title: Inspect an extension kit in Marketplace
persona: Bruno
journey: J-extension-kit-lifecycle
expected: Extension detail shows shipped and live resource counts by kind, live badges, bound environment key names, and Network confirmation state from daemon responses without inventing controls or leaking secrets.
entry_points: /marketplace/extension/$entryId?installed_name=$name; /marketplace/extensions; browser extension detail and confirmation dialog
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-critical-runtime-ui-fixes-20260807-225222-371495-lab/qa-artifacts/qa/spec-cycle-trusted-detail.png
last_report: docs/qa/reports/2026-08-07-critical-runtime-ui-fixes.md
overlaps: ET-ext-inventory; ET-web-extension-detail; ET-web-extensions-manage
---

QA impact 2026-08-02: new user-visible panel. Walk published global detail before and after enable,
the confirmation affordance, and a workspace dev overlay where global inventory must stay hidden.

QA impact 2026-08-02: the inventory panel now consumes the canonical change payload and refreshed
kind grouping; re-walk the installed extension detail and capture the rendered panel.
