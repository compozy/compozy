---
id: APP-brand-channel-visibility
area: APP
title: The app's public identity is CompozyOS with a visible beta channel
persona: Cora
journey: J-desktop-update-moment
expected: Window title, dock/launcher/start-menu identity, installer and uninstall records, and the About/update surface all name CompozyOS; About shows channel beta plus versions; no stable channel selector exists anywhere; the command and machine identifiers remain `compozy`, and no shell screen uses the retired product-language name.
entry_points: installer identity; app window and OS launch surfaces; About/update surface
qa_status: untested
bug_ids: BUG-20260810-desktop-dev-shell-crashes; BUG-20260810-initial-boot-window-absent
fix_status: fixed
retest_status: blocked-verify
fix_commits: 01a45c49; f081a1e
evidence: /Users/pedronauck/dev/qa-labs/compozy-desktop-app-release-20260810-110811-513872-lab/qa-artifacts/qa/platform-capability-blockers.txt
last_report: docs/qa/reports/2026-08-10-desktop-app-release.md
overlaps: ET-compozy-public-brand-navigation
---

PRD stories: US-018 (AC-1 channel + version visible; AC-2 no inactive stable choice), US-001.AC-2
(window branded CompozyOS throughout first run). BR-17/BR-19; ADR-006/ADR-012 (durable
CompozyOS-derived identity; product-language hard cut; `compozy` command identifiers stay).
Test IDs: E2E-016; UT-113 (brand-sweep occurrence gate owns the repo-wide sweep — this scenario is
the desktop-surface spot-check, not a duplicate of that gate).

Per-OS evidence: capture the installed identity on macOS (bundle name/dock) and Linux (`.desktop`
entry), the window title during provisioning and
product states, and the About/update surface showing `beta` + versions with no stable selector.

Dedup note: ET-compozy-public-brand-navigation (canonical) owns the public site/web/CLI/release
brand surfaces; this file owns only the desktop app's durable identity surfaces.
