# Analysis: production-surfaces

Read-only exploration of the slice `production-surfaces` (ordinal `02`) for the research prompt:

> What visual contracts, incumbent grammar, production seams, and canonical data must the loop-legibility design set honor so the six artboards under docs/design/opendesign/loop-legibility/ can be built without inventing states, copy, chrome, or entities? Extract only design-actionable facts for S1 (tasks list), S4 (run default + needs-you), S5 (DAG + roster), and S6 (runs roster).

## Scope

- Slice question: What production chrome, components, layout seams, delete targets, and current copy must the six artboards reuse, restyle, or omit so they stay truthful to the live host and do not invent unsupported controls?
- Primary sources: `web/src/systems/loops/` (run page, runs roster, node inventory, palette, DAG) and `web/src/systems/tasks/` (list, kanban, filters, hierarchy, formatters, detail rail)
- Sources read in full vs. sampled: focus files listed in the dispatch were read in full; adjacent run-page cards, rails, request UI, verb/copy libs, grouping, and list/kanban rows were read in full; stories/tests/hooks sampled for chrome composition only
- Total candidate sources surveyed: ~365 loop files + ~241 task files; working set ~45 production files inside the two system dirs

## Overview

This slice is the live host for four artboards: S1 (tasks list), S4 (loop run default + needs-you), S5 (definition DAG + recent-runs roster), and S6 (workspace runs roster). The production surfaces already own page chrome, section order, status vocabularies, glyphs, filters, verbs, and empty copy. The artboards may restyle those pieces; they must not invent a control, metric, filter, column, or sentence the daemon does not already publish.

The two systems share a host grammar: `ListingPage` / `ListingToolbar` / `ListingRow` for catalogs, `useTopbarSlot` for crumbs + status + actions + overflow, a 320px inspector rail on detail pages, and signal color reserved for state (never node class, never priority). Tasks and loops disagree on a few labels (`Completed` vs kanban `Done`; task `Needs attention` vs loop `Needs Approval` / `Needs you`) — those are incumbent, not typos to “fix” in the artboard.

Adjacent slices own spec text (01) and Open Design CSS (03). This file extracts only what is already shipped in `web/src/systems/{loops,tasks}/`. The operator should treat the section-order list, the closed vocabularies, and the “do not invent” list as hard constraints when drawing S1/S4/S5/S6.

## Mechanisms / Patterns

- **Run-page two-column seam (S4):** `LoopRunPageBody` is `max-w-[1240px] px-9 pt-6 pb-18` (`px-5` below 1080px) with `min-[1080px]:grid-cols-[minmax(0,1fr)_320px]` and `gap-8`. Main column `gap-6.5`; rail is one `rounded-lg border border-line bg-canvas-soft` card. Content scrolls; inspect is a right `Sheet`, not a third column. (`loop-run-page-body.tsx:255-256`)

- **Run-page section order (S4 — what competes today):** Conditional stack, top to bottom: (1) **Needs you** when `status === "needs-approval"` OR any quarantined node OR any request; (2) **Needs attention** (attention-flagged nodes); (3) **Outcome** for `failed | blocked | exhausted | stalled | no-op | canceled`; (4) **Happening now** *only if paused* (parked live card rises above Progress); (5) **Progress** (always); (6) **Fan-out strategy** block (hidden when no model); (7) **Happening now** for non-paused live statuses; (8) **Waiting**; (9) **What happened**; (10) next-note quiet sentence. Rail stack: **Usage** → **Waits & attention** → **About this run** → footer **Inspect** + `digest {7-char}`. (`loop-run-page-body.tsx:257-352`)

- **Run topbar chrome (S4):** Story/host publishes crumbs `Loops → Runs → {loop_name}`, leaf crumb = `run.id`, status = `LoopStatusPill`, actions = `LoopRunControls` (Pause / Pausing… / Resume / Cancel run) + `LoopRunOverflowMenu` (View graph, View definition, Kill run… last). Kill is overflow-only and omitted on a terminal run. One `⋯` menu (DESIGN-LESSONS L12). (`loop-run-page.stories.tsx:62-85`, `loop-run-controls.tsx:33-83`, `loop-run-overflow-menu.tsx:14-64`)

- **Needs you shell (S4):** Neutral `LoopSection` titled **Needs you**, Bell icon, gist `{n} item(s)`. Colour is the warning glyph only. Inner stack: one-at-a-time `LoopRequestQuestionnaire` (`Question i of n` + prev/next) → gate-approval block → one quarantine row per quarantined node. Gate verbs (not request verbs): **Approve & resume**, **Request changes**, **Reject & halt**. Fallback title/body when the `needs_approval` frame has not replayed: “Approve to continue this run?” / “The run parked and asks you first…”. Quarantine CTA: **Open entry**. Micro trail: `needs_approval · {gateId}` or `node_controls.quarantined true · {id}[item] · gen N`. (`loop-run-needs-you-card.tsx:79-228`)

- **Request vs gate vocabularies (S4 — do not merge):** Request kinds `ask | review`; request states `pending | answered | expired | canceled`; request decisions `approve | edit | reject | respond` labeled Approve / Edit and approve / Reject / Respond. Ask submit label is **Submit answer**. Review shows proposed args + `RadioCard` decision bar + optional Note. (`loop-request-vocabulary.ts:17-135`, `loop-request-card.tsx:145-227`)

- **Happening now (S4):** Card exists only for `running | watching | paused`, or `queued` when `concurrency === "queue"`. Labels: “Working on {label}” / “Round N in progress” / “Watching for the next event” / “Paused” / “Waiting for a slot”. Dots: accent+pulse (running), info+pulse (watching), hollow neutral (paused), solid neutral (queued). Outbound links only: **Open session**, **View child run**, **View task run**. Lifecycle lines inside the same card are retrying / paused / canceling only — waiting and quarantined are explicitly excluded (they live in Waiting / Needs you). (`loop-run-page-view.ts:46,221-223`, `loop-run-now-card.tsx:48-116,200-218`, `loop-node-now-view.ts:11-16,132-157`)

- **Progress + usage truth (S4):** Progress title = `contract.goal`; body = `definition_of_done`. Segment bar appears only when the latest generation has a fan-out with >1 `item_index`; otherwise left meta is `Round N`. Segment states: `clean | active | redo | failed | parked | quarantined | idle`. Parked branches leave the counted denominator. Usage rows are exactly four: Time, Tokens, Cost (`~$x.xx` estimate, never a cap), Rounds — warn at 90%, danger at/over ceiling; unreported tokens render “not reported”, not `$0`. (`loop-run-progress.ts:7-24,153-200`, `loop-run-usage.ts:93-153`)

- **Outcome copy (S4):** Section titles **What went wrong** (failed) or **Why it stopped** (blocked / exhausted / stalled / canceled). Canceled vs kill share status `canceled`; only terminal-frame `cause === "operator_kill"` flips the title to “Killed by you”. `no-op` is a quiet note: “Ran, nothing to do — the run finished without changes.” Recovery CTA when allowed: **Start a new run**. (`loop-run-outcome-card.tsx:59-237`, `loop-run-page-body.tsx:48-57`)

- **Run verbs (S4 — closed set):** Run: `pause` only from `running` (not while `pause_requested`), `resume` only from `paused`, `cancel` + `kill` on any live run; empty on terminal. Node: pause / three resume modes / resume-wait / cancel / kill / requeue / open-quarantine / amend / rerun — gated by daemon state. Promoted row buttons: **Resume**, **Resume with payload…**, **Requeue…**. Menu labels carry ellipsis when a confirm/form opens. (`loop-node-controls.ts:16-137,150-166`, `loop-node-row-actions.tsx:22-78`)

- **Loop status chips (S4/S5/S6):** Closed daemon set with labels/tones: Queued (neutral), Running (accent, pulse), Watching (info, pulse), Needs Approval (info), Paused (neutral), Done (success), No-op (neutral), Blocked (warning), Failed (danger), Exhausted (warning), Stalled (neutral), Canceled (neutral). Unknown → neutral “Unknown”. Never coerce a missing status into another chip. (`loop-formatters.ts:12-84`, `loop-status-pill.tsx:11-23`)

- **Definition DAG (S5):** Lives on the **loop detail** page, not the run page. `LoopBodyDag` is a horizontal topological spine of 124px (`w-31`) cards: class glyph (muted), node id, `nodeClassLabel` (`action` / `control` / `source` / `control · fan-out|route|ask|gate`), kind or `routeSummary`/`fanOutSummary`. Empty: “This Loop exposes no readable body graph.” Footer: “Read-only view. Open the builder to fork and edit this graph.” Colour is state-only; this view has no live state, so cards stay tint/line. (`loop-body-dag.tsx:12-94`, `loop-detail.tsx:186-218`)

- **Palette / kind glyphs (S5):** Three groups — Action (`run-agent`, `goal`, `run-loop`, `transform`, `compozy__network_send`, blank ToolID), Control (`fan-out`, `collect`, `branch`, `gate`, `sub-loop`, `wait`, `ask`, `route`), Source (`watch-source`, `watch-events`, `file-import`, `input`). Kind icons in `LOOP_NODE_KIND_ICONS`; DAG uses class icons (`Bot` action, `LogIn` source, `Split`/`CheckCheck`/`Signpost`/`MessageCircleQuestion`/`GitMerge` control). Open ToolIDs have no per-tool mark — wrench / class glyph only. Delete target in palette comments: never author retired `retry.max`; seed `retry.max_attempts`. (`loop-palette.ts:23-233`, `loop-node-kind-icons.ts:29-90`)

- **Definition page chrome + recent-runs roster (S5):** Topbar crumb = loop name; primary **Run loop**; overflow Edit / Fork & edit, Configure, Delete loop (workspace-writable only). Lede: `text-detail-h1` name, pills `{source} · v{n}`, meta `{apiVersion} · {category} · {n} nodes`. Body grid `1fr / var(--width-detail-inspector-inline)` via `PAGE_CONTENT_GUTTER`. Recent runs rows: status pill, run id, origin line + start-kind icon, `{n} gens`, best (`Gen N · 0.00` or —), clock duration. Empty: “This Loop has not run yet.” Link **All runs** → `/loop-runs`. (`loop-detail.tsx:102-237`, `loop-recent-runs.tsx:24-83`, `loop-generation-presentation.ts:25-32`)

- **Workspace runs roster (S6):** `LoopRunsView` = four KPIs then **Active** / **Past** tables. KPI set is closed: **Active now** (accent, pulse), **Awaiting you** (warning), **Done today** (success), **Needs a look** (warning). Live KPI statuses: `running | watching | needs-approval | paused | queued`. Needs-a-look: `failed | exhausted | stalled | blocked`. Done today counts `status === "done"` on `last_progress_at` same local day. Columns (never “Ended” — projection has only `created_at`): Outcome, Loop, Inputs, Gens, Best, Started, Budget. Row grid `128 / 1.4fr / 1fr / 84 / 112 / 112 / 128 / 16`. Pending requests = warning `Pill` with count, not a fifth KPI. Story harness width `max-w-[1320px]` (wider than the run page). (`loop-runs-view.tsx:17-47`, `loop-runs-view.ts:7-110`, `loop-runs-table.tsx:13-57`, `loop-run-row.tsx:17-87`, `loop-runs.stories.tsx:21-22`)

- **Runs filters (S6):** Chip bar fields only: Origin (`catalog | session`), Session id (text), Outcome (full generated status vocab, **no counts**). Outcome is client-side partition; origin/session are server query. Empty: “No matching runs” / “No runs match the selected outcome filter.” (`loop-list-filters.ts:67-86`, `loop-runs-filters.tsx:40-91`)

- **Node inventory sibling (S5/S6 seam):** Same `/loop-runs` route, `?nodes={waiting|quarantined|attention|retrying}` + optional `nodes_loop` / `nodes_run`. Four state pills **without count badges** (route publishes `{items, next_cursor}` only). Columns: Node, Loop · run, Reason, Age (`{n}s|m|h|d` + “in this state”). Footer: “Showing N loaded · more available” / “Showing all N” — never a population total. View toggle cards/list. Empty titles: “Nothing is waiting/quarantined/needs attention/is retrying”. (`loop-node-inventory-view.tsx:76-83,115-118`, `loop-node-inventory.ts:16-28,172-216`, `loops-route-search.ts:18-26,95-110`)

- **Tasks list chrome (S1):** Body is `ListingPage` + grouped cards; **toolbar lives in the window topbar**, not the body: Search (`placeholder="Search tasks"`), **Filter** (status / priority / owner — owner options from the live list only), **Sorted by** `Most recent | Priority`. Default surface mode is `list`; URL `?mode=` accepts only `kanban | dashboard | inbox`. Stories host `OsWindowFrame` `title="Tasks"` at `max-w-[1080px]`. (`tasks-list-surface.tsx:52-149`, `tasks-list-toolbar.tsx:22-63`, `tasks-list-surface.stories.tsx:76-143`, `task-location-search.ts:13-79`)

- **Tasks list groups + row identity (S1):** Groups in order: Active (`in_progress`, accent ring), Blocked (danger), Needs attention (warning), Queued (`ready|pending|draft`, faint ring), Done (`completed` only), Failed (`failed|canceled`). Header count is loaded `n` or `n of {facet total}`. Row: `ListingRow` + `ListChecks` icon, **title**, `MonoId` (`task.identifier` or loop-cell short id or 7-char id), relative `last_activity_at`, meta · owner · `attempt N [of M]` (loop cells omit max) · `{n} subtasks` · `{n} deps` · `parent {shortId}` · failed error. Trailing pills only when signalled: priority (always **neutral**), Approval pending (accent), Blocked, Needs attention. Title row does **not** carry a status chip for ordinary queued/done work. (`task-grouping.ts:31-94`, `task-card.tsx:27-135`, `tasks-list-row.tsx:41-86`, `task-formatters.ts:189-193`)

- **Loop-owned task ids (S1 delete / parse target):** Canonical cell `^loop\..+\.g(\d+)\.node\.(.+)\.(\d+)$` → short `g{gen}.{nodeId}[{item}]` (item omitted when 0). Coordinator `^loop\..+\.coordinator$` → `coordinator`. Do not invent other nestings or a second id grammar. (`task-formatters.ts:68-111`)

- **Task status + filter vocab (S1):** Statuses and labels: Draft, Not started (`pending`), Blocked, Needs attention, Ready, In progress, Completed, Failed, Canceled. Tones: `in_progress`/`running` accent+pulse; `blocked`/`failed`/`canceled` danger; `needs_attention` warning; all else neutral. Filters: those nine statuses, priorities Urgent/High/Medium/Low, owners `{ref}` or `{ref} · {kind}`. Search matches title + identifier only. Kanban columns collapse terminals into **Done** and label `completed` as “Done”, `pending` as “Pending” — do not silently apply kanban labels on the list. (`tasks-list-filters.ts:25-46`, `task-formatters.ts:113-153`, `task-grouping.ts:14-28`, `task-kanban-card.tsx:24-34`)

- **Subtask disclosure (S1):** Collapsed by default. Summary always `{n} subtask(s)` then escalations first: needs attention · blocked · failed · running · done. Toggle warning-tinted when any escalation exists. Nested row: StatusDot + MonoId + status · attempt + relative time or failed error. A child whose parent is not on this page stays a root (pagination must not hide it). (`task-hierarchy.ts:12-90`, `task-subtask-list.tsx:73-114`)

- **Task empty / load-more (S1):** Empty unfiltered: title “No tasks yet”, description “Open a new task contract from the topbar to populate this list.” Filtered: “No tasks match the current filters” / “Clear filters to see other tasks in this workspace.” Error: “Unable to load tasks” + **Retry loading tasks**. Pagination: **Load more** / **Loading more**. `TasksEmptyState` (template cards) exists but is **not** mounted by `TasksListSurface` — do not draw template tiles on S1 unless the host list is switched to that component. (`tasks-list-surface.tsx:73-146`)

- **320px properties rail (not S1 body, but the shared detail seam):** Task detail rail documents itself as 320px, groups Approval / Current run / Last run / Properties / Execution / Activity, footer **Inspect** + `compozy task inspect {id}`. Explicitly omits lease, heartbeat, claim hash, seq. Approve/Reject here are task-approval, not loop-gate. (`task-properties-rail.tsx:55-60,85-297`)

- **Waits rail → inventory deep-link (S4→S6):** Rail rows **Needs an answer**, **Open waits**, **Needs attention**, **Quarantined** — nonzero values are links to `/loop-runs?nodes={state}&nodes_run={runId}`. Zero stays a faint numeral, not a hidden row. (`loop-run-waits-rail.tsx:17-88`)

- **Story / waiting / attention copy already shipped (S4):** Waiting titles by kind: timer / event / `approval_escalation` / request (ask vs review sentences). Attention flags: silence, dependency_quarantined, expired_wait — plus inventory-link “All flags in the inventory”. Story section **What happened**, gist `{n} events`, empty “Nothing yet”. (`loop-run-parked-panels.tsx:42-54,117-122,349-377`, `loop-run-story-timeline.tsx:226-240`)

## Relevant Sources

- `web/src/systems/loops/components/run-page/loop-run-page-body.tsx:48-57,255-372` — run layout, 1240/320 seam, section order, inspect sheet mount
- `web/src/systems/loops/lib/loop-run-page-view.ts:46,103-241` — now-card status gate, progress/usage/request projection
- `web/src/systems/loops/components/run-page/loop-run-now-card.tsx:48-292` — Happening now labels, dots, outbound links, node lines
- `web/src/systems/loops/components/run-page/loop-run-needs-you-card.tsx:40-228` — Needs you shell, gate verbs, quarantine rows
- `web/src/systems/loops/components/run-page/loop-run-inspect-sheet.tsx:125-213` — Inspect sheet chrome and tiles
- `web/src/systems/loops/components/run-page/loop-run-controls.tsx:21-84` — Pause / Resume / Cancel run
- `web/src/systems/loops/components/run-page/loop-run-overflow-menu.tsx:13-66` — View graph / View definition / Kill run…
- `web/src/systems/loops/components/run-page/loop-run-progress-panel.tsx:16-79` — Progress section + segment colours
- `web/src/systems/loops/components/run-page/loop-run-outcome-card.tsx:59-237` — Why it stopped / What went wrong
- `web/src/systems/loops/components/run-page/loop-run-about-rail.tsx:28-100` — About this run fields
- `web/src/systems/loops/components/run-page/loop-run-usage-rail.tsx:20-42` — Usage header
- `web/src/systems/loops/components/run-page/loop-run-waits-rail.tsx:17-88` — Waits & attention counts + inventory links
- `web/src/systems/loops/components/run-page/requests/loop-request-questionnaire.tsx:62-176` — one-question-at-a-time
- `web/src/systems/loops/components/run-page/requests/loop-request-card.tsx:145-237` — ask/review card + Submit answer
- `web/src/systems/loops/components/run-page/requests/loop-request-decision-bar.tsx:20-73` — approve/edit/reject/respond
- `web/src/systems/loops/components/run-page/loop-run-parked-panels.tsx:42-122,338-377` — Waiting / Needs attention
- `web/src/systems/loops/components/run-page/loop-run-story-timeline.tsx:226-270` — What happened
- `web/src/systems/loops/components/run-page/loop-strategy-progress.tsx:20-228` — fan-out strategy block
- `web/src/systems/loops/components/run-page/loop-node-row-actions.tsx:22-79` — promoted Resume / Requeue
- `web/src/systems/loops/components/stories/loop-run-page.stories.tsx:51-117` — topbar crumbs + actions composition
- `web/src/systems/loops/lib/loop-formatters.ts:12-84` — loop status labels/tones/pulse
- `web/src/systems/loops/lib/loop-node-lifecycle.ts:24-31,138-225` — lifecycle states + projection rules
- `web/src/systems/loops/lib/loop-node-now-view.ts:11-16,132-157` — which lifecycle lines enter Happening now
- `web/src/systems/loops/lib/loop-node-controls.ts:16-137,150-166` — offerable verbs + menu copy
- `web/src/systems/loops/lib/loop-node-verb-copy.ts:16-237` — confirm dialog copy
- `web/src/systems/loops/lib/loop-request-vocabulary.ts:17-135` — request kinds/states/decisions
- `web/src/systems/loops/lib/loop-run-progress.ts:7-24,153-200` — progress model
- `web/src/systems/loops/lib/loop-run-usage.ts:8-17,93-177` — usage rows + notes + approval facts
- `web/src/systems/loops/lib/loop-run-next-note.ts:14-46` — next-note sentence rules
- `web/src/systems/loops/lib/loop-palette.ts:23-233` — palette groups + retired `retry.max`
- `web/src/systems/loops/lib/loop-node-kind-icons.ts:29-90` — kind + class glyphs
- `web/src/systems/loops/lib/loop-story-icons.ts:32-67` — story timeline glyphs
- `web/src/systems/loops/components/detail/loop-body-dag.tsx:12-94` — read-only DAG
- `web/src/systems/loops/components/detail/loop-detail.tsx:102-237` — definition page chrome, DAG + recent runs
- `web/src/systems/loops/components/detail/loop-recent-runs.tsx:24-83` — per-loop roster
- `web/src/systems/loops/components/loop-page-lede.tsx:16-49` — definition lede
- `web/src/systems/loops/components/loop-section.tsx:14-52` — collapsible section chrome
- `web/src/systems/loops/components/loop-status-pill.tsx:11-23` — status chip
- `web/src/systems/loops/components/runs/loop-runs-view.tsx:17-47` — KPI + Active/Past
- `web/src/systems/loops/components/runs/loop-runs-table.tsx:13-57` — columns; no Ended
- `web/src/systems/loops/components/runs/loop-run-row.tsx:17-87` — row grid + pending-request pill
- `web/src/systems/loops/components/runs/loop-runs-kpis.tsx:18-70` — four KPI labels/tones
- `web/src/systems/loops/components/runs/loop-runs-filters.tsx:40-91` — origin / session / outcome chips
- `web/src/systems/loops/components/runs/loop-node-inventory-view.tsx:76-384` — inventory chrome, no totals
- `web/src/systems/loops/lib/loop-runs-view.ts:7-196` — KPI math, partition, inputs, budget bar
- `web/src/systems/loops/lib/loop-list-filters.ts:67-145` — runs filter fields
- `web/src/systems/loops/lib/loop-node-inventory.ts:16-216` — inventory labels/tones/empty copy
- `web/src/systems/loops/lib/loops-route-search.ts:18-26,95-110` — `/loop-runs` search keys
- `web/src/systems/loops/lib/loop-generation-presentation.ts:25-32` — Best column
- `web/src/systems/loops/lib/loop-graph.ts:225-230,302-326` — DAG class/kind summaries
- `web/src/systems/tasks/components/tasks-list-surface.tsx:32-150` — list body, empty, load more
- `web/src/systems/tasks/components/tasks-list-toolbar.tsx:22-63` — search / filter / sort
- `web/src/systems/tasks/components/tasks-list-filters.tsx:32-64` — Filter trigger
- `web/src/systems/tasks/components/tasks-list-sort.tsx:14-59` — Most recent / Priority
- `web/src/systems/tasks/components/task-card.tsx:27-135` — list identity + trailing pills
- `web/src/systems/tasks/components/tasks-list-row.tsx:41-86` — ListingRow anatomy
- `web/src/systems/tasks/components/task-group.tsx:23-68` — group header + count
- `web/src/systems/tasks/components/task-subtask-list.tsx:73-114` — collapsed subtasks
- `web/src/systems/tasks/components/task-kanban-card.tsx:24-152` — kanban identity (label deltas)
- `web/src/systems/tasks/components/tasks-kanban-board.tsx:16-28,82-95` — five columns
- `web/src/systems/tasks/components/task-properties-rail.tsx:55-297` — 320px rail contract
- `web/src/systems/tasks/components/task-page-head.tsx:26-336` — task head verbs (detail, not list)
- `web/src/systems/tasks/lib/tasks-list-filters.ts:25-114` — filter fields
- `web/src/systems/tasks/lib/task-grouping.ts:14-94` — list groups + kanban columns
- `web/src/systems/tasks/lib/task-hierarchy.ts:12-90` — tree + subtask summary
- `web/src/systems/tasks/lib/task-formatters.ts:44-66,68-111,113-193` — signals, loop-id parse, labels
- `web/src/systems/tasks/lib/task-location-search.ts:13-79` — list/kanban/dashboard/inbox modes
- `web/src/systems/tasks/lib/inbox-grouping.ts:9-29` — inbox lanes (adjacent to S1, not S1 body)
- `web/src/systems/tasks/components/stories/tasks-list-surface.stories.tsx:76-143` — Tasks window + toolbar slot

## Transferable Patterns

- **1240 / 320 / 1080 breakpoint → S4:** Draw the run artboard as main + 320px rail, collapsing to a single column below 1080px. Do not add a third column or inline the DAG/inspect tiles on the default view — Inspect is a sheet; DAG is on `/loops/$name` and `/loops/$name/editor`.

- **Incumbent section stack → S4:** Keep Needs you first when anything needs a person; keep Progress always; park Happening now under Progress except when status is `paused`. Do not invent a “timeline-first” or “DAG-first” default. Competing sections already named above are the ones that may be restyled or collapsed — not replaced with new named regions.

- **Closed chip + verb sets → S4/S6:** Reuse `LoopStatusPill` labels/tones and the four run verbs. Do not add Stop / Retry run / Skip / Approve-all. Gate buttons and request decisions are two different triads; do not draw one bar that mixes “Request changes” with “Edit and approve”.

- **Needs you anatomy → S4 needs-you artboard:** Neutral panelbox, warning glyph only, questionnaire then gate then quarantine. Reuse gist, fallback approval copy, and **Open entry**. Do not add a fourth gate verb or a count badge the payload does not publish.

- **Happening now exclusion → S4:** Restyle the live card, but omit waiting/quarantined lines from it. Those states already have Waiting / Needs you. Do not add a “lanes in flight” metric unless it is the confirm-strip clause (`loopRunStateStrip`) — and that strip omits zero counts.

- **KPI + column contract → S6:** Four KPIs, Active/Past, seven columns. Reuse “Best” as `Gen N · {score}` from `best_generation`/`best_score` (client never recomputes). Reuse budget mini-bar (`tokens_used / budget_tokens`, warn ≥90%, danger ≥100%, uncapped = count only). Do not add Ended, Success rate, or per-status KPI tiles.

- **Inventory as a mode, not a fifth KPI → S5/S6:** `?nodes=` is a sibling surface on `/loop-runs`. State pills have no badges. Foot is loaded-count only. Deep-links from the run waits rail already exist — reuse those query keys.

- **DAG as definition chrome → S5:** Horizontal spine, muted class glyphs, id + class + kind, read-only footer. Do not paint ToolID-specific marks or invent a live-state overlay on this view (live state belongs on the run page / inventory). Recent-runs roster stays on the same page under **Recent runs**, not inside the DAG.

- **Tasks list grammar → S1:** Topbar search + three filters + two sorts; body = six groups of `ListingRow`. Reuse `taskShortId` (including loop-cell parse). Trailing pills only for priority / approval / blocked / needs_attention. Do not colour priority. Do not add tag, date-range, or worktree filters to the list toolbar — those are not in `buildTaskFilterFields`.

- **Subtask collapse → S1:** Keep collapsed-by-default with escalation-first summary. Do not hide a child whose parent is off-page.

- **Shared rail footer → S4 detail / task detail:** Ghost **Inspect** + mono trail (`digest …` / `compozy task inspect {id}`). Reuse that foot; do not promote inspect tiles onto the default artboard.

- **LoopSection chrome → S4/S5:** Icon 14px + Eyebrow title + optional gist + optional right link + chevron. Restyle tokens, keep the anatomy so Needs you / Progress / What happened / Body · DAG read as the same family.

## Risks / Mismatches

- **Drawing a DAG on the run default (S4) would invent a surface.** Production run page never mounts `LoopBodyDag`. Graph access is overflow **View graph** → editor, and Inspect **View graph**. An S4 artboard that inlines a live DAG competes with Progress / Now / Story and is untruthful to the host.

- **Merging gate verbs with request verbs would invent a control.** Gate: Approve & resume / Request changes / Reject & halt. Request: Approve / Edit and approve / Reject / Respond / Submit answer. Same English word “Approve”, different payloads.

- **Kanban labels on the S1 list would lie.** List says `pending` → “Not started”, `completed` → “Completed”. Kanban says “Pending” / “Done”. S1 is the list.

- **`TasksEmptyState` template tiles on S1 would invent chrome.** List empty is a single `Empty`. Template cards are a different component, not wired into `TasksListSurface`.

- **Inventory count badges or “N waiting in workspace” would invent a total.** Route has no population field (SD-007). Same for KPI tiles computed beyond the loaded runs page — `buildRunKpis` only sees the current `runs` array.

- **An Ended / duration-to-end column on S6 would be untruthful.** Comment at `loop-runs-table.tsx:13-14`: projection exposes only `created_at`.

- **Colouring priority, node class, or ToolID would violate signal rules.** Priority tone is forced neutral. DAG glyphs are muted. Palette class is identity, not hue.

- **Authoring `retry.max` or `params.cwd` in any S5 node card is a delete target.** Runtime rejects both; palette seeds `retry.max_attempts` and environment.directory.

- **Promoting Kill beside Cancel, or Pause on `watching`/`needs-approval`, would be rejected by the daemon.** `loopRunVerbs` is the offerability contract.

- **Showing attempt ceilings on loop-cell task rows would invent a budget.** `TaskCard` passes `max_attempts: null` when `parseLoopTaskId` matches — loop cells retry through generations.

- **Task-rail Approve/Reject on a loop Needs-you artboard would mix products.** Task approval is a task record field; loop gate is `needs-approval` + `onDecision(gateId)`.

- **Story harness widths disagree (1080 Tasks / 1240 run / 1320 runs).** Artboards should pick the **production** run-page 1240 and the listing stories’ 1320/1080 as host hints, not invent a fourth max-width. Exact `PAGE_CONTENT_GUTTER` / `--width-detail-inspector-inline` tokens live in `@compozy/ui` (outside this slice).

- **Inbox / dashboard / kanban are not S1.** They exist (`TaskViewMode`) but S1 is the default list. Drawing inbox lanes (My work / Approvals / Failed runs / Blocked / Archived) on the tasks list invents a surface.

## Open Questions

- The `/loop-runs` **route** that swaps `LoopRunsView` vs `LoopNodeInventoryView` and hosts the listing toolbar is not in this slice (no page file under `systems/loops/components/runs/`). Confirm tab/title copy (“Runs” vs “Inventory”) from the route layer before locking S6 chrome.
- Whether S5’s “roster” is `LoopRecentRuns` on the definition page, the workspace inventory, or both — production has both; the artboard name is ambiguous.
- `PAGE_CONTENT_GUTTER` and `--width-detail-inspector-inline` numeric values are imported from `@compozy/ui`, outside the two system dirs — slice 03 should pin the token numbers.
- Host nav (app rail, window title “Loops” / “Tasks”) is composed in `web/` routes / OS chrome, not in these system dirs. Stories use `StoryTopbarHost title="Loops"` and `OsWindowFrame title="Tasks"`.
- `TasksEmptyState` vs list `Empty`: if product intent is to promote template tiles onto the list, that is a host change, not an artboard invention — needs an operator decision.
- Goal-turn disclosure and strategy-progress blocks compete for height on S4; no production rule says they fold by default (`LoopSection defaultOpen = true`).

## Evidence

- `web/src/systems/loops/components/run-page/loop-run-page-body.tsx`
- `web/src/systems/loops/lib/loop-run-page-view.ts`
- `web/src/systems/loops/components/run-page/loop-run-now-card.tsx`
- `web/src/systems/loops/components/run-page/loop-run-needs-you-card.tsx`
- `web/src/systems/loops/components/run-page/loop-run-inspect-sheet.tsx`
- `web/src/systems/loops/components/run-page/loop-run-controls.tsx`
- `web/src/systems/loops/components/run-page/loop-run-overflow-menu.tsx`
- `web/src/systems/loops/components/run-page/loop-run-progress-panel.tsx`
- `web/src/systems/loops/components/run-page/loop-run-outcome-card.tsx`
- `web/src/systems/loops/components/run-page/loop-run-about-rail.tsx`
- `web/src/systems/loops/components/run-page/loop-run-usage-rail.tsx`
- `web/src/systems/loops/components/run-page/loop-run-waits-rail.tsx`
- `web/src/systems/loops/components/run-page/loop-run-parked-panels.tsx`
- `web/src/systems/loops/components/run-page/loop-run-story-timeline.tsx`
- `web/src/systems/loops/components/run-page/loop-strategy-progress.tsx`
- `web/src/systems/loops/components/run-page/loop-node-row-actions.tsx`
- `web/src/systems/loops/components/run-page/loop-run-next-note.tsx`
- `web/src/systems/loops/components/run-page/requests/loop-request-questionnaire.tsx`
- `web/src/systems/loops/components/run-page/requests/loop-request-card.tsx`
- `web/src/systems/loops/components/run-page/requests/loop-request-decision-bar.tsx`
- `web/src/systems/loops/components/stories/loop-run-page.stories.tsx`
- `web/src/systems/loops/lib/loop-formatters.ts`
- `web/src/systems/loops/lib/loop-node-lifecycle.ts`
- `web/src/systems/loops/lib/loop-node-now-view.ts`
- `web/src/systems/loops/lib/loop-node-controls.ts`
- `web/src/systems/loops/lib/loop-node-verb-copy.ts`
- `web/src/systems/loops/lib/loop-request-vocabulary.ts`
- `web/src/systems/loops/lib/loop-run-progress.ts`
- `web/src/systems/loops/lib/loop-run-usage.ts`
- `web/src/systems/loops/lib/loop-run-next-note.ts`
- `web/src/systems/loops/lib/loop-palette.ts`
- `web/src/systems/loops/lib/loop-node-kind-icons.ts`
- `web/src/systems/loops/lib/loop-story-icons.ts`
- `web/src/systems/loops/lib/loop-graph.ts`
- `web/src/systems/loops/lib/loop-generation-presentation.ts`
- `web/src/systems/loops/lib/loop-runs-view.ts`
- `web/src/systems/loops/lib/loop-list-filters.ts`
- `web/src/systems/loops/lib/loop-node-inventory.ts`
- `web/src/systems/loops/lib/loop-node-inventory-state.ts`
- `web/src/systems/loops/lib/loops-route-search.ts`
- `web/src/systems/loops/lib/loop-run-about.ts`
- `web/src/systems/loops/components/detail/loop-body-dag.tsx`
- `web/src/systems/loops/components/detail/loop-detail.tsx`
- `web/src/systems/loops/components/detail/loop-recent-runs.tsx`
- `web/src/systems/loops/components/loop-page-lede.tsx`
- `web/src/systems/loops/components/loop-section.tsx`
- `web/src/systems/loops/components/loop-status-pill.tsx`
- `web/src/systems/loops/components/runs/loop-runs-view.tsx`
- `web/src/systems/loops/components/runs/loop-runs-table.tsx`
- `web/src/systems/loops/components/runs/loop-run-row.tsx`
- `web/src/systems/loops/components/runs/loop-runs-kpis.tsx`
- `web/src/systems/loops/components/runs/loop-runs-filters.tsx`
- `web/src/systems/loops/components/runs/loop-node-inventory-view.tsx`
- `web/src/systems/loops/components/stories/loop-runs.stories.tsx`
- `web/src/systems/tasks/components/tasks-list-surface.tsx`
- `web/src/systems/tasks/components/tasks-list-toolbar.tsx`
- `web/src/systems/tasks/components/tasks-list-filters.tsx`
- `web/src/systems/tasks/components/tasks-list-sort.tsx`
- `web/src/systems/tasks/components/task-card.tsx`
- `web/src/systems/tasks/components/tasks-list-row.tsx`
- `web/src/systems/tasks/components/task-group.tsx`
- `web/src/systems/tasks/components/task-subtask-list.tsx`
- `web/src/systems/tasks/components/task-kanban-card.tsx`
- `web/src/systems/tasks/components/tasks-kanban-board.tsx`
- `web/src/systems/tasks/components/task-properties-rail.tsx`
- `web/src/systems/tasks/components/task-page-head.tsx`
- `web/src/systems/tasks/components/tasks-empty-state.tsx`
- `web/src/systems/tasks/components/stories/tasks-list-surface.stories.tsx`
- `web/src/systems/tasks/lib/tasks-list-filters.ts`
- `web/src/systems/tasks/lib/task-grouping.ts`
- `web/src/systems/tasks/lib/task-hierarchy.ts`
- `web/src/systems/tasks/lib/task-formatters.ts`
- `web/src/systems/tasks/lib/task-location-search.ts`
- `web/src/systems/tasks/lib/inbox-grouping.ts`
- `web/src/systems/tasks/lib/task-time-formatters.ts`
- `web/src/systems/tasks/hooks/use-tasks-page.ts`
