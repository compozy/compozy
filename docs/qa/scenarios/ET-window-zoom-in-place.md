---
id: ET-window-zoom-in-place
area: ET
title: Zoom a window in place and keep working around it
persona: Bruno
journey: J-administer-window-manager
expected: Zooming a window whose desktop shows nothing else fills that desktop with its frame in place; zooming a window while another window is visible on its desktop moves the frame to a new desktop right after the current one, the pager gains one desktop, the client follows it, and the other window stays where it was; the new desktop accepts tabs and tiles like any other; dragging another window's head onto the zoomed head folds it into the zoomed frame and the deck stays zoomed; tiling another window to a screen edge on the zoomed desktop shrinks the zoomed island to the free zone and ends the zoom instead of dropping it back to its old rect; zooming a window again while a tiled neighbour is visible never covers the neighbour; unzooming returns the frame to the exact slot it left and removes the desktop the zoom created when it is empty; closing the zoomed frame removes that desktop too; minimizing the zoomed window and restoring it from the dock brings it back zoomed on the same desktop and unzoom still takes it home; closing the zoomed tab of a zoomed deck keeps the deck zoomed; the traffic-light zoom control reports pressed while zoomed; compozy window zoom works without --client and compozy window list shows zoomed per window.
entry_points: web desktop traffic lights; zoom menu Fill; command palette Zoom window; drag to top-center; compozy window zoom; compozy__window_zoom; compozy window list
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits: a1baedd3a
evidence: /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/test-cases/walk-window-manager-zoom-results.json; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/screenshots/03-edge-tile-splits-solo-zoom.png; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/screenshots/04-zoom-lifted-over-neighbor.png; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/screenshots/05-unzoom-returned-home.png; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/screenshots/10-edge-drop-shrinks-zoomed-deck.png; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/screenshots/14-closed-zoomed-releases-desktop.png; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/qa-audit-report.json; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-09-01-window-manager-hardening.md
overlaps: ET-window-manager-layout-gestures; ET-window-tab-deck-lifecycle; RT-desktop-pager-overview; ET-window-manager-public-parity
---

story: As a builder, I can maximize one window to read it without covering the windows I had open,
still bring other windows in as tabs or splits, minimize and restore it, and never end up with an
orphan desktop.

qa-impact: 2026-09-01 zoom is structural again: the zoomed unit becomes the only full-frame island of
a desktop, zooming in place when the desktop shows nothing else and lifting to a fresh regular desktop
otherwise; unzoom and close release the desktop the zoom created; snapshot version 4 with stored
version 3 arrangements migrating on load (former focus owners stay zoomed on their desktop with their
return anchor; layout history resets). Walk: open Tasks alone, zoom → fills the desktop in place, pager
still one desktop; open Settings from the menubar (floats above), drag it to the right screen edge →
Tasks island shrinks to the left half, zoom ends, both visible; zoom Tasks again with Settings tiled →
Tasks moves to a new desktop right after, pager shows two, Settings stays; unzoom → Tasks returns to
the left half, pager back to one; tile Tasks and Settings side by side, zoom Tasks (lifts), open Agents
from the dock (lands on the lifted desktop, floating), drag its head onto the Tasks head → one
Tasks+Agents deck, still zoomed; unzoom → deck returns to the original split slot and the lifted
desktop disappears; re-zoom the deck, open a spare app, drag it to the left screen edge → the deck
island shrinks to the right half, zoom ends, the desktop stays; zoom the deck again, minimize it, click
it in the dock → back zoomed on the same desktop; close its zoomed tab → survivor stays zoomed; close
the survivor → its desktop disappears; run `compozy window zoom --id <tasks>` with no `--client` →
applied and `compozy window list` shows `zoomed=true`.

qa-impact: 2026-09-01 walked on the structural model: A0–A8 pass 11/11 (solo zoom in place, edge tile hands the remaining half, lift beside a tiled neighbour and return, head drop into the lifted deck, unzoom drops the lifted desktop, edge tile shrinks the deck island, minimized tab restores into the zoomed deck, zoomed tab close, CLI zoom without a client, closing the zoomed unit releases its desktop).
