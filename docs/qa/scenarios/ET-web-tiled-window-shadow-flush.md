---
id: ET-web-tiled-window-shadow-flush
area: ET
title: Zoomed and tiled windows sit flush without a clipped cast shadow
persona: Bruno
journey: J-operate-desktop-shell
expected: A zoomed window (green traffic light / window.zoom) fills the focus desktop as a tiled pane and shows wallpaper in the configured gutters with no hard-clipped drop-shadow smear against the dock or desk edges. Split and edge-snapped tiled panes behave the same. Unzooming or opening a floating window restores the sanctioned `--shadow-window` / `--shadow-window-unfocused` cast over the wallpaper.
entry_points: web desktop window traffic-light zoom; Window menu Zoom; command palette Zoom window
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-window-manager-layout-gestures; ET-web-desktop-shell-lifecycle
---

Flagged 2026-08-20: zoomed Home left a hard-edged reddish smear at the bottom-left gutter because tiled chrome still painted `--shadow-window` (≈90px blur) into an 8–10px gap, then `os-desk` overflow and desktop `contain: strict` clipped it. Cast elevation now belongs only to floating frames.
