---
id: APP-web-update-two-track
area: APP
title: Read and apply the one product update from Settings in a plain browser
persona: Bruno
journey: J-desktop-update-moment
expected: Settings → General → Updates shows CompozyOS as one row with one version, one status, and one Update action, never separate runtime and app rows. Update applies every eligible part in one operation, the row shows "Updating" with a measured percent while it runs, and staged, blocked, and rolled-back outcomes are reported in plain words without ever claiming success the daemon has not reported.
entry_points: Settings → General → Updates in Chrome or the app; GET /api/settings/update; POST /api/settings/update/apply; POST /api/settings/update/cancel; POST /api/settings/update/check over HTTP and UDS
qa_status: untested
bug_ids:
fix_status: 
retest_status: 
fix_commits: 
evidence: 
last_report: docs/qa/reports/2026-08-17-electron-shell.md
overlaps: APP-single-command-multi-target-update; APP-runtime-update-managed; APP-cancel-dormant-update; APP-web-update-indicator
---

Added 2026-08-16 for the Electron shell web update surface (ADR-006 S1). Reworked 2026-10-12: the
daemon still keeps runtime and app tracks and applies them as one durable operation, but the
surface folds them into one product row (`settingsProductUpdate`). The id is kept for history.
Re-walk required.

Test ids: UT-040–UT-048 in `-general.test.tsx` own the DOM contract (including the absence of any
runtime/app naming); `update-presentation.test.ts` owns the per-track projection and the
product fold. E2E-020 in `settings.spec.ts` owns the browser apply journey.

Branches to walk, each against real daemon truth rather than a stubbed payload:

- both parts available, one part available, and both up to date: one "CompozyOS" row each time,
  with `current → latest` only when a newer version is pending;
- managed runtime (package-manager install): the row shows the upgrade command verbatim and **no
  Update control exists in the DOM**, not a disabled one;
- headless host with no desktop app: the same single row;
- apply from a plain browser: the Update button gives way to "Updating · N%" (percent only where
  measured; the daemon phase rides on `data-phase`), the section polls at 2s, and the tab reloads
  by itself once the daemon answers on the new version;
- app closed: the row says the update finishes the next time the app opens, with a working
  "Cancel update" for the dormant operation;
- blocked: another surface holds the lease, the row names the holder in plain words, and no
  optimistic success shows;
- failed: the row says CompozyOS went back to the restored version, "What went wrong" shows the
  daemon error, and the offer returns once the daemon reports a fresh available state;
- refresh failure: the section header carries the failure once while the row keeps last-known
  truth.

Browser/app equivalence is part of the verdict: the same walk must render identically in the desktop
app and in a plain browser, because the SPA carries no desktop awareness.


Discovery acceptance (issue #804): the Updates header shows **Last checked** with the last successful
upstream timestamp and a **Check for updates** action. The action refreshes daemon discovery, then
the shared Settings/menubar query exposes any new offer. Failed checks retain the previous timestamp
and diagnostic. A newer release remains eligible when a previous app release has an archived failure.
