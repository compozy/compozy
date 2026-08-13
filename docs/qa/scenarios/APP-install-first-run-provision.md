---
id: APP-install-first-run-provision
area: APP
title: Install CompozyOS and reach the product with no prior setup
persona: Lea
journey: J-desktop-first-run
expected: A machine with no runtime goes installer → guided provisioning with visible phases → download, verify, install, and self-start → full product UI; relaunch lands directly in the product with exactly one daemon and `compozy status` healthy. A startup failure keeps the boot window available for redacted diagnostics, local copy, and explicit local export.
entry_points: CompozyOS installer (macOS dmg, Linux package); app first launch
qa_status: untested
bug_ids: BUG-20260810-desktop-dev-shell-crashes; BUG-20260810-desktop-runtime-stalls; BUG-20260810-initial-boot-window-absent; BUG-20260810-boot-controls-unavailable
fix_status: fixed
retest_status: pending
fix_commits: 01a45c49; b415f24b; b3aa3d27; bd610cfa; 02b55a46; f081a1e; working tree
evidence: /Users/pedronauck/dev/qa-labs/compozy-issue-362-runtime-manifest-20260813-004336-922826-lab/qa-artifacts/qa/candidate-preconditions.md; /Users/pedronauck/dev/qa-labs/compozy-issue-362-runtime-manifest-20260813-004336-922826-lab/qa-artifacts/qa/qa-audit-report.md
last_report: docs/qa/reports/2026-08-12-runtime-manifest-verification.md
overlaps:
---

PRD stories: US-001 (all ACs incl. AC-3 signed publisher), US-002 (failure/interrupt recovery,
EC-1 launch race, EC-3 external runtime appears → attach). Test IDs: E2E-001, E2E-002, E2E-020
(reinstall over N-1: single app entry, single uninstall record); IT-003, IT-011, IT-012;
UT-029–UT-038, UT-096–UT-098.

Per-OS evidence (N-004 — verdict requires both shipping OSes; Windows is paused until Trusted
Signing is restored): macOS = scripted-manual smoke (no WebDriver): screen recording of
install→provision→product, Gatekeeper acceptance, `compozy status` transcript. Linux =
tauri-driver script + package install/reinstall transcript. All OSes: process-table capture
proving exactly one daemon, and the offline-first-run retry walk (E2E-002) on at least one OS
with the branch recorded.

QA impact 2026-08-12: release publication now requires the signed candidate runtime manifest to
pass the desktop's exact verifier for every shipping target before upload. The scenario is reset
because installer trust and packaged provisioning must be re-walked with signed candidate macOS
and Linux packages. No signed candidate exists for this branch, so both package walks remain
blocked; the dated report preserves the exact verification steps and prior reports retain their
historical evidence.

QA impact 2026-08-13: the live beta.13 runtime manifest predates canonical feed generation and is
rejected by the packaged app before first-run provisioning. The signed feed repair preserves every
manifest value, canonicalizes and re-signs the exact live version, then verifies the published
bytes. Reset to `untested` for the repaired live-feed macOS and Linux package walks.
