# Loop Run Detail — Redesign Implementation Spec

- **Date:** 2026-07-23
- **Status:** Ready for implementation
- **Scope:** the loop run detail page only (`/loop-runs/$runId`). Runs list, loop detail, and loop editor are out of scope.
- **Canonical visual references:** `loops/loop-run-detail.html` (running) and `loops/loop-run-detail-states.html` (needs-approval · watching · paused · failed). Iterate on those files, don't regenerate.
- **Authority chain:** daemon contract (`openapi/agh.json`, `internal/api/contract/loops.go`) > these prototypes > this spec > current `web/` code. On any conflict about *data*, the daemon wins; about *presentation*, the prototypes win.

## 1. Why

The shipped page (`web/src/systems/os/apps/loops/loop-run-detail-location.tsx` + `web/src/systems/loops/components/run-page/`) is operator-first: 5 policy-tagged meters, a DSL-shaped node spine with template refs, an embedded channel transcript, an 11-status legend, and a raw event rail. Under the Agent OS directive the page must read as a plain-language story for non-operators, while every operator fact stays reachable (Inspect drawer), and every rendered element stays truthful to the daemon.

Design principles locked with Pedro:

1. The page answers four questions in order: **Progress** (goal + how far), **Happening now**, **What happened**, **What happens next**.
2. **Color carries state only.** Accent = running, success = clean, warning = revise/approval, danger = real failures. No tinted meters, no decorative color.
3. **Operator vocabulary leaves the surface.** Templates, CEL, policies, fan-out params, cursors → Inspect drawer. Runtime truth survives as small mono micro-labels on story rows (`gate_verdict · revise`).
4. **"Generation" renders as "Round"** in copy; micro-labels keep `gen N`.
5. **Controls map to daemon-accepted transitions** (§7). Nothing renders for a state the run is not in.

## 2. Page anatomy (new IA)

```
[topbar slot]  status pill · Pause/Resume/Stop (existing LoopRunControls) · ⋯ menu
[subhead]      run_id · watched subject (from inputs) · started relative · elapsed
[main col]                                  [rail 320px]
  Needs you (only needs-approval)             Usage   (time/tokens/cost/rounds + note)
  What went wrong (only failed)               About   (loop, version, inputs, origin, workspace, id)
  Progress (goal + done-when + group bar)     footer: Inspect · definition digest
  Happening now (live card)
  What happened (story timeline)
  What happens next (quiet note)
[Inspect drawer]  operator sheet (right, 560px)
```

Geometry, tokens, and component shapes are exactly those of the prototypes (agent-detail token lineage; 320px rail; flat spine; `scard` outcome cards). Transcribe, don't recreate.

## 3. Component change matrix

All paths relative to `web/src/systems/loops/` unless noted. Per greenfield policy these are hard cuts — no compat props, no dual rendering.

| Current | Action | Replacement / notes |
|---|---|---|
| `components/run-page/loop-run-contract-header.tsx` | **Delete** | `LoopRunProgressPanel` (new): `contract.goal` as title, `contract.definition_of_done` as one-line sub, group progress bar (§5.2), meta line. Verification rows, terminal_states chips, reattempt/start metadata move to Inspect. |
| `components/run-page/loop-run-meters.tsx` + `lib/loop-run-meters.ts` | **Delete** | `LoopRunUsageRail` (new, rail): 4 `prow` rows — Time `elapsed / budget_wall_sec`, Tokens `tokens_used / budget_tokens`, Cost `~$ estimate`, Rounds `generation / iteration_cap (∞ when ≤0)` — plus one policy note sentence derived from `budget_on_exceeded` (§6 copy). Keep `ratioTone` warn/danger coloring on values ≥90% / ≥100%. |
| `components/run-page/loop-generation-timeline.tsx`, `loop-generation-card.tsx`, `loop-node-row.tsx` | **Delete** | `LoopRunStoryTimeline` (new): flat, newest-first, event-derived rows (§5.3). No per-generation collapsible cards, no node-class badges, no template refs. |
| `components/run-page/loop-gate-card.tsx` | **Rework** | Verdict becomes a story row: title + sub with confidence and `blocking_issues` (id + note, mono ids). Full per-criterion table moves into the Inspect drawer (rendered from the latest `gate_verdict` payload). |
| `components/run-page/loop-run-channel.tsx` | **Delete from this page** | The embedded transcript was the top confusion driver. `channel_msg` events do not render rows. If a network channel exists, About rail may show a "Channel" link row; the transcript lives on the network surface. |
| `components/run-page/goal-turn-timeline.tsx` | **Rework** | Only when the graph has `goal` nodes: a "Turns" disclosure inside the corresponding story row, reusing `/turns` paging. Each turn links its `session_id` to the session route (today it renders an unlinked MonoId — upgrade it). |
| `components/run-page/loop-run-events-rail.tsx` | **Delete** | The humanized story timeline *is* the event feed. Raw frames (kind/seq/payload) become an "Events" section in Inspect for operators. |
| `components/run-page/loop-watch-events-panel.tsx` | **Rework** | Parked read-model feeds the `watching` now-card: one sentence + `last_wake_at` relative + poll cadence when the watch spec declares one. Subscriptions list + per-stream cursors move to Inspect. |
| `components/run-page/loop-run-facts.tsx` | **Rework** | `LoopRunAboutRail`: Loop (link), Version `v{definition_version} · pinned`, watched subject rows from `inputs` (e.g. PR), Started by (humanized `started_origin_kind/ref`), agent input binding when present, Workspace, Run id + copy. Concurrency moves to Inspect. |
| `components/run-page/loop-status-legend.tsx` | **Delete** | No legend. Status meaning is carried contextually (chip + now-card copy). |
| `components/run-page/loop-approval-gate.tsx` | **Rework** | "Needs you" `scard` (warning tone) at the top of main col: title/prompt from `needs_approval` payload, facts row from payload `facts[]` (fallback: usage snapshot), actions **Approve & resume** / **Request changes** / **Reject & halt** → existing approve mutation with `gate_id`. |
| `components/run-page/loop-failure-detail.tsx` + `loop-node-failure-detail.tsx` | **Rework** | "What went wrong" `scard` (danger tone): `cause` as body, `recovery` as second line, mono micro with `status_changed` cause code. Actions: **Start a new run** (`POST /loops/:name/run` with same inputs) and a contextual fix link when recovery names a surface (e.g. Vault). Node-level failures render as danger story rows with the parsed `action_failure` cause as sub. |
| `components/run-page/loop-run-controls.tsx` | **Keep** | Visibility logic already matches §7. Add the ⋯ overflow (View graph, View definition, Inspect run). |
| `components/loop-status-pill.tsx` | **Keep** | Unchanged. |
| `use-loop-run-page.ts`, `hooks/use-loop-stream.ts`, `lib/loop-events.ts`, `lib/query-options.ts`, adapters | **Keep** | Same endpoints, SSE reducer, invalidation, token overlay (`Math.max`), keyed-by-runId reset, terminal auto-close. The redesign is a new presentation layer over the same view-model, plus the story adapter below. |
| `lib/loop-timeline.ts` | **Rework** | Becomes `lib/loop-run-story.ts` — the event→story adapter (§5.3). |

New components live in `components/run-page/` with the same naming convention; reuse `@agh/ui` primitives per repo rule (no shadowing).

## 4. Data bindings (all first-class; nothing invented)

| UI element | Source |
|---|---|
| Status chip, controls gating | `run.status` (11-value enum), `run.pause_requested` |
| Subhead id / started / elapsed | `run.id`, `run.started_at` (elapsed ticks client-side; parked/terminal states show `created_at → last_progress_at` span) |
| Watched subject ("Watching PR #128") | `run.inputs` (loop-declared primary input; generic fallback: first scalar input rendered as `key value`) |
| Goal title / done-when | `executed_definition.contract.goal` / `.definition_of_done` (fallback to `useLoop` when no pinned definition — existing behavior) |
| Group progress bar | latest generation's `outputs[]` grouped by `item_index` (§5.2) |
| "N open points" | latest `gate_verdict.blocking_issues[]` length (SSE reducer already keyed by nodeId) |
| Story rows | SSE `/events` + persisted `loop_run_events` (kinds: `status_changed`, `generation_started`, `node_running`, `node_succeeded`, `node_failed`, `gate_verdict`, `needs_approval`, `goal_turn_*`) |
| Happening now | newest `node_running` without a terminal node event; elapsed from that frame's `at` |
| "View task run" link | that node output's `task_run_id` → task-run detail route. Child loop runs: `child_loop_run_id` → `/loop-runs/$runId` (keep existing link) |
| Usage rows | `tokens_used` (with `token_tick` overlay), `budget_tokens`, `budget_wall_sec`, `generation`, `iteration_cap` |
| Cost | client-side estimate `tokens × LOOP_COST_PER_1M_TOKENS`, rendered `~$X.XX estimate`, never a bar/cap (ADR-017) |
| Usage note | `budget_on_exceeded`: `escalate` → "pauses and asks you" · `halt` → "stops as exhausted" |
| About rows | `loop_name`, `definition_version`, `started_origin_kind/ref`, `started_by_kind/ref`, `workspace_id`, `run.id` |
| Watching card | `watch_events.last_wake_at`, `watch_events.subscriptions`, poll cadence from the node's `watch` map when present |
| Approval card | `needs_approval` payload (`gate_id`, `title`, `facts[]`), decisions → `POST …/approve {decision, gate_id}` |
| Failure card | live `failure` (`code`, `cause`, `recovery`) or terminal `status_changed` payload |
| Inspect drawer | `executed_definition` (stop_when, verification[], reattempt_strategy, concurrency, watch spec, fan-out params, no_progress, terminal_states), `definition_digest`, latest `gate_verdict.criteria[]`, watch cursors, raw event frames |

## 5. The three engineered pieces

### 5.1 Genericity rule
The prototypes tell the `reviews-watch` story; the implementation must render **any** loop. Every string below is a template over real fields — domain flavor comes only from node ids, inputs, and event payloads. Never special-case a loop name.

### 5.2 Group progress bar
- If the latest generation has fan-out outputs (>1 distinct `item_index` for one node): render one segment per branch, flex-weighted equally (batch sizes are not first-class — do **not** invent weights). Segment state from output status: `succeeded/reused` → success · `running/enqueued/awaiting_*` → accent-dim · `failed` → idle-neutral (redo pending) or danger in terminal `failed` runs · `pending` → idle.
- Meta line: `{clean} of {total} fix groups clean` + status clause; right slot: blocking-issues count from the last verdict, else the run-phase clause (`waiting for …`, `paused with the run`).
- No fan-out in the graph → hide the bar; the meta line alone summarizes (`Round {n} · {blocking} open points from the last check`).

### 5.3 Event → story adapter (`lib/loop-run-story.ts`)
Newest-first; one row per meaningful frame; every row carries `{tone, icon, title, sub?, at, micro}` where `micro` = `{kind} · {qualifier}` (mono, faint). Humanize node ids (`fetch_issues` → "fetch issues") for generic titles.

| kind | tone | title template | notes |
|---|---|---|---|
| `generation_started` | accent (neutral once superseded) | `Round {generation} started` + reattempt clause when `failed_only` and gen>1 (`— redoing the failed group(s)`) | micro `generation_started · gen {n}` |
| `gate_verdict` pass | success | `Check passed — everything handled` | sub: `Verdict pass · confidence {c}` |
| `gate_verdict` revise | warning | `Check: not clean yet` | sub: `Verdict revise · confidence {c}.` + blocking issues as `{id} — {note}` (mono ids); micro `gate_verdict · revise` |
| `node_succeeded` | success | `Finished {node label}` (grouped: consecutive same-node branch successes collapse into one row `{node label} — {k} of {n} clean`) | micro `node_succeeded · {node_id}[{ix}]` |
| `node_failed` | danger | `{node label} came up short` | sub from parsed `action_failure` cause; failed-only clause when applicable |
| `node_running` | accent | feeds **Happening now**, not a timeline row | current step card with elapsed + View task run |
| `needs_approval` | warning | `Asked for your approval` | sub from payload title; micro `needs_approval · {gate_id}` |
| `status_changed` | by target | wake: `A new {source} event woke the run` · park: `Went back to watching` · pause/resume/terminal per transition | micro `status_changed · {from} → {to}`; terminal `failed` feeds the danger scard |
| `token_tick` | — | no row; updates Usage tokens/cost | |
| `channel_msg` | — | no row on this page | |
| `goal_turn_*` | — | fold into the goal node's Turns disclosure | |

"What happens next" note: derived from the pinned graph — remaining downstream nodes of the current generation in topological order, humanized into one sentence, ending with the `stop_when`/watch clause when the graph has a watch source. Static per definition; no LLM, no invention.

### 5.4 Elapsed / times
Run elapsed ticks only while status is `running`; `watching`/`paused`/`needs-approval` show the frozen active span; terminal shows duration. Timestamps get absolute `title` tooltips.

## 6. Copy (English, COPY.md register)

- Round, fix group, check, "open points", "asks you first". Forbidden on the main surface: generation, gate, fan-out, batch, CEL, template refs, policy keys.
- Usage note (escalate): `Cost is an estimate (tokens × rate), never a cap. If a limit is reached, this run pauses and asks you — it doesn't fail silently.` (halt variant: `…this run stops as exhausted.`)
- Watching note: `If the next review comes back clean, the run finishes as Done. If it raises new comments, a new round starts automatically.` — generalized from `stop_when` + watch source.
- Micro-labels are verbatim runtime identifiers — never translated or prettified.

## 7. Status × surface matrix

| status | chip tone | head controls | extra card | now-card |
|---|---|---|---|---|
| queued | info | Stop | — | "Waiting for a slot" (concurrency `queue` only) |
| running | accent pulse | **Pause** (disabled if `pause_requested`) · **Stop** | — | active step + View task run |
| watching | info pulse | Stop | — | watching card (last wake, cadence) |
| needs-approval | warning pulse | Stop | Needs you (3 decisions) | — |
| paused | neutral | **Resume** (primary) · Stop | — | paused card |
| done | success | ⋯ only | success summary row in timeline | — |
| no-op | neutral | ⋯ only | quiet note "Ran, nothing to do" | — |
| blocked | warning | ⋯ only | scard warning with cause/recovery | — |
| failed | danger | ⋯ only | What went wrong (Start a new run) | — |
| exhausted | warning | ⋯ only | scard: which limit, `halt` policy, Start a new run | — |
| stalled | neutral | ⋯ only | scard: no-progress window, repeated blocking ids | — |

Terminal runs: no run controls (current behavior), polling stops, SSE closed (existing).

## 8. Acceptance checklist

- [ ] Zero occurrences on the main surface of: `{{`, `CEL`, `fan-out`, `batch_size`, `max_parallel`, `on_exceeded`, status legend, raw event kinds (outside mono micro-labels).
- [ ] Every rendered value traces to a field in §4; cost always `~$` + `estimate`; unbounded caps render `∞`.
- [ ] Controls exactly per §7 for all 11 statuses; approve/request_changes/reject wired with `gate_id`; no Retry, no cancel.
- [ ] Embedded channel transcript gone from the run page.
- [ ] Session reachability: running action nodes link via `task_run_id`; goal turns link `session_id`; child runs keep `/loop-runs/$runId` links.
- [ ] Inspect drawer exposes everything deleted from the surface (verification, policies, watch spec, criteria table, raw events, digest).
- [ ] Works for a loop with no fan-out and no watch source (bar hidden, generic story rows) — test with a minimal fixture.
- [ ] Deleted files removed (no dead exports); `bunx turbo run lint typecheck test --filter=./web` green.
- [ ] QA impact flagged: reset/add `untested` scenarios in `docs/qa/scenarios/` for the loop-run page redesign.

## 9. Open questions (implementer may decide, defaults given)

1. Task-run link target route — use the canonical task-run detail route; default: link, don't embed.
2. Criteria table in Inspect — render from latest `gate_verdict` payload (default) vs. persisted `loop_gate_decisions` history (nice-to-have later).
3. `no-op` / `done` celebratory copy — keep quiet (default), no confetti-style UI.
