---
id: ET-web-extension-kit-inventory
area: ET
title: Inspect an extension kit in Marketplace
persona: Bruno
journey: J-marketplace-acquisition
expected: Extension detail shows shipped and live resource counts by kind, live badges, bound environment key names, and Network confirmation state from daemon responses without inventing controls or leaking secrets.
entry_points: /marketplace/extension/$entryId?installed_name=$name; Marketplace Extensions Installed Manage
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-ext-inventory; ET-web-extension-detail; ET-web-extensions-manage
---

QA impact 2026-08-02: new user-visible panel. Walk published global detail before and after enable,
the confirmation affordance, and a workspace dev overlay where global inventory must stay hidden.
