# BUG-20260906-find-specialized-tool-field: Find lands on a tool but hides the matching field

- **Status:** fixed — input and long output re-walked
- **Impact:** Workflow-Blocked
- **Severity:** Major · **Priority:** P1
- **Persona:** Théo · **Journey:** J-14 search long history
- **Scenario:** ET-web-session-transcript-calm-grammar
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

In the 3,023-entry incident archive, search for `ação λ café` returns the first tool message with field=input. Selecting it loads the oldest page and opens the containing turn and tool body, but the specialized Read renderer displays only its output/filename. The matched query field in the authoritative input stays invisible and CSS.highlights contains no transcript match ranges. Find claims 1 of 1 while the destination omits the searched text.

Expected: reveal the actual matching input/output/error field within the opened tool details, including matches beyond a truncated preview, and retain input focus. Reuse the existing bounded detail payload composition; do not inject DOM text or replace runtime values.

Actual public session: sess-5f467905b4a62303, first Read tool history-unicode, notes/ação-café.md. Search result and browser evidence are retained in the integrated lab. Pending frontend repair and re-walk.

Re-walk: the same retained first tool now reveals Input with query `ação λ café`, the active CSS highlight range contains that exact text at viewport y279, and document focus remains session-find-input. Browser errors=[]; evidence/navigation-hidden-input-verified.json/.png in the integrated lab. Root inspected the screenshot. Fable canonical tool-card/navigation/expanded-renderer/thread selection passed199 assertions and root Turbo typecheck; see memory/task_10-find-field.md.

Long-output follow-up: after the cache-head fix, the 260-line Bash output is rendered with the full matched output section (6815characters), and CSS.highlights contains the exact line251 needle. However the active Range rect is y4276, outside the tool's nested scrolling box/viewport. The find bar incorrectly reads1of1 while the visible viewport shows a different active watch message. The host needs to reveal the range inside nested payload scrolling before aligning the conversation. Evidence/navigation-large-output-rewalk.json/.png; root inspected both. Input-field proof remains valid; no full-output PASS until this is repaired and re-walked.

Final long-output re-walk 12:40 UTC: search payload needle ação λ café lands line251 at viewport y279.49..293.99 inside every clipping ancestor; the payload PRE scrollTop4330.5 exposes the exact active range, conversation chat-view top231.59..593, find field keeps focus,1of1, browser errors=[]. Root inspected navigation-large-output-final.png and matching JSON. Same unchanged260line fixture and actual retained tool; no fake highlights. Fable canonical4suites180tests and rootTurbo typecheck pass, memory/task_10-find-scroll.md.
