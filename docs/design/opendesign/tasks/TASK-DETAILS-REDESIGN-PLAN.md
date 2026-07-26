# Task Details Redesign Plan

> Status: **approved** · Created 2026-07-16 · Approved 2026-07-17 · Owner: Pedro
> Sources: 7 production screenshots (CleanShot 23.38.*), full web implementation audit, @agh/ui primitive audit, runtime-truth inventory (DTOs, state machines, HTTP/CLI verbs), AGH pattern bar (DESIGN.md, tokens.css, design-system.html, LOOPS-DESIGN-SPEC, agent-detail/run-detail prototypes).
> This document is editable. TODO markers and open questions are inline. Approve or edit it, then hand it back for prototype generation.

## 0. Intent

Rebuild the task-details surface (detail page, all its internal views, and the run-detail page) from an operator console into a calm, Linear-grade product page that a non-expert can read at a glance, while keeping every operator capability reachable through progressive disclosure. Strictly reuse `@agh/ui` primitives and the AGH pattern bar; render only controls the daemon actually supports.

**Done means:** a reviewer can answer "what is this task, what is happening now, what do I do next" in under 5 seconds on every state; heuristic score moves from the current ~16/40 baseline to 30+; zero raw enums/ids as primary text; one accent target per viewport; every control maps to a real HTTP/CLI verb.

---

## 1. Why: evidence of the problem

### 1.1 What the screenshots show (current production UI)

| # | Finding | Evidence |
|---|---|---|
| E1 | Status rendered 5+ ways at once: title dot, "In Progress" pill, "Running" pill, LIVE dots on 2 tabs, run pill, diagnostics "Current run: Running" | S1 header + tabs |
| E2 | Header wall: mono id + up to 8 pills + 4-line meta before any content, repeated on all 7 tabs | S1-S7 |
| E3 | 7 tabs + sub-tab systems (Events has 3 grouping modes for 4 events) | S1, S3 |
| E4 | 4-7 permanent action buttons; Delete always one click away; enablement rules invisible | S1 |
| E5 | Stat cards duplicating tab counts (Children 3 / Dependencies 1 / Runs 2), "Owner" oddly inside the Children card | S1 Overview |
| E6 | Raw system identifiers as primary text: `run_001`, `sess_product_brief`, `task.run_failed`, `seq 103`, `task.inspect.task_run_stuck.run_001`, `coord-launch-001` | S1, S3 |
| E7 | Operator diagnostics embedded in default Overview (INSPECT DIAGNOSTICS grid + warn card with rule code) | S1 |
| E8 | Unhumanized values: `ELAPSED 2176h 38m 26s` | S1 |
| E9 | Agents tab: giant cards where "LIVE: Yes" is a stat cell duplicating the LIVE pill beside the name | S4 |
| E10 | Near-empty full-width tables (Runs: 2 rows, Dependencies: 1 row) with misaligned wrapping | S2, S6 |
| E11 | Orchestration tab is a config dump: worker/coordinator/mode chips, capability tokens, warning banner in system prose | S7 |
| E12 | Accent orange exploded: "Open" buttons on every row, LIVE dots, event icons, title dot; accent stopped being a signal | all |

### 1.2 What the code audit confirms (file:line)

- **8-pill header + 7-button action bar** with 8 interlocking enablement booleans: `web/src/systems/tasks/components/tasks-detail-header-sections.tsx:51-154`, `:204-400` (`:217-239` for the boolean ladder).
- **Orchestration tab stacks 5 expert cards** (`tasks-detail-orchestration-panel.tsx:40-44`): raw-JSON execution profile editor (`tasks-execution-profile-card.tsx:351-360`, 472 ln), bridge-notifications table with 6 columns and an 8-field create dialog (`tasks-bridge-notifications-card.tsx:380-504`, 606 ln), reviews table with 7 columns exposing `submit_run_review` (`tasks-reviews-card.tsx:132`), SSE meters labeled "SSE resume seed / awaiting first frame" (`tasks-stream-resume-card.tsx:15-84`).
- **10 competing status tone maps** feed pills on the same screen (`task-formatters.ts:630`, `task-inspect-diagnostics-card.tsx:19-34`, `tasks-reviews-card.tsx:36-52`, `tasks-stream-resume-card.tsx:15-20`, `lib/status-tone`, others).
- **Scheduler vocabulary as user copy**, concentrated in `task-formatters.ts` (771 ln): "Coordinator handoff" (`:595`), "channel chatter never owns status" (`:621`), "Saved intent only -- no runs yet" (`tasks-detail-runs-panel.tsx:55`), raw enums `stranded` / `waiting_for_session` / `recovery_required` shown underscore-stripped (`task-inspect-diagnostics-card.tsx:19-25,50-55`).
- **Bespoke re-implementations of kit primitives**: `SummaryItem` and `DiagnosticRow` hand-rolled cards (`task-inspect-diagnostics-card.tsx:77-207`), `agent-card.tsx:44-165` (article + dl + ul instead of Card/MetricGrid/Timeline), `ProfileLine` and `BridgeTargetRow` byte-identical mono label/value helpers (`tasks-execution-profile-card.tsx:463`, `tasks-bridge-notifications-card.tsx:597`), hand-rolled shimmer progress (`tasks-detail-children-panel.tsx:118-132`), "Live" indicator re-implemented 3 times.
- **Panel padding drift**: overview `px-9 py-7` vs other tabs `px-6 py-5`; LaneTabs wrapped in a div that fights its own border (`tasks-detail-tabs.tsx:27-37`).
- **Tabs are React state, not URLs** (`tasks.$id.tsx:116-207`): no deep links into Runs/Activity.
- Scale: ~41 components / 7,268 ln + 17 hooks + 13 lib modules; jargon concentrated in `task-formatters.ts`.

### 1.3 Baseline heuristic score (current UI)

| Heuristic | Score /4 | Key issue |
|---|---|---|
| Visibility of system status | 2 | Status everywhere, hierarchy nowhere; stale `2176h` elapsed |
| Match system / real world | 1 | Coordinator handoff, seq, claim, raw enums |
| User control and freedom | 2 | Actions exist; enablement opaque; no undo affordances |
| Consistency and standards | 2 | 10 tone maps, padding drift, 3 live-indicator variants |
| Error prevention | 2 | Delete permanently exposed; force-fail has confirm |
| Recognition over recall | 2 | Ids must be memorized to correlate run/session/channel |
| Flexibility and efficiency | 1 | No keyboard shortcuts, no bulk, no command affordance |
| Aesthetic and minimalist | 1 | The core complaint: everything at max volume |
| Error recovery | 2 | Recovery paths exist but described in scheduler jargon |
| Help and documentation | 1 | Tooltips restate jargon |
| **Total** | **16/40** | **Poor: major UX overhaul required** |

Cognitive-load checklist: fails 6 of 8 (single focus, chunking, visual hierarchy, minimal choices, working memory, progressive disclosure). Target after redesign: fail 0-1.

---

## 2. Scope

**In scope**
- `/tasks/$id` detail page: header, all 7 current tabs, their replacement IA.
- `/tasks/$id/runs/$runId` run-detail page.
- The operator disclosure layer (diagnostics, bridges, stream, raw data).
- Task-scoped dialogs/sheets opened from these pages (execution profile editor, confirmations).
- New/changed `@agh/ui` primitives required by the above.

**Out of scope (separate initiatives)**
- `/tasks` list surfaces (List / Kanban / Dashboard / Inbox) beyond inheriting the same status vocabulary and topbar chrome.
- Task create/edit full-page forms (modal-redesign track owns dialog anatomy).
- Backend/API changes. The redesign works against today's contracts; implementation notes flag dead SSE listeners to clean up.

---

## 3. Design principles (Linear-grade, AGH-constrained)

1. **One question per zone.** Head answers "what is this and where does it stand". Content column answers "what happened and what's next". Rail answers "how is it configured". Operator drawer answers "why exactly, in machine terms".
2. **Single source of status.** Exactly one task-status pill, in `.head__pills`. Run rows carry their own run status. One live indicator total (pulse on the status pill while a run is active). Everything else derives, never repeats.
3. **Progressive disclosure with a firm floor.** Default view is readable by a non-expert. Every operator datum stays reachable in ≤2 clicks (Inspect drawer), none of it deleted. Heavy users lose zero capability.
4. **Exception-based pills.** Pills appear only when they carry exceptional information: priority ≠ medium, approval pending, paused, blocked, needs attention. Normal state = quiet page.
5. **Verbs, not toggles.** Task status is derived by the daemon (there is no set-status verb). The UI offers state transitions as actions (Start, Pause, Approve, Retry, Recover), never an editable status dropdown. Truthful UI over familiar UI.
6. **Human language first, machine truth one hover away.** Every event, state, and error renders in plain product language; the raw type/id stays as mono secondary text or tooltip for operators.
7. **Primitives only.** Every element maps to an `@agh/ui` export or a named domain composite built on them. No bespoke cards, dls, or live dots. Kit gaps get fixed in `packages/ui` (story + test), not worked around.
8. **Accent budget: one target per viewport.** The single contextual primary action owns the accent. Row links, tabs, dots, icons are neutral.
9. **Density where data earns it.** Tables and activity feeds can be dense (product register permission); chrome, meta, and configuration cannot.

---

## 4. Target information architecture

### 4.1 Page anatomy (`/tasks/$id`)

```
Topbar (48px, 3-zone: breadcrumb | tasks route-nav | actions)   <- design-system.html contract
└─ Page
   ├─ .head--detail: 24px accent icon well · H1 title · pills (exception-based) · meta line
   ├─ Tabs (3): Overview · Runs (n) · Activity
   ├─ Content column (minmax(0,1fr), sections max-width per pattern bar)
   └─ Properties rail (320px, DetailInspector: inline >=1440px, sheet below)
Operator layer: "Inspect" ghost icon-button (rail footer + overflow menu) -> Sheet drawer
Config layer: "Edit setup" -> execution-profile Sheet (form, not JSON)
```

Tab count drops 7 → 3. Former tabs are re-homed:

| Current tab | Destination |
|---|---|
| Overview | Overview (rebuilt) |
| Runs | Runs tab (kept, simplified) + run-detail page |
| Events | Activity tab (humanized feed; full log with filters) |
| Agents | Dissolved: owner + per-run claimant in Runs/rail; subtask owners on subtask rows |
| Children | Overview section "Subtasks" (inline, with progress) |
| Dependencies | Overview section "Blocked by / Blocks" (inline) |
| Orchestration | Split: execution profile → "Setup" sheet + rail summary; reviews → Runs/run-detail; bridges + stream + diagnostics → Inspect drawer |

TODO(pedro): confirm dissolving the Agents tab. Alternative: keep a 4th "Agents" tab only when a task has fan-out designations.

### 4.2 Head (uses `DetailHeader`)

- Crumbs: `Tasks / task-1` (breadcrumb belongs to the topbar contract; head carries no duplicate back button).
- Pre-title: none (the eyebrow "TASK" dies; the route already says it).
- H1: title, 1.4rem/600, tracking -.028em, single H1, `tabindex="-1"`.
- Pills row (`.head__pills`, max 3 visible + overflow):
  1. **Status pill** (single source of truth): tone-mapped task status, `pulse` only while a run is actively `starting|running`. Label vocabulary in §7.1.
  2. **Priority pill** only when ≠ medium (`urgent` danger, `high` warning, `low` neutral).
  3. **Exception pill** when applicable: `Approval pending` (info) / `Paused` (warning) / `Needs attention` (danger) / `Blocked` (warning). If 2+ exceptions, show the most severe + `+n` overflow popover.
- Meta line (12px subtle, dotsep): `Owner {owner} · Created by {created_by} · Updated {Time relative}`. Mono task id moves to the rail (with copy). `Origin WEB` leaves the header (tier c).
- Actions (right, per topbar contract: low→high emphasis, ONE accent target, then overflow):
  - Contextual primary (accent, state-driven, §6).
  - `Edit` (ghost).
  - Overflow menu `⋯`: Pause/Resume, Cancel, Publish, Delete (destructive, confirm via `ConfirmDialog`), Copy task id, Open in CLI hint.

### 4.3 Overview tab (content column)

Order, each as a `Section` (label 10.5px uppercase; body in `panelbox` only when framed content needs it):

1. **Now strip** (only when something is happening):
   - Active run → `RunCard`: "Attempt 2 of 3 · started 5m ago" + claimant (`OwnerAvatar` + name) + Open run link. Live dot inside the card only (header pill already pulses; no second LIVE label).
   - Terminal outcome (last run failed/completed within recency or task terminal) → flat tint band (`.oband` pattern / `StatusCard`): "Failed after 3 attempts: {error, humanized}" + `Retry` action; or "Completed · 2h ago" + View result.
   - Blocked → one `StatusCard` per `blocked_reasons` entry in plain language (§7.3) with its resolving action (Clear block, Approve, Resume, Open blocking task).
2. **Description** — `DescriptionCard` (markdown). Empty state: quiet inline "No description" + Add description (ghost).
3. **Subtasks** (when `child_count > 0` or on add) — `Section` label + count + `StackedProgress` (n of m completed) + `LinkedRecordTable`-style rows: status dot, title, owner avatar, updated `Time`. Row itself is the link (no accent "Open" button). `+ Add subtask` ghost row.
4. **Dependencies** (when any) — two quiet groups: "Blocked by" and "Blocks". Same row anatomy. Resolved dependencies collapse under "n resolved".
5. **Recent activity** — last 5 humanized events (`Timeline`/`TimelineEvent` compact), "View all" → Activity tab.

Deleted from Overview: metric cards (E5), diagnostics grid (E7 → Inspect drawer), channel pills (→ rail), description box frame when empty.

### 4.4 Properties rail (320px, `DetailInspector`)

`PropertyRow` list (label subtle 11px / value 13px, mono only for ids), grouped:

- **Properties**: Priority (inline `CommandSelect` editor: PATCH-backed), Owner (inline select: PATCH-backed), Workspace, Parent task (link, when subtask).
- **Execution**: Agent/worker (from execution profile: agent name or allowed set summary), Model (provider · model, mono), Sandbox (name), Attempts (used of max, e.g. "2 of 3"), Auto-enqueue (on/off, PATCH-backed switch), Channel (network channel, mono, when set). `Edit setup` ghost link → Setup sheet (§4.7).
- **Approval** (when policy = manual): state + Approve/Reject inline when pending.
- **Activity meta**: Created (`Time` relative + absolute tooltip), Updated, Closed (when set), Created by, Task id (`MonoId` copy), Current run id (`MonoId`, when active).
- Rail footer: `Inspect` ghost icon-button (opens operator drawer) — the only entry point to tier-c data besides the overflow menu.

Rule: rail shows tier (a) and (b) fields only (§11). No lease, heartbeat, claim hash, seq.

### 4.5 Runs tab

- `LinkedRecordTable`: columns **Attempt** ("Attempt 2" + `previous run` lineage caret when retried), **Status** (pill), **Claimed by** (`OwnerAvatar` + name, when claimed), **Started** (`Time`), **Duration** (humanized, live-ticking while running), **Result** (one-line: error snippet or "Result ready"). Row = link to run detail. No per-row accent button.
- Reviews, when `review` recorded for a run, render as a secondary line under the row: "Review: approved · reason" (tone by outcome), not a separate 7-column table.
- Empty state (`Empty`): title "Not started yet", body "Start a run to have {worker} work on this task.", action `Start run` (only when runtime allows).

### 4.6 Activity tab

- Humanized, newest-first feed on `Timeline`/`TimelineEvent`: icon by category, plain-language title (§7.2), actor + `Time`, optional detail line. Raw `event_type` + seq render as mono microtext on the right (operator scent, quiet).
- One `LiveBadge` in the section header (the only LIVE text on the page) while the SSE stream is attached.
- Filters: single `PillGroup` — All · Runs · Reviews · Changes (grouping modes "By agent / By event type" die; filtering replaces grouping).
- Load-more pagination (existing timeline query). Reduced motion: no pulse.

### 4.7 Setup sheet (execution profile)

Replaces the Orchestration tab's profile card + JSON editor:

- `Sheet` (right, per MODAL-STANDARD anatomy: icon well + eyebrow + title, body scroll, single primary footer action).
- Form on `Field*`/`FieldRow`: Worker mode (pillgroup), Worker agent(s) (CommandSelect multi), Provider/Model via the **canonical `RuntimeSelector`** (reuse verbatim, 7-bar reasoning meter, no rebuilt selector), Coordinator (mode + agent + guidance textarea), Review policy, Sandbox (mode + ref), Participants.
- Guardrail: when a run is active, sheet opens read-only with a quiet inline note "Editing is locked while a run is active." (replaces the warning banner prose). No catalog-status banners.
- `Fan out` action (when designations configured) moves next to `Start run` in the primary-action logic, opening a small dialog: channel + assignments builder (structured rows, not a mono textarea). TODO(pedro): confirm fan-out stays user-facing vs operator drawer.
- Raw JSON view stays available inside the sheet behind a "View JSON" toggle (`CodeBlock`, read-only + copy) for operators; editing is form-first.

### 4.8 Inspect drawer (operator layer)

`Sheet` (wide) with `LaneTabs`: **Diagnostics** · **Stream** · **Bridges** · **Raw**.

- Diagnostics: current `TaskInspect` content rebuilt on `MetadataTile` (bordered variant) + `StatusCard` per finding; next-action enums humanized (§7.3) with the suggested CLI command in `CodeBlock`.
- Stream: connection state, latest seq, resume seed (`Metric`/`MetadataTile`), reconnect action.
- Bridges: existing subscriptions table + create dialog rebuilt on `FieldRow` (kept 8 fields; this is operator turf).
- Raw: task + active run DTOs in `JsonViewer`.

Everything currently rendered is preserved here; nothing is lost, it just stops being the default.

### 4.9 Run-detail page (`/tasks/$id/runs/$runId`)

- Head (compact variant): crumbs `Tasks / task-1 / Attempt 2`; H1 "Attempt 2 of 3"; pills: run status (+ pulse when running), `operator forced` exception pill when `failure_kind` set. Meta: claimed by · started `Time` · duration.
- Actions: primary `Open session` (when `session_id`); `Retry` primary when failed and attempts remain; overflow: Release, Force fail (ConfirmDialog with reason field), Recover, Cancel.
- Content column:
  1. **Outcome band** (terminal runs): flat tint, humanized error or completion + `created_task_ids` links.
  2. **Result** — `result` JSON via `JsonViewer` (or `Markdown` when result is prose-shaped). Empty: "No result recorded."
  3. **Review** (when any): `StatusCard` per review round: outcome tone, reason, next-round guidance. Vocabulary §7.4.
  4. **Timeline** — run-scoped humanized events (same anatomy as Activity).
- Rail: Session (link + `MonoId`), Attempt lineage (previous run link), Queued/Claimed/Started/Ended timestamps, operational summary when non-null (Tool calls, Turns, Tokens, Cost) rendering "—" when null, never invented. Inspect drawer entry for lease/heartbeat/claim-hash/idempotency.
- Live note: today this page polls. Prototype may show live states, but implementation must attach the task stream (or session stream) before advertising "live" here (§11.4).

### 4.10 URL addressability

Tabs become URL state: `/tasks/$id?tab=runs` (or child routes). Deep links to a run, to Activity, and to the Inspect drawer (`?inspect=stream`) must survive refresh. TODO(eng): pick search-param vs nested-route during implementation.

---

## 5. Layout and visual system (binds the pattern bar)

- **Chrome**: 48px 3-zone topbar; breadcrumb left; center route-nav only on the `/tasks` index (detail pages keep center empty); right actions 22px, one accent, overflow last. Never: route icon, H1, status, counts in the topbar.
- **Grid**: `.page` max-width 1240px, padding 24px 36px 80px; content + 320px rail, gap 32px; collapse to 1 column ≤1080px (rail becomes sheet).
- **Tabs**: 40px height, 13px/500, count as mono chip; **foreground underline** (1.5px), never accent (per DESIGN.md Tabs contract; note: agent-detail.html's accent underline is the older idiom, do not copy it).
- **Sections**: label 10.5px/600 uppercase tracking .04em subtle; 24-28px section gap; one padding rhythm for all tab panels (kill the px-9/px-6 drift): panel padding 24px, rows 12-14px vertical.
- **Type**: Inter; H1 1.4rem/600 -.028em; body 13.5px; meta 12px; micro labels 10.5-11px uppercase; JetBrains Mono strictly for ids, event types, model names, timestamps in feeds. Never mono for prose or labels.
- **Color**: tokens.css only. Signal palette carries state (success/warning/danger/info + neutral); accent = primary action + live pulse only. Status dots 6px inside pills; hollow ring = pending; pulse = live, wrapped in `prefers-reduced-motion`.
- **Depth**: flat; `--line`/`--line-soft` hairlines; `panelbox` canvas-soft radius-lg; shadows only on overlays (`--shadow-overlay`).
- **Motion**: 100/140/200ms, `--ease-out`; live-tick duration counter updates without layout shift (tabular numerals); skeleton shimmer 1.4s; all guarded by reduced motion.
- **Bans in force**: no side-stripe accent rails, no gradients, no hero-metric cards, no accent tabs/hover/rails, no eyebrow-per-section scaffolding, no em dashes in UI copy, no demo/designer controls in product files.

---

## 6. Primary-action state machine (truthful controls)

Exactly one accent action, derived from state; everything else ghost/overflow. All verbs are real HTTP/CLI surfaces (`internal/api/httpapi/routes.go:156-231`, CLI `agh task ...`).

| Task state | Primary (accent) | Secondary visible | Overflow |
|---|---|---|---|
| draft | Publish | Edit | Delete |
| pending / ready (no active run) | Start run | Edit | Pause, Cancel, Delete, Fan out* |
| approval pending | Approve | Reject | Edit, Cancel, Delete |
| blocked | none (block cards carry their own resolving actions) | Edit | Pause, Cancel, Delete |
| in_progress (run active) | Open run | Pause | Cancel, Delete |
| paused | Resume | Edit | Cancel, Delete |
| needs_attention | Recover | Open run | Cancel, Delete |
| failed (attempts remain) | Retry | Edit | Start new run, Delete |
| failed (exhausted) / completed / canceled | none | Edit (completed: View result scrolls to outcome) | Delete, Re-open via new run* |

\* Fan out only when profile designations exist. "Re-open" = enqueue new run where the runtime allows it. TODO(pedro): validate the failed-exhausted row; runtime allows manual enqueue — decide whether to expose it as "Run again".

Run-detail: queued/claimed → overflow only (Release); running → Open session primary; failed → Retry primary; completed → Open session ghost. Force fail always behind ConfirmDialog with required reason ("Recorded in the audit log" as helper text, plain language).

---

## 7. Copy and humanization spec

All copy changes concentrate in `task-formatters.ts` (single file, 771 ln today). Rules: sentence case, no em dashes, no exclamation, no scheduler nouns (claim, handoff, coordinator, seq, lease, heartbeat, starvation) outside the Inspect drawer, glossary-compliant (capability, never workflow/recipe).

### 7.1 Task status vocabulary

| Enum | Pill label | Tone |
|---|---|---|
| draft | Draft | neutral |
| pending | Not started | neutral |
| ready | Ready | info |
| in_progress | In progress | accent-tint (pulse while run active) |
| blocked | Blocked | warning |
| needs_attention | Needs attention | danger |
| completed | Completed | success |
| failed | Failed | danger |
| canceled | Canceled | neutral |

Run status: Queued / Claimed → "Assigned" / Starting → "Starting" / Running / Completed / Failed / Canceled / Needs attention.

### 7.2 Event humanization (Activity feed titles)

| event_type | Title (plain) |
|---|---|
| task.created | Task created |
| task.updated | Details updated |
| task.published | Published |
| task.run_enqueued | Run queued |
| task.run_claimed | {agent} picked this up |
| task.run_started | Attempt {n} started |
| task.run_completed | Attempt {n} completed |
| task.run_failed | Attempt {n} failed: {short error} |
| task.run_canceled | Attempt {n} canceled |
| task.run_needs_attention | Attempt {n} got stuck |
| task.run_recovered | Recovered and requeued |
| task.dependency_added | Now waits on "{title}" |
| task.paused / resumed | Paused by {actor} / Resumed |
| task.approved / rejected | Approved by {actor} / Rejected |
| task.block.created / cleared | Blocked: {reason} / Block cleared |
| task.run_review_recorded | Review: {outcome} |
| task.auto_enqueue.triggered | Started automatically (dependencies cleared) |

Full 63-type map to be completed during implementation; unmapped types fall back to a generic "System event" with the raw type as secondary mono text. Raw type + seq always available as microtext/tooltip.

### 7.3 Diagnostics next-action enums (Inspect drawer + Now strip)

| Enum | Plain language |
|---|---|
| claim_available | A worker can pick this up now |
| waiting_for_session | Waiting for the agent session to attach |
| stranded | Run lost its worker. Retry or release it |
| recovery_required | Stuck run needs recovery. Use Recover to requeue |

Example rewrite (S1 warn card): "Run heartbeat is stale. The claimed run has not reported a heartbeat inside the expected window." → **"This attempt looks stuck. The agent stopped responding 4 minutes ago."** + action `Recover` + mono microtext `task_run_stuck · run_001`.

### 7.4 Time and numbers

- `Time` relative by default ("5m ago"), absolute on hover/tooltip; absolute in tables when scanning matters (Started column keeps relative + tooltip).
- Durations humanized to 2 units max ("3d 2h", "5m 12s"); never "2176h 38m 26s". Live durations tick with tabular numerals.
- Costs/tokens: render "—" when null (best-effort fields), never a fake 0.

---

## 8. Component mapping

### 8.1 Reuse as-is (`@agh/ui`)

| Surface element | Primitive |
|---|---|
| Page scaffold / topbar slots | `PageShell` (density route) + `Topbar` slots |
| Head | `DetailHeader` (crumbs, title, pills, meta, actions) |
| Tabs | `LaneTabs` (URL-bound), counts via `count` prop |
| Rail | `DetailInspector` (320px inline ≥1440px, sheet below) |
| Sections | `Section` (label/count/right) |
| Status chips | `Pill` + `Pill.Dot` (tone map §7.1, `pulse` for live) |
| Now strip / outcome | `RunCard`, `StatusCard`, `Alert` |
| Subtasks/dependencies/runs rows | `LinkedRecordTable` (+ `StackedProgress` for subtask progress) |
| Activity | `Timeline` + `TimelineEvent`; `PillGroup` filter |
| Key/values | `MetadataList`, `MetadataTile`, `ContextBox` |
| Ids / time | `MonoId` (copy), `Time` |
| Result / raw | `JsonViewer`, `CodeBlock`, `Markdown`, `DescriptionCard` |
| States | `RouteState` (route-level), `DataSurface`, `Empty`, `Skeleton`/`BlockLoading` |
| Overlays | `Sheet` (setup, inspect), `ConfirmDialog`, `DropdownMenu` (overflow), `CommandSelect` (priority/owner editors) |
| People | `OwnerAvatar` |
| Setup selector | canonical `RuntimeSelector` (verbatim; 7-bar reasoning meter) |

### 8.2 Add to `packages/ui` (each: story + test)

| New primitive | Why |
|---|---|
| `PropertyRow` (label/value row, optional inline editor slot, mono variant) | Rail anatomy; kills `ProfileLine`/`BridgeTargetRow` duplicates |
| `MetadataTile` `variant="bordered"` | Diagnostics `SummaryItem` replacement |
| `LiveBadge` (pulse dot + label, reduced-motion-safe) | 3 hand-rolled implementations today; single live vocabulary |
| `PillField` (eyebrow label + wrapped pill cluster) | Profile `ListSlot` replacement (allowed agents, capabilities) |

Rejected: generic `StatusSelect` for task status — status is derived by the daemon; an editable status control would be untruthful. Priority/Owner use `PropertyRow` + `CommandSelect` instead.

### 8.3 Domain composites (`web/src/systems/tasks/components/`)

`TaskStatusPill` (single tone map, replaces the 10), `TaskPrimaryAction` (state machine §6), `TaskNowStrip`, `TaskBlockCard`, `TaskSubtasksSection`, `TaskDependenciesSection`, `TaskActivityItem` (event → humanized props), `TaskPropertiesRail`, `TaskSetupSheet`, `TaskInspectDrawer`, `TaskRunOutcomeBand`, `TaskReviewCard`. All are compositions of §8.1/8.2 primitives; zero bespoke card/dl/dot markup.

---

## 9. States matrix (every view ships all of these)

| State | Treatment |
|---|---|
| Loading | `Skeleton` rows matching final layout (no spinners mid-content); route-level `RouteState mode=loading` |
| Empty | `Empty` with next useful action (§4.5 example); rail hides empty groups |
| Error | `RouteState mode=error` with retry; inline `Alert` for section-level fetch errors |
| Live | Status pill pulse + LiveBadge on Activity; SSE invalidation as today; reduced-motion kills pulse |
| Blocked/paused/needs-attention | Now-strip cards with resolving actions (never banner prose) |
| Disconnected | Quiet `ConnectionIndicator` in Activity header, "Reconnect" action in Inspect > Stream |

---

## 10. Accessibility floor

- Contrast: body text ≥4.5:1 on canvas ramp (verify muted-on-canvas-soft combos), pills ≥3:1.
- Full keyboard path: tabs, rows (row = link), overflow menu, sheets, drawer; visible focus ring (`--shadow-focus-ring` only).
- Status never color-alone: pill text + dot shape (ring vs solid) carry state.
- Live regions: status pill and Now strip announce transitions (`aria-live="polite"`).
- Reduced motion: no pulse, no shimmer, instant transitions.

---

## 11. Truthful-UI constraints (runtime facts the design must respect)

1. **No set-status.** Task status is reconciled from runs + gates (`manager_reconcile_status.go:199-241`). UI transitions only via: publish, start, cancel, pause, resume, approve, reject, recover, block clear, run retry/release/force-fail.
2. **No delete-run verb.** Runs terminalize; only tasks delete. Run overflow never shows Delete.
3. **Nullable operational metrics.** Tool calls/turns/tokens/cost come best-effort from the bound session; render "—" when null (`contract/tasks.go:313-324`).
4. **Run page is not live today.** Polling only (`use-task-run-page.ts:48`); either attach the task stream during implementation or do not render live affordances there.
5. **Tier-c fields stay in Inspect**: claim_token_hash, lease_until, heartbeat_at, idempotency_key, designation_group_id, coordination_channel_id, event seq, previous_run_id, spawn failures, review circuit fields, raw payloads. Raw claim tokens do not exist in any DTO (never render).
6. **Reviews come from their own endpoints** (`/:id/reviews`), not the task record.
7. **CLI surface is `agh task` (singular)** for any command hints.
8. **Dead SSE listeners** in `use-task-stream.ts:33-96` (`task.run_review_circuit_opened`, `task.notification_delivered`, hook-bus names) to be pruned during implementation.
9. Every rendered control maps to §6 verbs; anything else is forbidden (fake affordances ban).

---

## 12. Prototype deliverables (next generation step, in this workspace)

| File | Content |
|---|---|
| `tasks/task-detail.html` | Full detail page, Overview default; working tabs (Overview/Runs/Activity), rail, overflow menu, Inspect drawer, Setup sheet (static states OK); primary states demonstrated via realistic data: in_progress with live run |
| `tasks/task-detail-states.html` | Same layout in 4 key states: blocked+approval, failed (retry), needs_attention, completed (compact vertical gallery of real pages, one per anchor; no designer chrome) |
| `tasks/task-run-detail.html` | Run detail: running state + failed state anchors, result viewer, review card, inspect entries |
| `tasks/critique.json` | 5-axis critique record per house convention |

Rules for the prototypes: tokens copied verbatim from the canonical token block (agent-detail.html lineage), self-contained files, `data-od-id` on sections, no viewport/theme toggles, screen-file-first, real specific copy (the launch-cutover scenario from the screenshots is a good fixture), no invented metrics.

TODO(pedro): confirm whether `tasks/task-detail-states.html` should be one gallery file or separate files per state.

---

## 13. Implementation notes (for the eventual web/ execution)

- **Consolidation map**: rebuild `agent-card.tsx`, `task-inspect-diagnostics-card.tsx`, `tasks-execution-profile-card.tsx` (form + RuntimeSelector), `tasks-bridge-notifications-card.tsx` (FieldRow), `tasks-multi-agent-panel.tsx` on primitives; delete `SummaryItem`, `DiagnosticRow`, `AgentMetric`, `ProfileLine`, `BridgeTargetRow`, `ListSlot`, hand-rolled shimmer + live dots. Single tone map exported from one module; delete the other nine.
- **Copy**: all vocabulary changes in `task-formatters.ts`; new humanized event map as its own module with fallback.
- **Routing**: URL-addressable tabs; keep `use-task-stream` invalidation model; prune dead listeners; wire run page to the stream before advertising live.
- **Greenfield rule**: hard cuts, no compat props or legacy variants left behind; obsolete components deleted in the same change.
- **AGH Impact Audit (for the implementation task)**: Native tools: no impact expected (UI-only; verify no `agh__*` descriptor references task UI copy). Extensibility/hooks: no impact (no contract change); config lifecycle: none. Workspace data isolation: no impact (read models unchanged; verify workspace_id stays in all task queries). Official AGH skill: update `skills/agh/` only if CLI hints/copy embedded there change.
- **QA tracker**: changed user-visible behavior → reset affected `docs/qa/scenarios/` task-detail scenarios to `untested` when the web implementation lands (not for prototypes).
- Design-system/redesign implementation in `web/` must run through the `designer` agent flow with `agh-design` + `ui-craft` and `agh-ui-screenshot` evidence, per repo rules.

---

## 14. Open questions

1. Agents tab: dissolve entirely (recommended) or keep conditionally for fan-out tasks? (§4.1)
2. Fan out: user-facing next to Start run, or operator-only in Inspect? (§4.7)
3. Failed-exhausted tasks: expose "Run again" (manual enqueue) or keep terminal? (§6)
4. Inbox/triage actions (mark read/archive) appear anywhere on the detail page, or stay list-only? (assumed list-only)
5. Should the detail page offer a keyboard/command layer in v1 (e.g. `e` edit, `p` pause) or defer? (Linear parity argues yes; scope argues defer)
6. States gallery as one file or per-state files? (§12)

---

## 15. Next step

1. Review and edit this document directly (TODOs in §4.1, §4.7, §6, §12, §14 need your calls; everything else is proposal-final).
2. When approved, hand it back for generation: the next step builds `tasks/task-detail.html` first (Overview + rail + drawer, in_progress state), then the states file, then run detail, each critiqued against §1.3's target (≥30/40) and the acceptance rules in §5-§11.
3. After prototype approval, the web/ implementation follows §13 as its own tracked initiative.
