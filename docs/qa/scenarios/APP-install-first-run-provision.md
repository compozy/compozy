---
id: APP-install-first-run-provision
area: APP
title: Install CompozyOS and reach the product with no prior setup
persona: Lea
journey: J-desktop-first-run
expected: A machine with no runtime goes installer → guided provisioning with visible stages → full product UI; relaunch lands directly in the product with exactly one daemon and `compozy status` healthy.
entry_points: CompozyOS installer (macOS dmg, Linux package); app first launch
qa_status: blocked-verify
bug_ids: BUG-20260810-desktop-dev-shell-crashes; BUG-20260810-desktop-runtime-stalls; BUG-20260810-initial-boot-window-absent; BUG-20260810-boot-controls-unavailable
fix_status: fixed
retest_status: pass
fix_commits: 01a45c49; b415f24b; b3aa3d27; bd610cfa; 02b55a46; f081a1e; working tree
evidence: /Users/pedronauck/dev/qa-labs/compozy-coderabbit-desktop-remediation-20260810-153824-470714-lab/qa-artifacts/qa/boot-retry-blocked.jpeg; /Users/pedronauck/dev/qa-labs/compozy-coderabbit-desktop-remediation-20260810-153824-470714-lab/qa-artifacts/qa/boot-diagnostics-fixed.jpeg; /Users/pedronauck/dev/qa-labs/compozy-coderabbit-desktop-remediation-20260810-153824-470714-lab/qa-artifacts/qa/boot-retry-fixed.jpeg; /Users/pedronauck/dev/qa-labs/compozy-coderabbit-desktop-remediation-20260810-153824-470714-lab/qa-artifacts/qa/platform-capability-blockers.txt
last_report: docs/qa/reports/2026-08-10-desktop-coderabbit-remediation.md
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
