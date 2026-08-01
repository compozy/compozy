---
id: ET-window-tab-v3-discard
area: ET
title: Reject stale window layouts without compatibility state
persona: Ada
journey: J-agent-manage-window-tabs
expected: Exported v3 layouts round-trip floating stacks, pins, navigation stacks, and daemon-owned return anchors without closed history; a v2 profile or snapshot is rejected with the unsupported-version diagnostic, boot discards pre-tabs arrangement state as documented, and no alias, converter, partial apply, or schema fallback survives.
entry_points: compozy layout export|validate|apply; compozy layout-profile get|put; HTTP and UDS layout endpoints; skills/compozy/references/window-management.md
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-01-window-tabs/agent-01-cli-route-parity.png; /Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260801-085219-264358-lab/qa-artifacts/qa/evidence/layout-v2.json; /Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260801-085219-264358-lab/qa-artifacts/qa/evidence/layout-v3.json
last_report: docs/qa/reports/2026-08-01-window-tabs.md
overlaps: ET-window-manager-layout-recovery; MS-layout-profile-cli-roundtrip
---

Derived from J-agent-manage-window-tabs step 4. Covers ADR-012, TechSpec invariant 13, data
integrity on rejection, and the Greenfield Alpha hard-cut policy.
