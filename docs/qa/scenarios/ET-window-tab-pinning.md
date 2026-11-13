---
id: ET-window-tab-pinning
area: ET
title: Keep pinned tabs protected and left-collated
persona: Bruno
journey: J-organize-tabbed-work
expected: Pin tab and Unpin tab (tab menu, `compozy window pin|unpin`, `compozy__window_pin` with `pinned`) pin the whole tab; pinned tabs are icon-only and form a contiguous prefix at the left across reorder, split, unsplit, reopen, and reload; a pinned single-window tab has no Close tab item and `close --scope tab` on its last window returns `window_manager_window_pinned`, while a member of a pinned split tab with other members closes and `--scope group` closes the whole tab; bulk closes skip pinned tabs; a pinned tab dropped into another tab takes that tab's pin state and a window moved out of a pinned tab becomes an unpinned tab; pinned tabs remain accessible without color-only state.
entry_points: tab context menu Pin tab/Unpin tab; compozy window pin|unpin|close; compozy__window_pin
qa_status: untested
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

qa-impact: 2026-11-13 window-tabs-split S10. Pins now belong to tabs. Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build, one workspace open in the Web shell. Pin tab 3: it moves left and becomes icon-only. Try to drag an unpinned tab before it: it stays behind the pinned prefix. Right-click the pinned tab: no Close tab. `compozy window close --id <its window> --revision <r>`: `window_manager_window_pinned`. Drag another tab into the pinned tab's content: the split tab stays pinned; close one member from its header: it closes. Move a window out with Move to new tab: the new tab is unpinned. Close other tabs: the pinned tab survives. Reload: pins kept.

qa-impact: 2026-11-13 issue #1659 (tab menu grouping). The bulk closes moved under Close ▸ (Other tabs, Tabs to the right, Tabs to the left, All tabs); "Close other tabs" in the walk above is now Close › Other tabs. Walk: right-click the pinned single-window tab: Unpin tab leads, no Close tab item, and Close › lists only the bulk closes that reach an unpinned tab. Close › Other tabs: the pinned tab survives.
