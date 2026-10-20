---
id: RT-web-mermaid-diagrams
area: RT
title: Read Mermaid diagrams and their source in session messages
persona: Bruno
journey: J-12
expected: Completed Mermaid fences render as theme-aware diagrams; streaming, invalid syntax, and unavailable renderer chunks retain readable, copyable source without swallowing subsequent Markdown.
entry_points: Session messages; shared StreamMarkdown stories
qa_status: pass
bug_ids: 818
fix_status: in_progress
retest_status: pass
fix_commits:
evidence:
last_report: docs/qa/reports/2026-10-19-issues-818-819-828.md
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

2026-10-20 targeted re-walk: PASS in real Chromium and isolated Electron 44.5.1/macOS against a
live Codex session. The assistant diagram rendered; View source exposed the original fence; following
prose stayed separate. Blocking the renderer request retained readable code. Earlier Storybook probes
covered light/dark, streaming, invalid syntax, directive security, and lazy loading. The Electron
renderer used production security settings and a temporary profile, not the packaged application.
Evidence and teardown are recorded in the issues verification report.
