---
id: ET-web-loop-editor-topbar
area: ET
title: Loop editor chrome uses shell topbar actions
persona: Bruno
journey: J-06
expected: `/loops/:name/editor` shows a single shell Topbar with breadcrumb on the left (Home › Loops › name › Editor) and Validate / Save layout / Publish plus version/dirty chips on the right via `useTopbarSlot`. There is no secondary identity bar with pencil + "title · Edit". The canvas sub-toolbar (zoom, Graph|DSL, invariant chips) remains below the topbar.
entry_points: web /loops/:name/editor
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: TA-086; ET-web-route-chrome-topbar
---

Added by loop editor topbar chrome collapse. Flag only — retest in the next QA cycle.
