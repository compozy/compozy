---
id: APP-install-first-run-provision
area: APP
title: Install CompozyOS and reach the product with no prior setup
persona: Lea
journey: J-desktop-first-run
expected: A machine with no runtime goes installer → guided offline provisioning with visible verify, install, and start phases for the bundled lockstep runtime → full product UI; relaunch lands directly in the product with exactly one daemon and `compozy status` healthy. A startup failure keeps the non-interactive boot window available for redacted diagnostics, local copy, and explicit local export.
entry_points: CompozyOS installer (macOS dmg, Linux package); app first launch
qa_status: blocked-verify
bug_ids: BUG-20260810-desktop-dev-shell-crashes; BUG-20260810-desktop-runtime-stalls; BUG-20260810-initial-boot-window-absent; BUG-20260810-boot-controls-unavailable; BUG-20260817-desktop-release-channel-provenance; BUG-20260817-signed-macos-x64-digest-drift; BUG-20260817-desktop-smoke-local-isolation
fix_status: fixed
retest_status: pass
fix_commits: 01a45c49; b415f24b; b3aa3d27; bd610cfa; 02b55a46; f081a1e; 107890a0; c38ba0fa; 94e2ce7; 1a0b52d; 560cd17
evidence: docs/qa/reports/2026-08-17-electron-shell.md; docs/qa/reports/2026-09-03-appimage-static-runtime.md
last_report: docs/qa/reports/2026-09-03-appimage-static-runtime.md
overlaps:
---

PRD stories: US-001 (all ACs incl. AC-3 signed publisher), US-002 (failure/interrupt recovery,
EC-1 launch race, EC-3 external runtime appears → attach). Test IDs: E2E-001, E2E-002, E2E-020
(reinstall over N-1: single app entry, single uninstall record); IT-003, IT-011, IT-012;
UT-029–UT-038, UT-096–UT-098.

Per-OS evidence requires both shipping OSes. macOS records install→provision→product, Gatekeeper
acceptance, and the `compozy status` transcript. Linux uses Playwright `_electron` plus the package
install/reinstall transcript. Both OSes retain a process-table capture
proving exactly one daemon, and an airplane-mode first run (E2E-002) on both release-gate OSes;
the packaged runtime must install without a feed request.

QA impact 2026-08-12: release publication now requires the signed candidate runtime manifest to
pass the desktop's exact verifier for every shipping target before upload. The scenario is reset
because installer trust and packaged provisioning must be re-walked with signed candidate macOS
and Linux packages. No signed candidate exists for this branch, so both package walks remain
blocked; the dated report preserves the exact verification steps and prior reports retain their
historical evidence.

QA impact 2026-08-13: the live beta.13 runtime manifest predates canonical channel generation and is
rejected by the packaged app before first-run provisioning. The signed channel repair preserves every
manifest value, canonicalizes and re-signs the exact live version, then verifies the published
bytes. Reset to `untested` for the repaired live-feed macOS and Linux package walks.

QA result 2026-08-14: the repaired beta.13 channel passed canonical signature verification, then the
beta.16 DMG and AppImage each provisioned from an empty isolated home. The publication job rebuilt,
signed, uploaded, and publicly re-read the beta.16 desktop channel and every referenced payload.

QA impact 2026-08-16: Electron packages the lockstep runtime and verifies its embedded digest before
the first write. Reset for offline bundled provisioning on macOS and Linux; the prior channel-backed
first-run evidence no longer settles this behavior.

QA impact 2026-09-03: the Linux AppImage now ships the static AppImage runtime
(`toolsets.appimage` pinned in `desktop/electron-builder.yml`) instead of the legacy toolset that
linked against libfuse2. Reset because the Linux install-and-launch walk produced its evidence with
the previous runtime. See APP-appimage-fuseless-launch for the host matrix.

QA result 2026-09-03: a focused Linux run observed the static-runtime AppImage verify its bundled
runtime digest, provision `~/.compozy/bin/compozy`, and start one daemon. The run did not reach this
scenario's healthy full-product end state and retained no canonical teardown evidence, so the Linux
leg remains blocked pending a fresh isolated re-walk. Evidence:
docs/qa/reports/2026-09-03-appimage-static-runtime.md. The macOS leg is untouched by the packaging
change and keeps its prior evidence.


QA impact 2026-11-04 — #1257 (macOS command exposure):

- From a desktop-only DMG install in the default home, reach the product, open a fresh
  terminal, and run `command -v compozy` and `compozy --version`. Check that the link in
  `/usr/local/bin` (or the already-on-PATH `~/.local/bin` fallback) targets the stable
  `~/.compozy/bin/compozy`.
- With an unwritable system directory, decline the first-run install offer. Confirm the
  app remains usable; use **CompozyOS → Install compozy Command…** to retry, approve the
  macOS administrator prompt, and verify from a fresh terminal.
- Cancel or fail the macOS administrator prompt. Confirm the result states that installation
  failed or was cancelled and points to the desktop log, rather than asking for the same
  access without acknowledging the failed attempt.
- With only `~/.local/bin` on PATH, make that directory unwritable or block its creation.
  Confirm first launch records the attempt and reports directory access, not a missing PATH.
  While the automatic check is pending, choose the menu action and confirm visible feedback.
- Repeat with fish as the login shell and zsh as the interactive shell. Shell profiles
  must remain unchanged. If the installer directories are absent from PATH, follow the
  reported PATH instruction and retry; do not report success before terminal resolution.
- Repeat with an existing Homebrew/npm/user `compozy` earlier on PATH and with a conflicting
  file or dangling link at the destination. Verify it is left unchanged and reported.
- Upgrade through the app with a running session. Confirm one healthy daemon, an unchanged
  command link, and the upgraded version in a fresh terminal. Relaunch must not repeat the
  first-run prompt. Custom `COMPOZY_HOME` labs and E2E builds must not automatically write
  to host PATH directories.

Focused filesystem/flow checks cover the install decision, idempotency, conflicts, explicit
administrator consent and atomic runtime replacement. The signed DMG/menu/administrator
prompt and live-daemon upgrade walks still require a packaged candidate in CI/release QA;
the focused checks do not establish those results.
