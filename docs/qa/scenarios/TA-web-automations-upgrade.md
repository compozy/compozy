---
id: TA-web-automations-upgrade
area: TA
title: Upgrading from Jobs and Triggers keeps links, desktops and palette pins
persona: Bruno
journey: J-administer-window-manager
expected: "On a CompozyOS home upgraded from v0.3.x: old links redirect with history replace (Back does not bounce) — `/jobs?enabled=true&q=nightly` → `/automations?start=schedule&enabled=true&q=nightly`, `/jobs/<id>` → `/automations/jobs/<id>`, `/triggers?event=session.stopped` → `/automations?start=event&q=session.stopped`, `/triggers/<id>` → `/automations/triggers/<id>`, and the `create=loop` handoffs keep their Loop with the matching `start`. A saved desktop with Jobs and/or Triggers windows (tiled, tabbed in one deck, floating, and recently closed) reopens every one as an Automations window on the rewritten path in the same place; nothing is closed, and the next save stores `automations` only (snapshot v5). Pinned Open Jobs / Open Triggers become one pinned Open Automations with summed usage. During v0.4.0, `compozy window open --app jobs --pathname /jobs/<id>` succeeds on Automations and prints `warning: app \"jobs\" is deprecated and will be removed in v0.5.0; use \"automations\"`; palette routes for `app.open.jobs` and `palette.view.triggers` resolve to the automations ids with one WARN log per id per process."
entry_points: web `/jobs*` and `/triggers*` links; a saved desktop from v0.3.x; command palette pins; `compozy window open --app jobs|triggers`; `POST /api/cmd-palette/commands/app.open.jobs/invoke`
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-web-automations-listing; ET-palette-domain-views
---

New in the Automations spec (task 07; US-032, US-033, US-035.AC-3; ADR-002, ADR-004). Bootstrap the lab from a v0.3.x home (or seed a v4 window-manager snapshot and palette rows naming `jobs`/`triggers`), then upgrade in place; a fresh home cannot prove the migration.

Verify the daemon side through structured reads, not only the browser: `compozy layout get -o json` shows `automations` windows after the first save, the INFO `windowmanager.snapshot_migrated` log appears once, and the palette personalization read shows one Open Automations pin. The v0.5.0 removal is out of scope for this release's walk; the release note names it.
