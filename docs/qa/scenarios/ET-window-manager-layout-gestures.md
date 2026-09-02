---
id: ET-window-manager-layout-gestures
area: ET
title: Arrange and resize windows through structural pointer gestures
persona: Bruno
journey: J-administer-window-manager
expected: Edge and corner intent uses configured bands and hysteresis; repeated side snap cycles one-half, two-thirds, and one-third; occupied side bands structurally reflow while the occupied center swaps whole units without a modifier — a stacked window swaps as its whole tab frame — and grouping as tabs lives on the deck row and the solo head only, never a body drop; every landed window is separated from its neighbour by exactly the configured gap between tiles — edge-snapped windows included — and the drop preview shows that same landing box; shared seams resize every descendant on both sides, and abutting islands expose one draggable boundary that moves every island edge on that shared line, regardless of split or tile origin; a tiled unit's free edges and corners resize that unit alone — growth stops at the nearest island, and a split member detaches into its own island at the released frame while siblings keep their exact zones; drag-away follows policy for solo windows while a multi-member tab frame drags as one unit from its deck bar (a tiled frame floats out whole, active tab and order preserved); impossible minima adapt to a stack; a focus desktop is released the moment its zoomed window leaves, so no owner-less focus desktop lingers in the pager to reject later window opens; Zoom and unzoom restore exact group/node identity, order, weights, placement, and active stack member when the source is unchanged while preserving later source edits through deterministic fallback; and Escape, pointercancel, lost capture, outside release, or ambiguous stale revision commits nothing.
entry_points: web desktop windows; shared seams; island boundaries; window edges; zoom control; command palette; keyboard shortcuts
qa_status: pass
bug_ids: BUG-20260724-arrange-preset-overlap-reject; BUG-20260724-placement-cycles-unpruned; BUG-20260724-single-gesture-slot-multi-pointer; BUG-20260724-stale-return-anchor-on-desktop-transfer
fix_status: pending
retest_status: pass
fix_commits: a1baedd3a
evidence: /Users/pedronauck/dev/qa-labs/compozy-window-manager-review-fixes-20260902-015422-962467-lab/qa-artifacts/qa/logs/walk-window-manager.jsonl; /Users/pedronauck/dev/qa-labs/compozy-window-manager-review-fixes-20260902-015422-962467-lab/qa-artifacts/qa/screenshots/03-edge-tile-splits-solo-zoom.png; /Users/pedronauck/dev/qa-labs/compozy-window-manager-review-fixes-20260902-015422-962467-lab/qa-artifacts/qa/screenshots/04-zoom-lifted-over-neighbor.png; /Users/pedronauck/dev/qa-labs/compozy-window-manager-review-fixes-20260902-015422-962467-lab/qa-artifacts/qa/screenshots/05-unzoom-returned-home.png; /Users/pedronauck/dev/qa-labs/compozy-window-manager-review-fixes-20260902-015422-962467-lab/qa-artifacts/qa/qa-audit-report.json; /Users/pedronauck/dev/qa-labs/compozy-window-manager-review-fixes-20260902-015422-962467-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-09-01-window-manager-review-fixes.md
overlaps: ET-profile-desktop-restoration; ET-web-window-routing-lifecycle; ET-window-manager-layout-recovery; ET-web-command-palette-shortcuts; ET-web-ui-resilience
---

story: As a builder, I can throw, split, stack, resize, and detach windows with predictable previews and atomic final placement.

scope: Include landscape and portrait viewports, one-to-many descendants, floating clamp and reachable title bars, group-move modifier, top-center zoom, Dock-safe bottom center, reduced motion, and concurrent remote edits during a gesture.

qa-impact: 2026-07-22 replaced fraction heuristics with structural topology, a pure target resolver, and one final semantic command per gesture; 2026-07-23 corrected unzoom to preserve exact structural identity for unchanged sources without overwriting source edits; 2026-07-24 arrangement-fix pass — pointer converts to layer coordinates (snap bands were displaced by the menubar height), overshoot past an edge clamps and keeps the edge armed (Dock bottom-center stays reserved), dragged windows are no longer clamped to the work area during drag (commit still clamps with grab preservation), seams reflow live during drag with one weight-space `layout.resize` on release (delta now normalized by the split's axis span), unzoom rejoins a surviving source stack, and weight comparison tolerates renormalization noise; same-day review rounds: closing or minimizing a zoomed window returns it (and the issuing client) to its source, a focus desktop hosting co-resident windows graduates to a standard desktop instead of being wiped or deleted, reduced motion completes desktop transitions instantly instead of skipping them, and keyboard seam steps now pass through the same geometry and minimum-weight clamps as pointer drag. Flag only; the next QA cycle owns live retesting.

qa-impact: 2026-07-31 added tab reorder, drag-out, drag-merge, insertion affordances, and group
movement semantics. Reset for the window-tabs targeted cycle.

qa-impact: 2026-07-31 drag-UX fix pass — group moves now resolve the same drop zones as solo
windows: tile zones land as one tiled stack via `layout.arrange{stack}` and reactivate the active
tab, occupied centers fold into the target stack, occupied sides splice the frame in as one stack
node (`window.move{move_group, placement, target}` gained structural semantics daemon-side), and
the top-center zoom target lifts the whole frame; only swap stays solo-only (no frame-level
command). `window.zoom` on any stack member now zooms the whole frame to the focus desktop and
unzoom restores the frame (exact tiled residue or original floating-stack slot). The dragged frame
renders at reduced opacity with a slight shrink while moving (scale suppressed under reduced
motion), the released rect holds until the daemon commit settles (no old-position flash on drop),
and an advertised merge drop suppresses the snap preview beneath it. Fix-verification evidence:
gesture smoke vs live daemon (float-hold, group tile, whole-frame zoom+restore) in lab
compozy-window-drag-ux-smoke-20260731-151337-798977-lab/qa-artifacts/qa/evidence. Flag only; this
cycle owns live retesting.

qa-impact: 2026-07-31 deck-drag semantics pass — dragging a tab frame by its deck bar (the strip
outside any tab) is now always a whole-frame gesture, tiled frames included: an empty-space drop
floats the frame out as one unit (`window.move{move_group, floating_rect}` gained tiled-stack
detach daemon-side, order and active tab preserved) instead of detaching the active tab. The
occupied-center "Add to stack" snap target became "Swap windows" without a modifier (the tiling-WM
convention): `window.swap` now exchanges whole units — a stacked window swaps as its whole tab
frame across tiled and floating slots, two members of one frame never swap — and grouping as tabs
happens only via the deck row or a solo head's "Group as tabs" zone, matching the browser
tab-strip contract. The swap modifier still forces a swap over the structural side bands. Flag
only; this cycle owns live retesting.

qa-impact: 2026-08-01 resize-completeness pass — resize now works over any shared boundary and any
free edge, independent of split-vs-tile origin. Abutting island frames project one draggable frame
seam per shared line (transitive along the line so every frame stays a rectangle); its drag commits
one atomic `layout.frame_resize` (new command: multi-group frame rewrite, overlap-rejected,
CLI `compozy layout frame-resize`, tool `compozy__layout_frame_resize`). Tiled units regained
per-edge/corner resize handles on free edges only (shared edges resize through their seam): the
commit is the new `window.resize` (CLI `compozy window resize`, tool `compozy__window_resize`),
which resizes floating rects and solo islands in place and detaches a split member into its own
island at the released frame, siblings keeping their exact zones (middle members split the
remainder into islands). Growth clamps at the nearest island live via rnd max bounds. Flag only;
this cycle owns live retesting.

qa-impact: 2026-08-11 separated semantic ordering from visual layers: tiled windows render below structural seams, while every floating layer renders above them. Reset to prove a covered seam cannot intercept the floating window and an uncovered segment still resizes its siblings.

2026-08-11 retest: passed for layer ordering. A live four-window grid kept tiled windows at layer 1 and shared seams at layer 2; floating the active window raised it to layer 7.
qa-impact: 2026-08-22 window arrangements moved from one document per workspace to one per
(workspace, profile), and every window-manager read and write now names the profile it acts as.
Reset to verify isolation, restoration on switch, and that a workspace still purges every profile's
desks when it is removed.

Profile arrangement isolation, switch restoration, archive, delete, and workspace purge are owned by
ET-profile-desktop-restoration. This row retains gesture geometry, resize, drag, zoom, and cancellation
assertions and links the profile row as its first overlap rather than duplicating that walk.

qa-impact: 2026-09-01 zoom is a per-window `zoomed` flag on the window's own desktop; focus desktops
are gone, so "a focus desktop is released the moment its zoomed window leaves" no longer applies —
the zoomed frame fills the work area over the untouched tree and unzoom reveals it unchanged. Edge
tiles against an existing island now displace that island to the free zone (clean cut) instead of
being refused with `topology.group_overlap`; a corner tile keeps the larger band. Pointer releases
after a mid-drag revision change carry a rebase proof instead of cancelling with `stale-layout`.
Zoom-specific assertions moved to ET-window-zoom-in-place. Reset for the current build.

qa-impact: 2026-09-01 zoom is structural: the zoomed unit is the only full-frame island of its desktop, so an edge tile displaces it to the free zone instead of dropping it back to its old rect, and a zoom beside a visible window lifts to a fresh desktop instead of covering it. Walked A0b, A2, A4 and P4 on the final binary.

qa-impact: 2026-09-01 review remediation keeps stale drag rebases tied to the gesture's captured source node for both snap and free-drop commands, including queued top-center zoom. Reset for a focused stale-gesture walk on the current head.

qa-impact: 2026-09-01 focused current-head walk passed normal top-center zoom, edge snap, lifted-desktop paging, and exact unzoom return through the public web/API/CLI surfaces. The owning hook regressions then changed the live window node mid-gesture and proved free-drop and queued top-center zoom still submit the captured source node and revision.
