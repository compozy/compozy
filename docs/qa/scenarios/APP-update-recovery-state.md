---
id: APP-update-recovery-state
area: APP
title: A failed update never strands me silently
persona: Dora
journey: J-desktop-update-moment
expected: A forced app apply failure leaves the installed app intact and launchable with the failure reported and a manual-download path; a post-swap runtime failure retains the replacement and backup, archives the operation as `failed` with recovery guidance, and preserves migrated user state. A live slow boot remains in progress until ready or actual exit. Recovery to the target version or newer clears historical rollback from the live projection without editing history.
entry_points: update surface after a forced app apply failure; compozy update -o json; update-history.jsonl after a runtime failure
qa_status: pass
bug_ids: BUG-20260810-healthy-retry-corrupts-state
fix_status: fixed
retest_status: pass
fix_commits: f081a1e
evidence: docs/qa/reports/2026-08-17-electron-shell.md
last_report: docs/qa/reports/2026-09-09-issue-559-safe-update-recovery.md
overlaps: APP-agent-cli-app-verbs
---

PRD stories: US-015 (AC-1 failed apply → report + manual path; AC-2 malformed/unreachable channel
visible with last-checked; EC-1 crash-on-new-version detectable, fallback reachable; EC-2 no
downgrade), US-016.EC-1 (runtime apply fails → previous usable + diagnostics). Test IDs: E2E-013,
E2E-025; IT-015, IT-017, IT-029; UT-057, UT-059–UT-063, UT-101, UT-102, UT-114, UT-115.

Forced-failure posture (release rehearsal requirement): after the locked-install-dir failure the
app must remain OS-launchable AND the install path's permissions must be unchanged — never left
clamped to `0700` (or the platform ACL equivalent) by the failed apply.

Per-OS evidence: E2E-017 runs on macOS and Linux with the locked-dir fixture — capture the failed-
update report, the opened release page, a fresh OS-level app launch, and the install-path
permission listing before/after. E2E-018 forces a runtime post-swap health failure on both release
OSes and records replacement retention plus the archived `failed` outcome. Overlap:
APP-agent-cli-app-verbs owns the structured CLI readout of the same durable update result.

Issue 559 re-walk: `docs/qa/reports/2026-09-09-issue-559-safe-update-recovery.md`.

Staged recovery verifies the packaged runtime digest before any transition-client execution. The issue 559 packaged re-walk covers both a valid bundle with failing daemon boot and a tampered bundle that must never execute.


### Bootstrap shutdown coordination

A staged app installer can run before runtime readiness, after bundle verification. Before installer handoff or shell shutdown, the desktop cancels and awaits its bootstrap observer, preserves the detached daemon, and prevents late product-window publication. A later successful daemon startup reconciles superseded `starting` restart observations without changing the current restart operation or restoring an older binary. Verified by the issue #559 shell and daemon boot integration evidence in `../reports/2026-09-09-issue-559-safe-update-recovery.md`.

Clean shell shutdown drains already queued app-state publications before removing the startup marker. Later runtime/update callbacks cannot rewrite the state record or recreate the marker. The existing desktop app-state publisher suite owns the concurrent-publication regression; the packaged bootstrap-quit and relaunch scenarios verify the shutdown integration.
