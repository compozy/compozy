---
id: RT-web-session-attention-sort
area: RT
title: Bring sessions needing attention forward without losing selection
persona: Théo
journey: J-respond-to-agent-attention
expected: Choosing Attention first uses daemon ordering to place auth, input, and failed sessions ahead of other work with stable ties, preserves keyboard selection through live transitions, renders unknown honestly, and persists the global sort without overlapping full-section writes.
entry_points: web Sessions catalog sort menu; web session-window sidebar sort menu
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-cross-workspace-needs-you-fixed.png; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-attention-all-quiet-cleared.png; .compozy/tasks/herdr-parity/evidence/visual/task_03
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps: RT-session-attention-catalog
---

Walk keyboard and pointer selection while sessions enter and leave the needs-you class, including
equal timestamps and an unreporting session. Every badge must carry a distinct glyph and accessible
label; color alone is not an acceptable signal.

QA impact 2026-08-16: Task 03 added the daemon-backed attention-first sort and unified badge
dictionary. Flag only; Task 08 owns the real-user walk and evidence.

QA 2026-08-16 Herdr parity: The isolated browser journey, focused attention Playwright lane, and full Web E2E exercised cross-workspace landing, permission resolution, counts, channel suppression, task canary, catalog scope/order, finished presence clearing, and honest quiet/stale states. The lab browser exposed its real notification capability; deterministic granted and denied branches ran in the canonical browser suite.

QA impact 2026-10-16 (#746): In a fresh/unset shell preference, verify Created is selected; the
menu offers Created, Name, Last activity, Attention first. Keep several parents and nested running
children visible, including equal creation timestamps and names differing only by case. While tool
activity and attention badges change, Created keeps creation-descending/ID-descending sibling order;
Name keeps Unicode-lowercase title ascending, then creation/ID descending. Rename a session and
verify only Name may reposition it. Add/remove a session and verify the expected insertion/removal.
Repeat with search (including matching subagents), Next/Previous pages, archive, and All workspaces.
Verify unnamed rows sort as New session and Unicode names İ/i̇ compare identically.
Persist each choice through Settings and CLI config get/set, reload, and verify a second client
receives it. An existing explicit last_activity or attention must survive upgrade unchanged; an
unset preference picks Created without rewriting config. Keep profile/workspace isolation intact.
Targeted SQLite/catalog and rendered-sidebar validation owns the slice receipt; full integration
and E2E gates run only in CI per maintainer instruction.

First-title boundary: start an unnamed session in both stable orders and complete its first turn.
When its automatic title arrives, Name may reposition it once, as for any rename; Created must
retain its position. Include equal accented names ÉCLAIR/éclair in the paginated order walk.
