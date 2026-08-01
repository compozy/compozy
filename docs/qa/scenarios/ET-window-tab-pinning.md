---
id: ET-window-tab-pinning
area: ET
title: Keep pinned tabs protected and left-collated
persona: Bruno
journey: J-organize-tabbed-work
expected: Pin and unpin preserve a contiguous pinned prefix across reorder, group, ungroup, reopen, and reload; direct tab close returns the pinned-window error while group close remains explicit and deterministic; pinned tabs remain accessible without color-only state.
entry_points: tab context menu; compozy window pin|unpin|close; compozy__window_pin
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/reports/2026-08-01-window-tabs.md
last_report: docs/qa/reports/2026-08-01-window-tabs.md
overlaps: ET-window-tab-agent-parity; ET-window-tab-close-reopen
---

Derived from J-organize-tabbed-work step 3. Covers functional ordering, accessibility, destructive
error recovery, and TechSpec invariant 15.
