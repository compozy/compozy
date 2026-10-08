---
id: TA-web-automation-detail
area: TA
title: One automation detail page for schedules, events and links
persona: Bruno
journey: J-24
expected: "`/automations/jobs/$jobId` and `/automations/triggers/$triggerId` render one page grammar: crumb Automations / <name>, the sentence as head with a labeled On switch opposite (Turning on…/Turning off… until the daemon answers, previous state + toast on failure, pause line when Off), and a subhead (On a schedule · Project · Next run in X / No next run, or On an event · … · Last ran X). How it works shows Starts / Only if / Does: next 3 runs for schedules (every and at variants), conditions as friendly name + path joined by and, agent message clamped to 3 lines with Show full prompt (event templates as chips), Loop inputs with ← from the event / = always, task Title and For, and the webhook POST path with copy and Show example request. Runs lists the last 10 newest first with the shared labels (Scheduled · Running · Handed off · Completed · Failed · Canceled · Skipped · Missed) and StateGlyph (Handed off = delegated), one drawer open at a time, danger text only inside a failed run's drawer with Set up retries opening the editor at Options, Open session / Open loop run / Open task only when the id exists, and No runs yet otherwise. Run now exists only for schedules (works while Off, Starting… then a Queued run toast; disabled when automations are unavailable; a daemon refusal toasts with no row added). The rail is one card (Details, Schedule or Public link + Security, Reliability, Identity) with Inspect and the CLI hint; Inspect says This is a job/trigger in the daemon's terms and shows Scheduler state or Sample event; the signing secret reads only Set / Not set. config/package sources show the lockbar naming `config.toml` (entry name) or the package, no Edit/Delete, switch and Run now intact. Delete requires the exact name, toasts Deleted <name>. and replaces to `/automations`. Unknown id, other-project and loading states have a way back."
entry_points: web `/automations/jobs/$jobId`; web `/automations/triggers/$triggerId` (row click or deep link)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-web-automations-listing; TA-web-automation-editor; TA-automation-crud-loop-target
---

Replaces `ET-web-trigger-detail-rule-page` (Automations spec task 07; US-010–US-018). The trigger rule page became the one detail grammar for both kinds; the job detail page and its At a glance metrics were removed (next run moved to the subhead and rail, scheduler internals to Inspect). Predecessor history lives in `docs/qa/reports/2026-10-02-untested.md` and `docs/qa/reports/2026-08-15-triggers-ui.md`.

Regression obligations carried over: Delete and Inspect overlays keep distinct identities (BUG-20260815-trigger-detail-duplicate-key); the failed-detail state keeps its return action (BUG-20261003-trigger-error-hides-return); the webhook sample JSON stays valid (BUG-20261003-webhook-sample-invalid-json). Walk Back from detail to confirm the listing restores its previous `start`, query, filters and display mode, plus compact viewport, keyboard/Escape and 200% zoom as the predecessor did. Visual contract: `docs/design/opendesign/automations/automations-detail.html` (detail VC-01…09).
