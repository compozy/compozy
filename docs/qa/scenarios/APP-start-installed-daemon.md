---
id: APP-start-installed-daemon
area: APP
title: Start my installed-but-stopped runtime by opening the app
persona: Dora
journey: J-desktop-attach-daily
expected: Opening the app with the runtime installed but stopped shows visible bounded starting progress and lands in the product UI; it waits for a live recorded daemon before a bounded desktop-owned retry, never starts a conflicting second daemon, and the started runtime survives app quit with `compozy status` healthy.
entry_points: dock/launcher icon with an installed, stopped runtime; compozy app open
qa_status: untested
bug_ids: BUG-20260810-desktop-runtime-stalls; BUG-20260810-initial-boot-window-absent
fix_status: fixed
retest_status: blocked-verify
fix_commits: b415f24b; b3aa3d27; bd610cfa; 02b55a46; f081a1e
evidence: /Users/pedronauck/dev/qa-labs/compozy-desktop-app-release-20260810-110811-513872-lab/qa-artifacts/qa/app-control-product.txt; /Users/pedronauck/dev/qa-labs/compozy-desktop-app-release-20260810-110811-513872-lab/qa-artifacts/qa/quit-contract.txt; /Users/pedronauck/dev/qa-labs/compozy-desktop-startup-diagnostics-20260811-200336-439901-lab/qa-artifacts/qa/runtime-cli-walk.md
last_report: docs/qa/reports/2026-08-11-desktop-startup-diagnostics.md
overlaps:
---

PRD stories: US-004 (start with visible progress; EC-1 start failure → evidence + retry, no loop;
EC-2 slow start bounded; EC-3 simultaneous starters → exactly one runtime). Test IDs: E2E-004
(start half), E2E-011 (bounded honest progress); IT-002, IT-008, IT-024; UT-024, UT-025, UT-027.

Per-OS evidence (N-004): all three OSes record the starting-progress state, the resulting healthy
product UI, and a `compozy status` transcript after quit proving the runtime survived. The
concurrent-starter race (EC-3) and crash-loop bound (IT-008 behavior) are walked on at least one
scripted OS (Linux or Windows) with the process evidence retained; macOS covers the happy start in
the scripted-manual smoke.

QA impact 2026-08-11: startup now waits for the recorded daemon and may retry only a runtime it
has proven desktop-owned. The isolated macOS working-tree walk started daemon PID `89043`, reached
product state with one listener, and kept the daemon healthy after app quit. Packaged progress UI
and Linux artifact evidence remain blocked for the mandatory pre-publish smoke.
