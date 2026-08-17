---
id: APP-runtime-update-managed
area: APP
title: A managed runtime install gets a recommendation, never a write
persona: Dora
journey: J-desktop-update-moment
expected: With a package-manager-owned runtime (e.g. homebrew) and a newer version available, the update surface shows availability plus the exact channel command, no binary is modified anywhere (mtime unchanged), and after the user updates through their channel the surface clears with no residual pending state.
entry_points: Settings → General → Updates in Chrome or the app on a managed-install home; GET /api/settings/update; compozy update --check -o json
qa_status: pass
bug_ids:
fix_status: 
retest_status: 
fix_commits: 
evidence: docs/qa/reports/2026-08-17-electron-shell.md
last_report: docs/qa/reports/2026-08-17-electron-shell.md
overlaps: APP-runtime-update-app-owned
---

PRD stories: US-017 (AC-1 recommendation, zero writes; AC-2 clears after external update; EC-1
inconclusive detection → safe recommendation-only with the stated reason). BR-8
default-to-recommendation. Test ID: E2E-026; IT-018, IT-023; UT-065, UT-066, UT-069, UT-079–
UT-081, UT-112.

Per-OS evidence: the managed branch is walked with a real managed install shape on both release OSes
(macOS Homebrew; Linux package-manager or PATH-install fixture), capturing
the verbatim recommendation string, a binary mtime/hash comparison proving zero writes, and the
cleared surface after the external update. The inconclusive-provenance branch (marker hash
mismatch) is walked on at least one OS. Overlap: APP-runtime-update-app-owned owns the apply
branch (canonical for the shared update surface).
