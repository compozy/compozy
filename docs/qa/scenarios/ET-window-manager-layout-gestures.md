---
id: ET-window-manager-layout-gestures
area: ET
title: Arrange and resize windows through structural pointer gestures
persona: Bruno
journey:
expected: Edge and corner intent uses configured bands and hysteresis; repeated side snap cycles one-half, two-thirds, and one-third; occupied drops structurally reflow; center drops stack; shared seams resize every descendant on both sides; drag-away follows policy; impossible minima adapt to a stack; Zoom and unzoom restore exact group/node identity, order, weights, placement, and active stack member when the source is unchanged while preserving later source edits through deterministic fallback; and Escape, pointercancel, lost capture, outside release, or ambiguous stale revision commits nothing.
entry_points: web desktop windows; shared seams; zoom control; command palette; keyboard shortcuts
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-window-routing-lifecycle; ET-window-manager-layout-recovery; ET-web-command-palette-shortcuts; ET-web-ui-resilience
---

story: As a builder, I can throw, split, stack, resize, and detach windows with predictable previews and atomic final placement.

scope: Include landscape and portrait viewports, one-to-many descendants, floating clamp and reachable title bars, group-move modifier, top-center zoom, Dock-safe bottom center, reduced motion, and concurrent remote edits during a gesture.

qa-impact: 2026-07-22 replaced fraction heuristics with structural topology, a pure target resolver, and one final semantic command per gesture; 2026-07-23 corrected unzoom to preserve exact structural identity for unchanged sources without overwriting source edits; 2026-07-24 arrangement-fix pass — pointer converts to layer coordinates (snap bands were displaced by the menubar height), overshoot past an edge clamps and keeps the edge armed (Dock bottom-center stays reserved), dragged windows are no longer clamped to the work area during drag (commit still clamps with grab preservation), seams reflow live during drag with one weight-space `layout.resize` on release (delta now normalized by the split's axis span), unzoom rejoins a surviving source stack, and weight comparison tolerates renormalization noise; same-day review rounds: closing or minimizing a zoomed window returns it (and the issuing client) to its source, a focus desktop hosting co-resident windows graduates to a standard desktop instead of being wiped or deleted, reduced motion completes desktop transitions instantly instead of skipping them, and keyboard seam steps now pass through the same geometry and minimum-weight clamps as pointer drag. Flag only; the next QA cycle owns live retesting.
