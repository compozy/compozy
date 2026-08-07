---
id: ET-window-deck-uniform-tab-width
area: ET
title: Window deck tabs share one uniform slot width with truncated titles
persona: Théo
journey: J-organize-tabbed-work
expected: Every unpinned deck tab renders at the same width regardless of title length — 180px slots that shrink equally down to 96px before the tablist scrolls — with long titles truncated by ellipsis (full title stays in the tab tooltip and window head); pinned tabs stay glyph-only content-sized.
entry_points: window deck tab row (web); Storybook systems-os-components-oswindowdeck VC-01/VC-04/VC-06
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-07-window-deck-uniform-tabs/vc04-density-fixed.png; docs/qa/evidence/2026-08-07-window-deck-uniform-tabs/vc06-pinned-fixed.png; docs/qa/evidence/2026-08-07-window-deck-uniform-tabs/vc01-three-tabs-fixed.png
last_report:
overlaps: ET-window-tab-close-reopen
---

Added 2026-08-07 by the deck uniform-width change: tab width moved off the tab (content-sized,
title-dependent) onto the deck's per-tab slot. Root cause of the old variance: the minimum-width
token used the `--width-*` namespace, which cannot generate Tailwind's `min-w-*` utility, so tabs
were purely content-sized. The token now uses the canonical `--min-width-*` namespace; the slot
carries `w-deck-tab-max` + `min-w-deck-tab`, and the tab fills it.

Walked 2026-08-07 against rendered Storybook visual contracts (MSW-backed): CDP measurement of
VC-04 (7 unpinned tabs, mixed short/long titles) returned 118px for all seven slots with computed
min-width 96px; VC-01 shows three tabs at the 180px cap; VC-06 shows pinned glyph-only tabs staying
compact beside uniform unpinned slots. Structure invariant lives in UT-077
(`os-window-deck.test.tsx`).
