---
id: ET-dormant-extension-placement
area: ET
title: Adopt a dormant extension profile placement
persona: Bruno
journey: J-extension-kit-lifecycle
expected: A resource placed in an absent profile stays hidden and appears as dormant in extension detail and workspace hints; creating that named profile removes the hint and publishes the resource there without changing another profile.
entry_points: /marketplace/extension/{entry_id}; workspace detail; compozy profile create; GET /api/extensions/{name}
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-declared-profile-install; MS-repo-profile-layer-adoption
---

Flagged by profiles Task 09. The final profiles QA cycle owns the first walk.
