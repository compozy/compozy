---
id: APP-update-recovery-state
area: APP
title: A failed update never strands me silently
persona: Dora
journey: J-desktop-update-moment
expected: A forced app apply failure leaves the installed app intact and launchable with the failure reported and a manual-download path; a post-swap runtime health failure restores the previous runtime, archives the operation as `rolled-back` with the typed failure, and allows retry only from a newly verified candidate.
entry_points: update surface after a forced app apply failure; compozy update -o json; update-history.jsonl after a runtime rollback
qa_status: untested
bug_ids: BUG-20260810-healthy-retry-corrupts-state
fix_status: fixed
retest_status: pass
fix_commits: f081a1e
evidence: /Users/pedronauck/dev/qa-labs/compozy-coderabbit-desktop-remediation-20260810-153824-470714-lab/qa-artifacts/qa/app-control-product.json; /Users/pedronauck/dev/qa-labs/compozy-coderabbit-desktop-remediation-20260810-153824-470714-lab/qa-artifacts/qa/platform-capability-blockers.txt
last_report: docs/qa/reports/2026-08-10-desktop-coderabbit-remediation.md
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
OSes and records byte-identical restoration plus the archived `rolled-back` outcome. Overlap:
APP-agent-cli-app-verbs owns the structured CLI readout of the same durable update result.
