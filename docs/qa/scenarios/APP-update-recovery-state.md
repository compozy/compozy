---
id: APP-update-recovery-state
area: APP
title: A failed update never strands me silently
persona: Dora
journey: J-desktop-update-moment
expected: A forced apply failure leaves the installed app intact and launchable with the failure reported and a manual-download path to the release page; a post-migration runtime boot failure surfaces sticky `recovery_required` with typed error and diagnostics, the old binary is not auto-restarted, and a later fixed signed build clears the state.
entry_points: update surface after a forced apply failure; compozy app status / diagnose in recovery_required
qa_status: blocked-verify
bug_ids: BUG-20260810-healthy-retry-corrupts-state
fix_status: fixed
retest_status: pass
fix_commits: f081a1e
evidence: /Users/pedronauck/dev/qa-labs/compozy-coderabbit-desktop-remediation-20260810-153824-470714-lab/qa-artifacts/qa/app-control-product.json; /Users/pedronauck/dev/qa-labs/compozy-coderabbit-desktop-remediation-20260810-153824-470714-lab/qa-artifacts/qa/platform-capability-blockers.txt
last_report: docs/qa/reports/2026-08-10-desktop-coderabbit-remediation.md
overlaps: APP-agent-cli-app-verbs
---

PRD stories: US-015 (AC-1 failed apply → report + manual path; AC-2 malformed/unreachable feed
visible with last-checked; EC-1 crash-on-new-version detectable, fallback reachable; EC-2 no
downgrade), US-016.EC-1 (runtime apply fails → previous usable + diagnostics). Test IDs: E2E-013,
E2E-025; IT-015, IT-017, IT-029; UT-057, UT-059–UT-063, UT-101, UT-102, UT-114, UT-115.

Forced-failure posture (release rehearsal requirement): after the locked-install-dir failure the
app must remain OS-launchable AND the install path's permissions must be unchanged — never left
clamped to `0700` (or the platform ACL equivalent) by the failed apply.

Per-OS evidence (N-004): E2E-013 runs per OS with the locked-dir fixture — capture the failed-
update report, the opened release page, a fresh OS-level app launch, and the install-path
permission listing before/after. E2E-025 (recovery_required journey) runs scripted on Linux or
Windows with the `compozy app status`/`diagnose` transcripts; macOS records the same readouts in
the scripted-manual smoke. Overlap: APP-agent-cli-app-verbs owns the structured-CLI readout of the
same recovery state.
