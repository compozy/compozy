---
id: APP-brand-channel-visibility
area: APP
title: The app's public identity is CompozyOS with a visible beta channel
persona: Cora
journey: J-desktop-update-moment
expected: Window title, dock/launcher/start-menu identity, installer and uninstall records, and the About/update surface all name CompozyOS; About shows channel beta plus versions; no stable channel selector exists anywhere; the command and machine identifiers remain `compozy`, and no shell screen uses the retired product-language name.
entry_points: installer identity; app window and OS launch surfaces; About/update surface
qa_status: pass
bug_ids: BUG-20260810-desktop-dev-shell-crashes; BUG-20260810-initial-boot-window-absent; BUG-20260817-desktop-release-channel-provenance
fix_status: fixed
retest_status: pass
fix_commits: 01a45c49; f081a1e; 94e2ce7
evidence: docs/qa/reports/2026-08-17-electron-shell.md
last_report: docs/qa/reports/2026-08-17-electron-shell.md
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

## 2026-11-08 brand identity v1.2 slice

Spec `.compozy/tasks/brand-site/` (E2E-003). The identity names do not change; the marks do. Extra
steps for this slice:

1. Dock, launcher, start-menu and installer icons show the blue-to-cyan symbol on a dark rounded tile
   (`desktop/build/icon.icns`, `icon.ico`, `icon.png`); the orange mark appears nowhere.
2. Menubar tray templates (`desktop/build/tray/{idle,working,attention}Template*.png`) render as
   monochrome template images in both macOS appearances.
3. The About/update surface and the first-run window show the new Logo from `@compozy/ui`.

Slice validation: partial. The icon artifacts were converted and inspected
(`/tmp/brand-s8-shots/10-desktop-icon-icns.png`, `11-tray-attention@2x.png`). The web first-run
dialog showed the new symbol beside "Set up CompozyOS". The installed app (dock, About, installer
records) was not walked in this slice. Finding for the brand owner: the icon tile is `rgb(8, 15, 20)`,
a blue-tinted black, while business rule 2 asks for neutral blacks.
