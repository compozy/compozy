---
id: ET-window-tab-close-reopen
area: ET
title: Close tab scopes and restore the newest entry
persona: Théo
journey: J-organize-tabbed-work
expected: Close scopes `tab`, `group`, `others`, `right`, `left` and `all` remove exactly the intended windows in one command and one closed entry (`others`, `right`, `left`, `all` skip pinned tabs; `group` closes every window of the tab); the tab menu offers Close tab, Close other tabs, Close tabs to the right, Close tabs to the left and Close all tabs, leaving out (not disabling) any with nothing to close, and lists Reopen closed tab only when there is a closed entry; Reopen closed tab (⌘⇧T) restores the newest entry with its IDs, routes and pins: a closed tab at its former bar position, a closed split member back into its split, else beside its former sibling branch while that branch's windows still sit together, else beside its former neighbor, else as a tab after its former tab; closing the last tab of a desktop leaves a New Tab window; minimize no longer exists and `--minimize` closes nothing.
entry_points: tab context menu; window header Close window; ⌘W; ⌘⇧T; compozy window close --scope; compozy window reopen; compozy__window_close|reopen
qa_status: untested
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-01-window-tabs/supervisor-02-recovery-desktop.png
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-window-manager-public-parity; ET-web-window-routing-lifecycle
---

Derived from J-organize-tabbed-work step 4 and its reload aftermath. Covers ADR-013, destructive
scope clarity, history bounds, missing-desktop fallback, and the TechSpec invariant 13 hot spot.

qa-impact: 2026-09-30 shell rail v2 (flat topbar, left dock rail, gutterless tiling, browser-tab deck, light/dark theme). Tab close moved to the 20px close button on browser-style tabs and window close to the quiet Close icon control. Reset to re-walk the close scopes and multi-level reopen from the new controls.

qa-impact: 2026-09-30 shell-rail polish E2E triage — after the focused tab closes, the client's focus falls back to the tab its frame now shows, never to a hidden tab from focus history, so a second Command-W closes the tab on screen (E2E-040). Walk: close two tabs of one deck in a row with Command-W and confirm each closes the visible tab.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

qa-impact: 2026-11-13 window-tabs-split S10. Adds `left` and `all`, split-member reopen, and the retired minimize. Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build, one workspace open in the Web shell. Open five tabs, pin the first. On tab 3 use Close tabs to the right (tabs 4–5 go), Reopen closed tab: both return in place. Close tabs to the left: tab 2 goes, the pinned tab stays. Close all tabs: only the pinned tab remains. Reopen. Split two windows in one tab, close the right one from its header (Close window): the left fills the tab; ⌘⇧T: it returns on the right. Close the last unpinned tab of a second desktop: it shows a New Tab window. Reload and reopen again: entries survive. `compozy window close --id <w> --minimize --revision <r>` prints a deprecation warning and the window stays open.

qa-impact: 2026-11-13 window-tabs-split S10 (review fixes). Bulk close items and Reopen closed tab are absent, not disabled, when they cannot apply. Walk: on the last tab, the menu has no Close tabs to the right; with only pinned tabs besides this one, no Close other tabs; on a fresh desktop with nothing closed, no Reopen closed tab.

qa-impact: 2026-11-13 #1651. A window closed beside a nested split reopens beside the whole branch, not next to one of its windows. Walk: in one tab put A on the left and B over C on the right (drag C to the bottom edge of B); close A from its header; ⌘⇧T: A returns as the left column at its former width with B over C on the right. Repeat after moving C out of the tab: A returns beside B alone.

qa-impact: 2026-11-13 #1653 unsaved agent edits on window commands. An operator close that covers an agent editor holding unsaved edits asks "Discard unsaved changes?" first; other windows close as before. Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build. Open an agent's Instructions page, add a "Things it must never do" item without saving. Press ⌘W: the question shows in the window. Keep editing: the window, focus and the item stay. Close another tab so focus lands on the editor's tab, then ⌘W: it asks again. Discard changes: the window closes; ⌘⇧T reopens it showing the saved content. Make two Agents windows dirty and use Close all tabs: one question reads "Your edits in 2 windows will be lost." and Keep editing closes nothing. A dirty editor next to a running terminal under Close all tabs: the unsaved question comes first, then the terminal question.
