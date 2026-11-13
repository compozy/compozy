---
id: ET-window-unsplit-four-ways
area: ET
title: Move a window out of a split four ways
persona: Théo
journey: J-organize-tabbed-work
expected: A window inside a split tab moves to its own tab right after the tab it came from, in one layout change, through each of: the window header "Move to new tab", Window › Move to new tab, the `window.unsplit` shortcut once bound, and dragging its header up onto the tab bar; the remaining windows of the source tab fill the space; `compozy window move --placement tab` without a target does the same; a window that is alone in its tab has no "Move to new tab" control.
entry_points: window header Move to new tab; Window › Move to new tab; window.unsplit shortcut; header drag onto the tab bar; compozy window move --placement tab; compozy__window_move placement tab
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-window-manager-layout-gestures; ET-window-seam-resize-keyboard; MS-window-shortcut-arrays-ranges
---

story: An operator who split two windows wants one of them back as its own tab without hunting for a maximize button.

scope: The four unsplit gestures in the Web shell and their CLI equivalent; split creation is ET-window-manager-layout-gestures.

Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build (unique home, port and socket). Open three windows in one tab split as A | B | C. (1) Click "Move to new tab" in B's header: B becomes the tab right after the source tab, A | C stay split. ⌘Z restores the split. (2) Focus C and use Window › Move to new tab. (3) Bind `window.unsplit` in Settings › Shortcuts, focus A in a split and press it. (4) Drag a split window's header up onto the tab bar and drop it: it lands as its own tab at the drop position; Escape mid-drag restores the split. Confirm a single-window tab shows no "Move to new tab". CLI: `compozy window move --workspace <id> --revision <r> --id <w> --desktop <d> --placement tab` moves a split window into its own tab and the Web shell converges.
