---
id: RT-web-mermaid-diagrams
area: RT
title: Read Mermaid diagrams and their source in session messages
persona: Bruno
journey: J-12
expected: Mermaid fences retain a stable bounded frame while streaming and rendering; completed diagrams fit within half the column and their natural size, with viewport fullscreen, zoom, pan, source and copy actions. Invalid syntax and unavailable chunks retain readable source without swallowing subsequent Markdown.
entry_points: Session messages; shared StreamMarkdown stories
qa_status: pass
bug_ids: 818, 981
fix_status: in_progress
retest_status: pass
fix_commits:
evidence:
last_report: docs/qa/reports/2026-10-27-issue-981-mermaid.md
overlaps: ET-web-session-thread-full-bleed
---

Send a flowchart fence followed by a heading and paragraph. Confirm the complete diagram renders
in light and dark themes, including when switching themes after rendering. Open View source and
copy the original source. The following heading and paragraph must remain separate prose.

During streaming, keep the fence readable as code, including syntactically valid prefixes. Finish
the message and confirm it becomes a diagram. Invalid syntax and a failed renderer-chunk request
must retain code without a blank area or an error panel. Navigate away during rendering and verify
there is no orphaned temporary SVG or late update to another message.

Check that ordinary messages and streaming diagrams do not load Mermaid. Exercise two completed
diagrams together and resize the session pane. Raw HTML, SVG, script, and iframe Markdown remain
blocked. Diagram directives must not enable HTML labels or relax strict security; generated SVG
is displayed as an isolated image, without clickable script callbacks or external resources.

Repeat in the desktop shell. Record browser-rendering evidence separately from the complete
session journey and desktop verification; a passing story alone does not establish those journeys.

Stream several tall top-to-bottom flowcharts, sequence diagrams and wide flowcharts in one response.
The source scrolls inside a reserved frame; completing each fence and decoding its image must not
change that frame's height. Inline diagrams use at most half the column and never exceed their natural
size or the frame's height limit. Check both a narrow pane and a wide desktop, in light and dark themes.
Open Fullscreen from a window smaller than the viewport: its dialog must use the viewport, not the
window bounds. Zoom in/out, drag to inspect clipped details, focus the viewer and pan with all four
arrow keys, reset, and dismiss with Escape/Close.
Reopening starts at 100% and centered; the original inline diagram, source and copy action remain usable.

2026-10-20 targeted re-walk: PASS in real Chromium and isolated Electron 44.5.1/macOS against a
live Codex session. The assistant diagram rendered; View source exposed the original fence; following
prose stayed separate. Blocking the renderer request retained readable code. Earlier Storybook probes
covered light/dark, streaming, invalid syntax, directive security, and lazy loading. The Electron
renderer used production security settings and a temporary profile, not the packaged application.
Evidence and teardown are recorded in the issues verification report.
