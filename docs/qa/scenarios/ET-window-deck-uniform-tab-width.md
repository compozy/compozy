---
id: ET-window-deck-uniform-tab-width
area: ET
title: Tabs shrink together to fit the tab bar, then scroll with an overflow list
persona: Théo
journey: J-organize-tabbed-work
expected: Every tab is at most 224px wide and shrinks together with the others as tabs are added, the active tab shrinking more slowly so it stays the widest; a tab drops its member count under 160px, its waiting count under 144px, its signal under 120px and its title under 72px (the active tab keeps its close button down to 60px and its title down to 88px); at the 40px icon-only floor tabs stop shrinking and the bar scrolls horizontally with a list of the tabs that do not fit; long titles truncate with an ellipsis and the full title stays in the tooltip; pinned tabs stay icon-only at the left.
entry_points: desktop tab bar; tab tooltip; tab overflow list
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-07-window-deck-uniform-tabs/vc04-density-fixed.png; docs/qa/evidence/2026-08-07-window-deck-uniform-tabs/vc06-pinned-fixed.png; docs/qa/evidence/2026-08-07-window-deck-uniform-tabs/vc01-three-tabs-fixed.png
last_report:
overlaps: ET-window-tab-close-reopen
---

> The notes and walks below up to the 2026-11-13 qa-impact note describe decks, floating windows, snapping or zoom and are history only; walk the 2026-11-13 note.

Added 2026-08-07 by the deck uniform-width change: tab width moved off the tab (content-sized,
title-dependent) onto the deck's per-tab slot. Root cause of the old variance: the minimum-width
token used the `--width-*` namespace, which cannot generate Tailwind's `min-w-*` utility, so tabs
were purely content-sized. That fix used the deck tokens, which were deleted with tabs and splits
(2026-11-13): tab widths now come from `TAB_MAX_WIDTH` (224px) and `TAB_MIN_WIDTH` (40px) in
`web/src/systems/os/lib/tab-width.ts`, which the tab bar applies as each tab plate's width.

Walked 2026-08-07 against rendered Storybook visual contracts (MSW-backed): CDP measurement of
VC-04 (7 unpinned tabs, mixed short/long titles) returned 118px for all seven slots with computed
min-width 96px; VC-01 shows three tabs at the 180px cap; VC-06 shows pinned glyph-only tabs staying
compact beside uniform unpinned slots. Structure invariant lives in UT-077
(`os-window-deck.test.tsx`).

qa-impact: 2026-11-13 window-tabs-split S10. The per-pane deck is gone; the desktop tab bar replaced it with shrink-to-fit widths from `web/src/systems/os/lib/tab-width.ts`. Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build, one workspace open in the Web shell. Open two tabs with long session titles: both render at full width and truncate. Keep opening tabs (⌘T) and watch all tabs narrow together; the active tab stays widest. Continue until tabs are icon-only, then open three more: the bar scrolls and its overflow control lists the hidden tabs; picking one activates it and scrolls it into view. Hover a truncated tab for the full title. Pin one tab: it moves left and stays icon-only.
