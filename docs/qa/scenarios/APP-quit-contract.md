---
id: APP-quit-contract
area: APP
title: Quitting the app never stops my runtime or agent work
persona: Dora
journey: J-desktop-attach-daily
expected: With agents/sessions working, closing the window ends only the app — the runtime and all in-flight work continue and are verifiable via `compozy status`; the same holds for runtimes the app itself started or provisioned, and after a force-kill the next launch attaches normally.
entry_points: app window close/quit while agent work is in flight; force-kill of the app process
qa_status: blocked-verify
bug_ids: BUG-20260810-desktop-runtime-stalls
fix_status: fixed
retest_status: pass
fix_commits: b415f24b; b3aa3d27; bd610cfa; 02b55a46
evidence: docs/qa/reports/2026-08-17-electron-shell.md; /Users/pedronauck/dev/qa-labs/compozy-native-window-chrome-20260817-190228-135313-lab/qa-artifacts/qa; docs/qa/reports/2026-08-17-native-window-chrome.md
last_report: docs/qa/reports/2026-08-17-native-window-chrome.md
overlaps:
---

PRD stories: US-008 (AC-1/AC-2 quit never stops the runtime, AC-3 CLI stays the one stop surface;
EC-1 OS shutdown/logout; EC-2 force-kill). Business rules BR-1/BR-2. Test IDs: E2E-004 (quit
half); IT-009, IT-010; UT-026, UT-028. US-008.EC-1 is a platform-smoke item (not CI-automatable).

Per-OS evidence (N-004): all three OSes capture `compozy status` + the surviving session after a
normal quit of an app-started daemon, and after a force-kill relaunch. The OS shutdown/logout walk
(EC-1) is recorded per OS in the release platform smoke — evidence is the post-login `compozy
status` transcript and daemon log window showing no app-initiated stop.

2026-08-17 qa-impact: the product window now exposes the operating system's close control inside
the Compozy menubar. Reset so the native close path proves the runtime and active work survive.

2026-08-17 QA: packaged macOS quit/relaunch checks passed with the native close control enabled.
Linux remains blocked until the charter is walked on a Linux desktop environment.
