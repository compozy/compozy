---
id: APP-update-config-cadence
area: APP
title: Control daemon update checks through the app config contract
persona: Dora
journey: J-desktop-agent-headless
expected: The daemon, not the shell, owns `[app].update_check` and `[app].update_check_interval`; disabling checks produces zero channel requests, changing the bounded interval affects the next daemon cadence without restart, and every config surface reads the same persisted global values.
entry_points: global config.toml [app]; compozy config get|set app.update_check and app.update_check_interval; config HTTP and UDS routes; compozy__config_get|set; configuration docs
qa_status: pass
bug_ids:
fix_status: 
retest_status: 
fix_commits: 
evidence: docs/qa/reports/2026-08-17-electron-shell.md
last_report: docs/qa/reports/2026-08-17-electron-shell.md
overlaps: APP-app-auto-update; APP-web-update-two-track
---

Added 2026-08-16 for the Electron cutover config lifecycle. The walk must prove that the shell does
not read or shadow either key, invalid intervals name the exact key, and sequential writes against
one isolated home converge across CLI, HTTP, UDS, native-tool, file, and live daemon read paths.
