# Automations — design notes

One window, **Automations**, replaces the Jobs and Triggers windows. Someone who is not a developer should only need one idea: an automation **starts** on a schedule or when something happens, and then it **does** one thing.

The daemon already treats these as one domain (`internal/automation`, one `automation_runs` table, shared retry, fire-limit and Loop-target structs). Only the web UI and navigation merge. The CLI, the HTTP routes and the `compozy__automation_*` tool IDs keep their names (see *Compatibility* below).

Status: reference set for Pedro's review, before `cy-create-spec`. Iterate on these files. Do not regenerate them.

## The model in one sentence

> **Starts** (on a schedule · when something happens · when another app calls a link) → **Only if** (optional conditions, events only) → **Does** (ask an agent · start a Loop · create a task).

| What the user sees | Daemon entity | Backend values |
| --- | --- | --- |
| Starts **On a schedule** | Job | `schedule.mode` `cron` "Repeats" · `every` "Every…" · `at` "Once" |
| Starts **When something happens** | Trigger | `event` `session.created` · `session.stopped` · `hook.<name>.completed` · `ext.<ext>.<event>` |
| Starts **When another app calls a link** | Trigger | `event` `webhook` (signed POST, write-only secret) |
| Only if… | Trigger `filter` | exact match, AND (`data.stop_reason = error`) |
| Does **Ask an agent** | `target_kind=agent` | `agent_name` + `prompt` (events: Go template over the event) |
| Does **Start a Loop** | `target_kind=loop` | `loop_target {loop_name, inputs, input_mapping}` |
| Does **Create a task** | Job only, `Job.Task` | `task {title, description, owner}` → run shows `delegated` |

`memory.consolidated` is **not** offered. The `memory-removal` spec deletes it.

### Vocabulary (proposed COPY.md + glossary rows)

| Surface word | Replaces | Note |
| --- | --- | --- |
| Automations (dock title, window, palette view, Settings row) | Jobs · Triggers | Closes the "pending owner decision" row in COPY.md:259 / glossary:431 |
| automation (noun) | job · trigger | The canonical nouns stay one step deeper: Inspect, CLI, API |
| Starts / Only if / Does | When / If / Then (trigger detail) · "What should run" / "On this schedule" (job form) | One rule grammar for both |
| On / Off (switch label) | Enabled / Disabled | Same `PATCH {enabled}` |
| Run now | Run now | Schedules only. Triggers have no run-now route |
| Handed off | Delegated (trigger run list) | Already the shared label in `automation-formatters.ts` |

## Shared story

Workspace **checkout-api** (`ws_checkout_api`). Every board uses the same seven automations:

| Id (name) | Starts | Only if | Does | Source | State |
| --- | --- | --- | --- | --- | --- |
| `morning-digest` | Every weekday at 09:00 UTC (`0 9 * * 1-5`) | — | ask **summarizer** | created here | on · next in 14h |
| `nightly-delivery` | Every day at 02:00 UTC (`0 2 * * *`) | — | start Loop **software-delivery** | created here | on · last run failed |
| `dependency-review` | Mondays at 08:00 UTC (`0 8 * * 1`) | — | create task "Review dependency updates" for pool `reviewers` | created here | off |
| `release-checklist` | Every 30 minutes (`every 30m`) | — | ask **release-manager** | from config | on |
| `summarize-failures` | A session stops | stop reason is `error` | ask **summarizer** | created here | on · last ran 2h ago |
| `rerun-delivery` | A session stops | stop reason is `error` | start Loop **software-delivery** | created here | on |
| `deploy-webhook` | Another app calls a link (`/api/webhooks/workspaces/ws_checkout_api/deploy--wbh_abc123`) | action is `deploy` and branch is `main` | ask **deployer** | created here | on · public link live |

Runs reuse the production fixtures: `sess_9f2a1c`, `looprun_8f3a2bce41d07a55`, "Agent summarizer was not available", plus the skip reasons `self_overlap` and `misfire_grace_exceeded`.

## Files

| File | What it shows |
| --- | --- |
| `index.html` | Overview, the model, the file map, and what gets deleted |
| `automations-list.html` | S1 — the listing: Start views, search, filters, Rows/Cards, row switch, empty (with suggestions), filtered empty, loading, unavailable, error |
| `automations-detail.html` | S2 — detail for a schedule (agent), an event (Loop) and a webhook; run list; locked (config); off; delete confirm; missing; loading |
| `automations-editor.html` | S3 — the create/edit dialog: Starts → Only if → Does → Options, live sentence bar, preview, edit-mode locks |
| `automations-surfaces.html` | S4 — dock, palette, Settings › Automation, Loop entry points, Home tile, redirects, persisted layouts |
| `automations.css` / `automations.js` | Domain lane and demo plumbing |

## Anatomy

### Listing (S1)

```
HEAD 48     [zap] Automations ·7                                  [+ New automation]
TOOLBAR 44  (All 7)(Scheduled 4)(On events 3)  ⌕ Search automations  ⌗ Filter   ⟶  [≣|▦]
BODY        rows: [kind well] name · Off/lock · sentence · meta     next run | last ran · switch · ···
FOOT        7 automations · 6 on · next run in 14h
```

- **Views lead.** All / Scheduled / On events is the Start facet. Because it is a view it is not also a filter (rule L3). Webhooks count as events.
- **Filters** keep the production ones: Location (This project / Global), Source (Created here / From config / From package), Status (On / Off). "Does" (Agent / Loop / Task) is new. The free-text Event filter is dropped from the strip: the Start views replace it, and the exact event stays searchable.
- **Row:** the kind glyph sits in the 34px well (`clock-3` · `radio` · `webhook`). `zap` is the app mark only (dock, window glyph, editor head, Settings row), so a row never repeats it. Title = name. Description = the automation as a sentence. Meta = location · last-run truth (`Last run failed` in danger text, never a pill) · owner tag in all-profiles mode. The trail has the stat (*next run* for schedules, *last ran* for events), the On/Off switch, and an overflow menu (Run now (schedules only) · Edit · Turn off · Delete). A row shows a badge only for an exception: `Off`, or the config/package lock.
- **Empty:** "No automations in default yet" plus "An automation runs an agent, a Loop or a task on a schedule, or when something happens." Two neutral starts, *On a schedule* and *When something happens*, preselect the editor's Starts. Suggestions now read **Suggested automations**, still only for workspace scope.

### Detail (S2)

```
HEAD 48     ‹ Automations / morning-digest                 [▶ Run now] [✎ Edit] [···]
LEDE        sentence (20/500) ……………………………………………… On ◯
SUB         [clock] On a schedule · Project checkout-api · Next run in 14h · Updated yesterday
MAIN        How it works (Starts / Only if / Does)  ·  Runs (accordion, last 10)
RAIL 320    one card: Details · Schedule | Public link · Reliability · Identity · foot [Inspect] cli
```

- **One rule card for every kind.**
  - *Starts* holds the schedule (with the next 3 runs inline), the event, or the webhook (path bar, copy, curl disclosure).
  - *Only if* appears only when there are conditions.
  - *Does* holds the agent and prompt (clamped to 3 lines), the Loop with its labeled mapping rows, or the task title and owner.
- The job **"At a glance" metrics are cut.** Next run moves to the subhead and the rail. Last run and success rate are readable from the run list. The scheduler internals (registered, last fire id, missed count) move to Inspect (ui-normie-pass D-09).
- **The run list is the trigger accordion, used for both kinds.** Status = production `StateGlyph` + the shared formatter word in `fg-2` (no tinted plates): Scheduled (queued, neutral) · Running · Handed off (`delegated` glyph, info) · Completed · Failed · Canceled, plus Skipped and Missed (stopped, neutral). The collapsed row keeps the cause muted; danger text appears only in the drawer, which shows the cause and *Open session* / *Open loop run* / *Open task*.

### Editor (S3)

`dialog--lg` (880). Head: `zap` well, eyebrow "Automation", title "New automation" / "Edit automation". A **live sentence bar** sits in the modebar (shape B: sentence · spacer · status pill "Ready" / "Needs a fix").

1. **Name** (mono, `morning-digest`).
2. **Starts.** Three choice cards. The sub-config opens in an inset below the chosen card:
   - schedule: Repeats / Every… / Once, with preset chips and a simple builder (every day, weekdays, chosen days at a time), Custom cron behind "Edit expression", and a readout;
   - event: four event cards plus a hook name or extension/event field;
   - webhook: slug, webhook id, signing secret, and the note "Always global".
3. **Only if** (events and webhooks only, optional): key = value rows, AND.
4. **Does.** Three choice cards. *Create a task* is disabled for events and webhooks, with the hint "Only scheduled automations can create tasks."
5. **Options** (collapsed, with a mono summary): retries, run limit, missed runs (schedules only), and "Turn on after saving".

The footer keeps production: *Show preview* (it swaps the body), the destination statement, Cancel, and **Create automation** / **Save changes**.

**Edit mode** locks Starts and Does with a visible reason: "Can't change after creating — make a new automation instead". This follows the target-kind immutability, and a job can't become a trigger.

## Visual rules (polish pass, 2026-10-07)

| Rule | Value |
| --- | --- |
| Accent at rest | none. The On/Off switch is the production `Switch` (checked = inverted `primary` track, `primary-foreground` knob; unchecked = `indicator`). `ds-core.css` still paints the checked track accent; the lane overrides it |
| Sentence emphasis (rows, cards) | glue `muted`, varying parts (time, agent, Loop, task) `fg-2`; the row title stays the only `fg-strong` line |
| Detail head sentence | 18px / 500 / -0.015em (`--text-detail-h1`); glue `fg-2`, varying parts `fg-strong` |
| Run status | `StateGlyph` + word in `fg-2`. Tone sits on the glyph only. Collapsed rows keep the cause `muted`; danger text only inside the drawer |
| Last-run meta on the row | danger text + glyph for a failed last run only; a skip stays neutral (`subtle`) |
| Start-kind glyphs | `clock-3` schedule · `radio` event · `webhook` link. `zap` = the Automations app mark only |
| Dialog head well | neutral (`dialog__head-icon--neutral`), never accent |
| Callouts | annotation ink only; pinned to their host so they never move product layout |

## Gating (daemon truth)

| Control | Gate |
| --- | --- |
| On/Off switch (row + detail) | `PATCH /api/automation/{jobs\|triggers}/{id}` `{enabled}` — all sources |
| Run now | `POST /api/automation/jobs/{id}/trigger` — **schedules only**; hidden for events/webhooks |
| Edit / Delete | `source = dynamic` only; config/package = On/Off only + dashed lockbar |
| Runs | `GET /api/automation/{jobs\|triggers}/{id}/runs?limit=10` |
| Listing | `GET /api/automation/jobs` + `GET /api/automation/triggers`, merged client-side by name (see *Backend asks*) |
| Next run | `scheduler.next_run_at ?? next_run` (jobs) |
| Last run on the row | **backend ask** — `last_run {status, started_at}` on both list DTOs (proposal, tagged `new · automations`) |
| Signing secret | `webhook_secret_present` — never the value |
| Starts kind / Does kind | immutable after create |
| Create a task | Job only (`Job.Task`) |

## Authorized deltas vs production

| Delta | Authority |
| --- | --- |
| One window, `/automations`, Start views on the toolbar | Pedro 2026-10-07 (this brief) |
| On/Off switch on the row trail (it used to exist only on the detail page) | authorized delta — reference: marketplace Installed rows; same PATCH |
| Row description = the sentence (it used to be a prompt excerpt or a schedule) | authorized delta |
| Trigger detail rail: separate cards → **one** railbox | design-system patterns §03 (rail = one card) |
| Job metrics block removed; scheduler internals → Inspect | ui-normie-pass D-09 |
| `scheduled` tone neutral (was warning in the triggers set) | ui-normie-pass C-04 |
| Run formatter maps `delegated` → `StateGlyph` `delegated` (was `running`) | existing primitive state; "Handed off" is parked work, not running work |
| Run row status = glyph + word, no tinted pill (job history used `Pill` for skip reasons) | production `TriggerRunRow` grammar, now shared |
| Location picker stays implicit (active workspace; webhook forces global) | production |
| "Last run failed" on the row | proposal — needs `last_run` on the list DTOs |

## Compatibility (SD-013 regimes)

| Surface | Regime | Plan |
| --- | --- | --- |
| Web routes `/jobs`, `/jobs/:id`, `/triggers`, `/triggers/:id` | public-ish (deep links, notifications, bookmarks) | redirect for one release to `/automations?start=schedule`, `/automations/jobs/:id`, `/automations?start=event`, `/automations/triggers/:id`; then delete |
| Persisted desktop layouts holding app ids `jobs` / `triggers` | **user state** | stored-layout migration (snapshot v4 → v5) maps both to `automations` and rewrites routes; if both are open, both stay (lossless); ships with the change |
| `?create=loop&loop=` deep link | public | kept; `/automations?create=loop&start=schedule\|event&loop=` |
| CLI `compozy automation jobs\|triggers`, HTTP `/api/automation/*`, `compozy__automation_*` | public | **unchanged**. These are the canonical nouns one step deeper |
| `app-catalog` ids, `os-types` union, dock glyphs, palette views, `systems/os/apps/{jobs,triggers}` | internal | rename together; delete the old modules |

## Delete targets (internal regime)

- `web/src/systems/os/apps/jobs/*` and `web/src/systems/os/apps/triggers/*` → `apps/automations/` (window, catalog location, detail location).
- `automation-job-row` / `automation-trigger-row` → `automation-row`; `automation-job-card` / `automation-trigger-card` → `automation-card`.
- `automation-run-history` + `trigger-detail/trigger-run-list` → `automation-run-list`.
- The job page header and sections in `automation-detail-panel` / `automation-detail-sections` merge into the shared detail (`trigger-detail/*` becomes `automation-detail/*`).
- `automation-job-form` + `automation-trigger-form` → `automation-form` with `starts/` (schedule = cron-builder, schedule-every, schedule-at; event = event-catalog, event-sub-config; webhook) and `does/` (agent-run-step + agent-prompt-step merge, task-run-step, Loop fields). There is one reliability section instead of two.
- `routes/_app/jobs*.tsx` and `triggers*.tsx` become redirect stubs for one release, then are deleted.
- Dock icon ids `jobs` / `triggers`, the `os-dock-icons` clock and bolt glyphs → one `automations` glyph (bolt).

## Cross-surface impact

- **Native tools:** none; the IDs are unchanged. Tool descriptions may say "automation (job)" and "automation (trigger)".
- **Extensibility / hooks / config:** none. `[[automation.jobs]]` / `[[automation.triggers]]` stay. The lockbar cites them.
- **Workspace isolation:** unchanged. The merged listing keeps scope/workspace filters per entity. Wrong-workspace detail → "This automation belongs to another project."
- **Official skill (`skills/compozy/`):** wording only. "Automations (jobs and triggers)" in the operating overview.
- **Web/Docs:** the site docs gain one "Automations" category with two tutorials ("Run an agent every morning" and "React when a session fails"). The old Jobs / Triggers pages redirect.
- **QA scenarios:** the jobs and triggers scenarios in `docs/qa/scenarios/` merge into one automations journey. The two Loop entry points are covered by it.

## Backend asks (the only ones)

1. `last_run {status, started_at}` on `GET /api/automation/jobs` and `/triggers` list items, for the row truth and for sorting by attention.
2. Optional, can come later: a merged `GET /api/automation/items?kind=` so pagination and `total` are honest across both entities. Until then the listing loads both lists (preload limit 50 each) and the footer total is the sum.

## Invented if shown

Run now on an event automation · converting a schedule into an event · task target on events · a timezone picker inside the editor (the zone is the global Settings value) · templates other than the daemon's suggestions · showing the secret value · `memory.consolidated` · a "paused" state other than Off.
