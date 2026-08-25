---
id: ET-electron-session-copy-debug
area: ET
title: Copy Session text and open Electron diagnostics
persona: Bruno
journey: J-operate-desktop-shell
expected: The packaged product window can copy the visible Session message through the OS clipboard because the exact daemon origin is allowed `clipboard-sanitized-write`; unrelated permission requests remain denied. `Ctrl+Opt+I` toggles DevTools for the product window, boot remains non-debuggable, and no remote-debugging endpoint is exposed.
entry_points: packaged Electron product window; SessionThread copy action; `Ctrl+Opt+I`
qa_status: blocked-verify
bug_ids:
fix_status: fixed
retest_status: blocked-verify
fix_commits:
evidence: docs/qa/reports/2026-08-25-eng-145-electron.md
last_report: docs/qa/reports/2026-08-25-eng-145-electron.md
overlaps: RT-053; APP-desktop-native-edit-shortcuts; ET-web-desktop-shell-lifecycle
---

story: As a desktop operator, I can copy a Session answer and intentionally open the product diagnostics console without weakening the shell's other security boundaries.

QA impact 2026-08-25: ENG-145 now covers the Electron permission seam and the product-only `Ctrl+Opt+I` debugging path. The focused packaged E2E passed; a physical macOS walk is still required before this scenario can become `pass`.
