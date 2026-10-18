---
id: APP-deep-link-cold-start
area: APP
title: A CompozyOS link survives a cold start of the app
persona: Théo
journey: J-desktop-link-driven
expected: Activating a CompozyOS link with the app closed launches the app, runs any needed provision/start states, and renders the linked view once ready — the destination is never dropped, and exactly one navigation fires.
entry_points: compozyos://open/<product-path> with the app not running
qa_status: pass
bug_ids:
fix_status: 
retest_status: 
fix_commits: 
evidence: docs/qa/reports/2026-08-17-electron-shell.md
last_report: docs/qa/reports/2026-08-17-electron-shell.md
overlaps:
---

PRD stories: US-010.AC-2 (cold-start link survives), US-010.EC-4 (link preserved through a
runtime-error state), US-009.EC-1 (second launch with link during cold start forwards). Test IDs:
E2E-023; UT-048.

Per-OS evidence (N-004): all three OSes walk one cold-start link into a home whose runtime needs a
start (or provision) step, capturing the intermediate state and the final linked view. macOS is
covered by the scripted-manual smoke (this is one of the two macOS single-instance/link E2Es that
replace the missing IT-020 runner there). Evidence includes the app log line showing exactly one
queued navigation firing after readiness.


CI remediation 2026-10-18: the desktop launch fixture reserves distinct available
ports below the default client ephemeral ranges, matching the Go harness. This
prevents an outbound connection from reclaiming a released port-0 probe before
the detached daemon binds. E2E-015/E2E-025 still covers cold Settings navigation,
hostile deep-link rejection, and CLI navigation with the same startup budget.
