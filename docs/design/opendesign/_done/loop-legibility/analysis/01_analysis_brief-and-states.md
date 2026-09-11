# Analysis: brief-and-states

Read-only exploration of the slice `brief-and-states` (ordinal `01`) for the research prompt:

> What visual contracts, incumbent grammar, production seams, and canonical data must the loop-legibility design set honor so the six artboards under docs/design/opendesign/loop-legibility/ can be built without inventing states, copy, chrome, or entities? Extract only design-actionable facts for S1 (tasks list), S4 (run default + needs-you), S5 (DAG + roster), and S6 (runs roster).

## Scope

- Slice question: What must each of the six artboards contain — default-read vs operator-register, every state to stage, the canonical `revisao-paralela` / `fabrica-assistida` data story, locked copy vocabulary, and the visual-contract IDs from task_04/task_05?
- Primary sources: `.compozy/tasks/loop-task-legibility/_uiux.md`, `_user_stories.md`, `_dx.md`, `adrs/adr-001.md`, `adr-002.md`, `adr-003.md`, `task_04.md`, `task_05.md`; `docs/_memory/glossary.md`; `docs/_memory/standing_directives.md` (SD-012) plus `docs/_memory/lessons/L-036-default-read-needs-its-own-gate.md` (named by the dispatch as SD-012 / L-036 only).
- Sources read in full vs. sampled: all ten dispatch paths read in full; L-036 read in full because SD-012 names it as the source lesson. No `web/` and no `docs/design/opendesign/graph-eng/` (adjacent slices 02 and 03).
- Total candidate sources surveyed: 11 (10 named + L-036).

## Overview

This slice is the **brief + state inventory** for the six loop-legibility artboards. It extracts what each board must contain so a designer does not invent surfaces, states, copy, chrome, or entities. Artboards land under `docs/design/opendesign/loop-legibility/` and become the visual contracts `task_04` / `task_05` cite at execution time. The set is six files, not seven: S2 (dashboard/inbox), S3 (task-detail provenance), and S7 (node inventory) have **no dedicated artboard**. S3 reuses S1 revealed-row grammar; S2 and S7 change numbers or labels only.

The binding layout rule is **two registers, no toggle** (ADR-002): one run page; the default register answers a short briefing-test question set in plain words; the operator register (DAG, roster, generations, raw events, ids) sits one disclosure deeper behind **Inspect**. Failure and needs-you never collapse. Loop execution records leave Tasks by default (ADR-001); the live DAG lives only in the operator register (ADR-003). SD-012 / L-036 require every surface to declare its default-read questions and what demotes — machine ids, raw enums, and mechanism copy never lead.

Adjacent slice 02 owns production seams (`web/` components, routes, delete targets). Adjacent slice 03 owns visual tokens and the graph-eng request-grammar lock in CSS. This slice records the **semantic** lock those slices must honor (vocabulary, states, data, VC ids) and does not resolve hex/tone final-call details.

The operator should treat the artboard inventory, the US↔VC map, the two-loop dataset, and the locked words as the only legal content for S1/S4/S5/S6. Anything not in those lists is out of scope for this set.

## Mechanisms / Patterns

- **Six-board set, three no-artboard surfaces:** `_uiux.md` Surface map S1–S7. Boards: `loop-legibility-tasks-list.html` (S1), `loop-legibility-run-default.html` + `loop-legibility-needs-you.html` (S4), `loop-legibility-run-dag.html` + `loop-legibility-run-roster.html` (S5), `loop-legibility-runs-roster.html` (S6). S2: "Artboard: none (no visual change)". S3: "reuse S1's revealed-row grammar; no dedicated artboard". S7: "Artboard: none".

- **Default read vs operator register (no mode toggle):** ADR-002 rejects a simple/advanced switch and a second route. Default register is a different artifact (narrative + derived rollups), not the cockpit minus panels. Operator register is progressive disclosure. S5's own internal lead is the DAG ("where is the run in its shape"); S5 is not a default read.

- **Per-surface briefing questions (≤4 + spend on the run page):**
  - S1 default: "what work do I have, what state is each item in, what needs me" — work items only; zero loop rows (`_uiux.md` S1).
  - S4 default: "what is running · what needs me · how far along · what has it spent · what came out" (`_uiux.md` S4). ADR-002 names four questions (running / needs you / how far / what it produced); S4 adds Usage (tokens · cost · budget · round count · duration) as always-visible spend. Order of the four main-column elements: (1) briefing strip, (2) needs-you, (3) progress, (4) story. Rail: Usage + About. Everything else demotes behind one **Inspect**.
  - S6 default: "which runs need me · which are moving · which finished" (US-010.AC-1/2).
  - S3 (no board): "what is this record, which run owns it, where do I go to act" — action home is the run page.

- **Exclusion + ephemeral reveal (S1):** Server-owned exclusion of coordinator + cell records (ADR-001). Reveal filter is quiet, **hidden by default**, **off on every navigation** (US-002.AC-3; `task_04` req 2; not a `config.toml` key). Revealed rows: loop glyph + plain identity `loop_name · run` / `loop_name · round N · step X`; activation → `/loop-runs/<id>`. Classification is provenance, never title matching (US-001.EC-4).

- **Needs-you never collapses:** US-007.AC-1 — visible in any disclosure state, leads the page. Card anatomy: what is asked, who asks, choices, expiry (`_uiux.md` S4; `task_05` VC-13). Multiple → ordered list with count (US-007.EC-1). Incumbent decision words: exactly `approve / edit / reject / respond`. Pending request on the page inks **warning** (graph-eng lock); danger reserved for failed/quarantined and the bell re-ink (`_uiux.md` Incumbent grammar).

- **Progress = action steps + optional round:** "step 3 of 6" plus round counter only when past round 1 (US-006.AC-1, EC-1). Fan-out = derived count ("7/10 · 1 failed"), never per-item elements (US-006.AC-2, US-011.EC-1). Attempts are metadata ("attempt 2"), never sibling steps (US-006.AC-3). Route-not-taken / never-materialized excluded from totals (US-006.EC-2). Control-only segments contribute no steps (US-006.EC-4). All-parked round states the dominant reason, not a frozen bar (US-006.EC-3).

- **Structure vs status on separate channels:** Node *kind* (agent/gate/collect/route) = glyph/border from the neutral ramp. *Status* = signal icon+text. Never share a channel (`_uiux.md` Design constraints). Kind inventory is additive; boards may stage a subset as story but must not drop a kind as an inventory statement.

- **Pending ≠ not_taken (Safety Invariant 14):** Reachable-but-unmaterialized = `pending` (neutral, hollow/outline + literal word). Durable route evidence = `not_taken` (neutral-dim + route glyph). Taken path lights; `not_taken` only on durable route evidence (`_uiux.md` S5 + signal table).

- **Current-state truth + attempts as metadata:** Recovered nodes read by current state. Per-attempt history in disclosure only. Canceled (strategy or operator) is neutral ramp — cause/actor in text, never alarm color.

- **One dataset, two loops, two named runs:** Boards render `_uiux.md` Canonical data story + `_dx.md` transcripts only. No third loop name, no invented artifact, no invented topology for `fabrica-assistida`. Edge states reuse the same entities.

- **VC ids are per-task, not global:** `task_04` VC-01..VC-06 and `task_05` VC-01..VC-36 reuse the same numbers. Cite as `task_04/VC-0N` or `task_05/VC-NN`.

- **Set chrome (not a seventh board):** `_uiux.md` Set deliverables — `loop-legibility.css`, `DESIGN-NOTES.md` (records needs-you tone resolution inside the graph-eng lock), `index.html`. Each board = final surface + states lab; labs full-viewport; staged fragments at production content width (fluid ≤1240) with 320px rail pixel-true.

### Artboard inventory and states to stage

| Board | Surface | Default read vs operator | States `_uiux.md` names | VC rows (qualify by task) |
| --- | --- | --- | --- | --- |
| `loop-legibility-tasks-list.html` | S1 | Default = work items only. Reveal = distinguished loop rows (opt-in, not a second register). | default + active loop; revealed mixed; revealed-empty; revealed retention-deleted; loop-only true empty; kanban default | task_04 VC-01..VC-06 |
| `loop-legibility-run-default.html` | S4 | Default register. Inspect demotes S5. | running-healthy; queued/admission + reason; watching/dormant; terminal done + artifacts; terminal failed uncollapsed; no-op; canceled + actor; partial outputs; pruned blob; long-run paging; fork/time-travel; budget nearly-exhausted / exhausted; reduced-motion | task_05 VC-01..VC-12 (reduced-motion has no S4 VC — see Open Questions) |
| `loop-legibility-needs-you.html` | S4 | Still default register — never collapses | single card; multiple + count (US-007 also: expiry; resolved elsewhere) | task_05 VC-13..VC-16 |
| `loop-legibility-run-dag.html` | S5 | Operator register. Lead: DAG. | running / terminal / failed / quarantined; pending ≠ not-taken; wide fan-out rollup; deep navigation; node panel; reduced-motion pulse unmount | task_05 VC-17..VC-24 |
| `loop-legibility-run-roster.html` | S5 | Operator register | complete roster incl. healthy; 10-attempt + next-retry; strategy- vs operator-canceled; zero-action-node; fan-out grouped; generation history scored + unscored; crash-partial; pruned session | task_05 VC-25..VC-32 |
| `loop-legibility-runs-roster.html` | S6 | Default read of `/loop-runs` | needs-you first + distinct; empty explains start; dozens-active; transport-degraded ≠ empty | task_05 VC-33..VC-36 |

**S2 / S3 / S7 — no artboard (reuse rule):**

- S2 Tasks dashboard + inbox: no new states; aggregates exclude loop records (US-003.AC-1/2, EC-1/2). Loop escalations → attention bell loop lane + run page, never Tasks inbox.
- S3 Task detail of a revealed loop record: states = cell with attempt history; coordinator; run-gone degrade (US-002.EC-2, US-015.AC-2). Reuse S1 revealed-row grammar + `TaskLoopProvenance` (loop name, "Open run", round, step, item; label = loop execution record). No seventh HTML file.
- S7 Node inventory: no new states; vocabulary alignment only ("step" plain labels); rows keep deep-links; frozen contract stays.

### State → US → VC map

Cite VC as `task_04/VC-xx` or `task_05/VC-xx`.

**S1 `loop-legibility-tasks-list.html`**

| State | US | VC |
| --- | --- | --- |
| Default list, active loop, zero loop rows; work items unchanged | US-001.AC-1, AC-2, AC-3; EC-1 | task_04/VC-01 |
| Revealed mixed: coordinator + cells + work; plain identity; click → run page | US-002.AC-1, AC-2 | task_04/VC-02 |
| Revealed-empty: "no loop records in this workspace" | US-002.EC-1 | task_04/VC-03 |
| Revealed row, run retention-deleted; link states run no longer available | US-002.EC-2 | task_04/VC-04 |
| Loop-only workspace → true Tasks empty (not leaked rows) | US-001.EC-2 | task_04/VC-05 |
| Kanban default: roots = work items only | US-001.AC-1 (board) | task_04/VC-06 |
| Filter hidden; off after reload/navigation | US-002.AC-3 | control on VC-01/VC-02 — not a separate board |
| Cross-workspace never leaks | US-001.EC-3, US-002.EC-3 | not a staged lab (scoping) |
| Human task that mentions a loop in the title stays visible | US-001.EC-4 | not a staged lab |

**S4 `loop-legibility-run-default.html`**

| State | US | VC |
| --- | --- | --- |
| Running-healthy; live update; no machine ids as primary | US-005.AC-1, AC-2 | task_05/VC-01 |
| Queued / admission-parked with reason (e.g. concurrency limit) | US-005.EC-1 | task_05/VC-02 |
| Watching / dormant — calmly idle, what it waits for | US-005.EC-2 | task_05/VC-03 |
| Terminal done + artifacts lead | US-008.AC-1 | task_05/VC-04 |
| Terminal failed, signal outside any accordion | US-008.AC-2 | task_05/VC-05 |
| Terminal no-op — no fake artifact section | US-008.EC-1 | task_05/VC-06 |
| Canceled with actor + when | US-008.EC-2 | task_05/VC-07 |
| Partial / preliminary outputs labeled | US-008.AC-3 | task_05/VC-08 |
| Pruned blob: keep name, "content no longer stored" | US-008.EC-3 | task_05/VC-09 |
| Long-run story paging; full history reachable | US-009.AC-2, EC-1 | task_05/VC-10 |
| Fork / time-travel beat + related-run link | US-009.EC-3 | task_05/VC-11 |
| Budget nearly-exhausted (warning, truthful) | `_uiux.md` S4 (no US id) | task_05/VC-12 |
| Budget exhausted | `_uiux.md` S4 | **no VC row** |
| Reduced-motion on default read | `_uiux.md` S4 | **no S4 VC** (task_05/VC-24 is DAG) |
| Deeper mechanics one disclosure away | US-005.AC-3; ADR-002 | Inspect chrome on every S4 state |
| Progress steps/rounds/fan-out/attempts | US-006.AC-1..3, EC-1..4 | compose into VC-01 and parked/terminal states |
| Story meaning titles; live append | US-009.AC-1, AC-3 | compose into VC-01 / VC-10 / VC-11 |

**S4 `loop-legibility-needs-you.html`**

| State | US | VC |
| --- | --- | --- |
| Single card: ask / asker / choices / expiry | US-007.AC-1, AC-2 | task_05/VC-13 |
| Multiple simultaneous, ordered, with count | US-007.EC-1 | task_05/VC-14 |
| Expiry stated; never auto-retries | US-007.EC-3 | task_05/VC-15 |
| Resolved elsewhere; card shows who answered | US-007.EC-2 | task_05/VC-16 |
| Act in place; story records resolution | US-007.AC-3 | behavior of VC-13 (approve / respond / requeue in US; card words locked below) |

**S5 `loop-legibility-run-dag.html`**

| State | US | VC |
| --- | --- | --- |
| Running node + edge liveness (pulse toward active) | US-011.AC-1, AC-3 | task_05/VC-17 |
| Terminal graph faithful (not last-live frame) | US-011.EC-4 | task_05/VC-18 |
| Failed + quarantined chips (icon+text) | US-011.AC-1 | task_05/VC-19 |
| Pending (reachable) ≠ not-taken (route evidence) | US-011.EC-2; SI-14 | task_05/VC-20 |
| Wide fan-out stays a rollup chip | US-011.AC-2, EC-1 | task_05/VC-21 |
| Deep / wide navigation; activity locatable | US-011.EC-3 | task_05/VC-22 |
| Node panel: attempts, links, verbs | US-011.AC-4; US-014.AC-1; US-015.AC-1..3 | task_05/VC-23 |
| Reduced-motion: edge pulse **unmounts** (not paused) | `_uiux.md` S5; `task_05` req 7 | task_05/VC-24 |
| Auto-center on needs-you | `_uiux.md` S5 | compose into VC-17 |
| Concurrent-intervention / stale-view rejection | US-014.EC-1, EC-2 | **no VC row** |

**S5 `loop-legibility-run-roster.html`**

| State | US | VC |
| --- | --- | --- |
| Complete node × round roster, healthy included | US-012.AC-1, AC-3 | task_05/VC-25 |
| Multi-attempt history + next-retry time | US-012.AC-2 | task_05/VC-26 |
| Strategy-canceled ≠ operator-canceled (cause/actor) | US-012.EC-2 | task_05/VC-27 |
| Zero action nodes (terminal before round 1) | US-012.EC-1 | task_05/VC-28 |
| Fan-out grouped under node + rollup | US-012.EC-3 | task_05/VC-29 |
| Generation history, scored and unscored (no invented scores) | US-013.AC-1, AC-2; EC-2 | task_05/VC-30 |
| Crash-interrupted round, true partial | US-013.EC-1 | task_05/VC-31 |
| Pruned session link: "session no longer available" | US-015.EC-1 | task_05/VC-32 |
| Not-taken node: no fabricated links | US-015.EC-2 | compose into VC-20 / panel |
| Compare / Fork preserved | US-013.AC-2 | compose into VC-30 |

**S6 `loop-legibility-runs-roster.html`**

| State | US | VC |
| --- | --- | --- |
| Needs-you first and distinct; plain outcome; id secondary | US-010.AC-1, AC-2 | task_05/VC-33 |
| Empty roster explains how to start | US-010.EC-1 | task_05/VC-34 |
| Dozens active; needs-you never below terminal | US-010.EC-2 | task_05/VC-35 |
| Transport-degraded (connecting/offline) ≠ empty | `_uiux.md` S6 (smithers `runsLandingState`) | task_05/VC-36 |

Server-owned S6 order before pagination: needs-you → active → terminal (`_dx.md` `compozy loop runs`; no client page-sort, no N+1). Columns: Loop · Status/needs-you · Progress (steps/round) · Started · Duration. Gens / Best / Budget demote to the run page.

### Canonical data story (do not invent)

Authority: `_uiux.md` lines 123–132 + `_dx.md` transcripts. Boards reuse these entities for every edge state.

**Loops (exactly two names)**

| Loop | Role on boards | Known facts only |
| --- | --- | --- |
| `revisao-paralela` | Primary story | Topology: `implementar` → fan-out `revisores` ×3 (`revisor-seguranca`, `revisor-perf`, `revisor-estilo`) → `sintetizador` → `saida`. Gate: `aplicar-correcoes`. Input in golden path: `tema="rate limiting"`. |
| `fabrica-assistida` | Second row on S6 / list filler | Running, step 2/9, started 18:41, duration 13m. **No run id, no node names, no artifacts in sources.** |

**Runs (exactly two ids)**

| Run id | Loop | Snapshot | Usage / outcome |
| --- | --- | --- | --- |
| `looprun-8f3ab2c1d4e5f607` | `revisao-paralela` | Live. Golden-path healthy beat: implementar running 2m14s, "Nothing needs you. 1 of 6 steps done." Canonical needs-you beat: round 1, step 4/6, approval waiting 3m on `revisor-perf` / gate `aplicar-correcoes`. | Briefing JSON: 82400 tokens · $0.31 · 12% budget · 9m40s. Roster row: started 18:32, duration 22m (later clock than briefing — do not invent a third snapshot). Tone `needs_you`. Headline: `Approval "aplicar-correcoes" waiting 3m`. Detail: `The gate is asking whether to apply the reviewers' corrections. 4 of 6 steps done in round 1.` Unblock: `compozy loop approve looprun-8f3ab2c1d4e5f607 --gate aplicar-correcoes`. |
| `looprun-77aa01b2c3d4e5f6` | `revisao-paralela` | Terminal done after 2 rounds | 214.5k / 214500 tokens · $0.87 · 38% · 18m12s. Finished 2026-08-19 17:58 (`outcome.at` 17:58:12Z). Roster started 17:40, duration 18m. Tone `ok`. Headline: `Finished after 2 rounds (18m12s)`. Artifact: `post-final.md` via output `saida` (`availability`: `available`; pruned variant keeps the name). `outcome.cause`: `verified`. |

**Node × round roster for the live run (g1 only in transcripts)**

| Step | State | Attempt | Duration | Session | Notes |
| --- | --- | --- | --- | --- | --- |
| `implementar` | succeeded | 1 | 4m02s | `ses-77120a3f` | |
| `revisor-seguranca` | succeeded | 1 | 1m48s | `ses-9a02bb17` | |
| `revisor-perf` | waiting | 1 | — | `ses-c3f00e42` | approval `aplicar-correcoes` |
| `revisor-estilo` | succeeded | 2 | 2m31s | `ses-5d871c99` | recovered; attempt 1 `tool_error` ended 18:40:52Z; attempt 2 succeeded 18:43:38Z |
| `sintetizador` | queued | — | — | — | post-approve event uses `ses-e1b20f77` |
| `saida` | pending | — | — | — | reachable, not started — distinct from `not_taken` |

Fan-out rollup: `revisores` done 2 / total 3 / failed 0. Timeline seqs shown: 84 `revisor-estilo` succeeded; 85 approval opened; 86 run status running; 87 `gate_verdict` approved; 88 `sintetizador` running.

**Work items (S1 default — not loop records)**

| Id | Status | Title |
| --- | --- | --- |
| `tsk-92ab41` | in_progress | Escrever post sobre memoria |
| `tsk-88cf02` | ready | Revisar landing page |

**Revealed loop-record identity (S1 reveal / CLI)**

- Coordinator: `loop.looprun-8f3ab2c1d4e5f607.coordinator` → `revisao-paralela · run` / title `Loop run revisao-paralela`.
- Cell example: `loop.looprun-8f3ab2c1d4e5f607.g1.node.implementar.0` → `revisao-paralela · g1` / `step implementar`.
- Board primary text: `revisao-paralela · run` and `revisao-paralela · round 1 · step revisor-perf` (`_uiux.md` S1). Ids never lead in default reads.

**Reuse rule for missing states:** queued, watching, failed, canceled, no-op, fork, 10-attempt, 100-item fan-out, 30-run roster, crash-partial, retention-deleted, pruned blob — same two loop names, same two run ids (or `loop_name` absent with the `looprun-…` id kept), same artifact name `post-final.md`. Do not mint `fabrica-assistida` nodes or a third run id.

**Briefing tones and cascade** (`_dx.md`): tones `ok`, `needs_you`, `degraded`, `failed`. First matching verdict wins: approval > quarantine > request > failure > quota/backoff > running > terminal. Web never re-derives a different verdict (`task_05` req 2). Terminal outcomes in copy: done, blocked, failed, exhausted, stalled, canceled, no-op (US-008.AC-1; glossary).

### Locked vocabulary

| Use | Do not use |
| --- | --- |
| **step** in the plain register | `loop cell` as primary copy (operator-only term) |
| **Loop run** as the grouped human label | "Loop coordinator" as primary text |
| loop execution record (revealed label) | title-cased enums (`node_failed`, `NODE_FAILED`) |
| meaning titles ("second reviewer rejected the draft") | mechanics ("event node_failed g2", "14 tool calls") |
| needs-you decisions: `approve / edit / reject / respond` | a second request vocabulary on the run page |
| filter: hidden, ephemeral, off on every navigation | persisted setting / `config.toml` key |
| revealed empty: `no loop records in this workspace` | generic empty when the filter is on |
| run gone: `run no longer available` (`loop_name` omitted) | broken link / error page |
| pruned artifact: keep `post-final.md` + `content no longer stored` | hide the row |
| pruned session: `session no longer available` | error page |
| Inspect (one disclosure) | simple/advanced toggle |
| Open run / Open session / Open record / View child run | live-only hero links |
| Progress: `step 4 of 6` / `step 4/6 · r1`; hide `round 1` on single-pass | fan-out as sibling rows |
| `attempt 2` as metadata on the step | attempt as a sibling step |
| literal word `partial` + coverage numbers | unlabeled leftovers |
| canceled: disposition/cause/actor in text, neutral | danger/alarm color for cancel |
| `pending` vs `not_taken` as distinct words | one "waiting" treatment for both |

**S1 / S4 / S6 briefing questions** — paint these, not extra chrome questions. S4 Usage rail is the spend answer (tokens · cost · budget consumption · round count · duration).

**US-007.AC-3 vs incumbent lock:** story AC lists "approve, respond, requeue". Card choices stay `approve / edit / reject / respond`. `requeue` is a **node verb** (US-014: pause / resume / cancel / kill / requeue / amend / approve / respond) on the operator panel, not a fourth needs-you decision word.

### ADR layout constraints

- **ADR-001:** Loop coordinator + cells **leave Tasks by default** on list/kanban/dashboard/inbox and every structured surface. Reveal is explicit and reversible (distinguish, don't erase). Rejected: one grouped row per run in Tasks; client-side nesting; stopping kernel materialization. Escalations leave the Tasks inbox — bell loop lane + run page own them. Revealed rows still distinguish + link.
- **ADR-002:** **One page, two registers, no toggle, no second route.** Default = narrative answering the briefing questions. Operator = Inspect. Failure + needs-you stay in the default register. Progress = action-node steps in the current generation + round counter; attempts as metadata; parked branches neutral.
- **ADR-003:** Live run DAG is **in scope** and lives in the **operator register**. Same `/nodes` read model as roster and S4 progress. DAG is read-only observability — no editor/drag/palette chrome. Fan-out = count chips, never per-item nodes.

## Relevant Sources

- `.compozy/tasks/loop-task-legibility/_uiux.md:5-17` — default-read discipline, incumbent needs-you grammar, structure-vs-status, attempts, rollups, truthful UI, glossary terms.
- `.compozy/tasks/loop-task-legibility/_uiux.md:20-28` — S1–S7 surface map (artboard vs none).
- `.compozy/tasks/loop-task-legibility/_uiux.md:30-88` — per-surface today/change/default-read/states/artboard.
- `.compozy/tasks/loop-task-legibility/_uiux.md:108-121` — closed signal → tone-family table (14 persisted states + derived `not_taken`).
- `.compozy/tasks/loop-task-legibility/_uiux.md:123-136` — canonical dataset + set deliverables/lab layout.
- `.compozy/tasks/loop-task-legibility/_user_stories.md:39-200` — US-001..010 AC/EC for S1/S4/S6.
- `.compozy/tasks/loop-task-legibility/_user_stories.md:202-278` — US-011..015 AC/EC for S5.
- `.compozy/tasks/loop-task-legibility/_dx.md:8-35` — golden-path transcripts (healthy + needs-you beats).
- `.compozy/tasks/loop-task-legibility/_dx.md:75-165` — roster order, node table, briefing JSON, tones, artifacts.
- `.compozy/tasks/loop-task-legibility/adrs/adr-001.md:18-21` — exclusion + reveal, not erasure.
- `.compozy/tasks/loop-task-legibility/adrs/adr-002.md:16-17` — two registers, no toggle.
- `.compozy/tasks/loop-task-legibility/adrs/adr-003.md:16-17` — DAG in operator register.
- `.compozy/tasks/loop-task-legibility/task_04.md:34-47` — task_04 VC-01..VC-06.
- `.compozy/tasks/loop-task-legibility/task_05.md:35-76` — task_05 VC-01..VC-36.
- `docs/_memory/glossary.md:77-91` — Loop terminals/live states; step / Loop run / loop cell.
- `docs/_memory/standing_directives.md:189-204` — SD-012 default-read contract.
- `docs/_memory/lessons/L-036-default-read-needs-its-own-gate.md:17-26` — briefing test as a gate.

## Transferable Patterns

- **Six filenames + no-artboard reuse →** S1/S4/S5/S6 production. Do not add `loop-legibility-dashboard.html`, `loop-legibility-task-detail.html`, or `loop-legibility-node-inventory.html`. S3 provenance follows task_04/VC-02 grammar.
- **Two-register page, Inspect, never a toggle →** `loop-legibility-run-default.html` composition and S5 boards as disclosed depth. Replaces today's 9+ section cockpit (`_uiux.md` S4 Today).
- **US↔VC staging table →** each board's states lab. One lab panel per VC row in `task_04` / `task_05`. Host chrome + `@compozy/ui` identity stay; reference parity is visual language only (`task_04` / `task_05` authorized differences).
- **Canonical two-loop dataset →** every board's fixture. Running-healthy uses the same `looprun-8f3ab2c1d4e5f607` earlier beat (implementar / 1 of 6); needs-you uses the later beat (step 4/6 + gate). Terminal/canceled/failed/pruned derive from `revisao-paralela` / `fabrica-assistida` history — no extra loop names.
- **Locked words (step / Loop run / approve-edit-reject-respond / ephemeral filter) →** all six boards' labels, chips, empty states, and needs-you cards. Replaces enum-as-label and a forked HITL vocabulary.
- **ADR-001 exclusion + hidden filter →** S1 default vs reveal labs (task_04/VC-01 vs VC-02). Replaces client nesting / grouped-row-in-Tasks (rejected alternatives).
- **ADR-003 DAG-in-operator-register + pending≠not_taken →** `loop-legibility-run-dag.html` (task_05/VC-17..VC-24). Replaces exception-only node rows and a static definition DAG as the run view.
- **Server-owned S6 GROUP_ORDER →** `loop-legibility-runs-roster.html` (task_05/VC-33..VC-36). Replaces client sort and KPI-led Outcome·Gens·Best columns.
- **SD-012 / L-036 briefing test →** default-read labs must pass with Inspect collapsed: S1 work-only; S4 status + needs-you + progress + spend + outcome; S6 needs-me / moving / finished.

## Risks / Mismatches

- **Inventing a seventh board for S2/S3/S7** would violate `_uiux.md` "Artboard: none" / reuse rules and `task_04`'s "S3 follows VC-02" line.
- **Simple/advanced toggle or a second consumer route** would violate ADR-002 (rejected alternatives 1 and 2) and PRODUCT.md approachable-first as cited there.
- **DAG on the default read** would violate ADR-002 demotion list and ADR-003 ("operator register of the run page").
- **Editor chrome on the run DAG** (drag, palette, edit) would violate ADR-003 Alternative 2 rejection and `_uiux.md` "read-only observability".
- **Forking needs-you vocabulary or inking pending requests as danger on the page** would violate `_uiux.md` incumbent grammar (warning on the page; danger = failed/quarantined + bell).
- **Per-item fan-out nodes or attempt-as-sibling-steps** would violate US-011.EC-1 / US-006.AC-3 and ADR-003 fan-out risk note.
- **Painting `pending` and `not_taken` as one dim state** would violate Safety Invariant 14 and task_05/VC-20.
- **Inventing `fabrica-assistida` topology, a third loop, extra artifacts, or a run id for `fabrica-assistida`** would violate the canonical data story ("never invented ones").
- **Leading default reads with `looprun-…` / `loop.…` ids** would fail E2E-012 (`task_05` Tests) and US-005.AC-1 / US-010.AC-1.
- **Persisting the S1 reveal filter** would violate US-002.AC-3 and `task_04` "deliberately not a persisted setting".
- **Collapsing failure or needs-you behind Inspect** would violate ADR-002, US-007.AC-1, US-008.AC-2, SD-012.
- **Color-alone chips** would violate `_uiux.md` WCAG floor and `task_05` req 7.
- **VC number collision:** citing "VC-01" without `task_04` vs `task_05` maps the tasks-list default to the run-page healthy state.
- **S4 reduced-motion and S4 exhausted-budget and S5 US-014 conflict states have `_uiux.md` "states to design" but no VC row** — staging them as extra lab panels without a VC id, or dropping them, both risk contract drift (see Open Questions).
- **Briefing question count:** ADR-002 says four questions; `_uiux.md` S4 default-read line adds spend. Dropping Usage from the default rail would fail `task_05` req 1 and E2E-012; treating spend as a sixth main-column section would re-cockpit the page.
- **US-007.AC-3 `requeue` on the needs-you card** would fork the locked `approve / edit / reject / respond` set. Keep requeue on the node panel.
- **Copy/hex final call** belongs to slice 03 + `COPY.md` / tokens. This slice must not invent English replacements for the Portuguese work-item titles in `_dx.md`.
- **Live-run duration 9m40s (briefing) vs 22m (roster)** — two clocks in the same dataset. Do not invent a reconciliation; pick the source that owns the surface (Usage rail ← briefing JSON; S6 row ← `compozy loop runs` table) or flag for the parent.

## Open Questions

- S4 lists **reduced-motion** as a state to design, but `task_05` only stages reduced-motion as task_05/VC-24 on `loop-legibility-run-dag.html`. Does `loop-legibility-run-default.html` need a lab panel with no VC id, or is default-read motion out of this set?
- `_uiux.md` S4 lists **budget exhausted** alongside nearly-exhausted; `task_05` has only VC-12 (nearly-exhausted). Stage exhausted on the same board without a VC, or treat VC-12 as the only budget lab?
- S5 lists **concurrent-intervention conflict** and **stale-view rejection** (US-014.EC-1/2) with no `task_05` VC. Are those implementation-only (toast/error), or must the DAG/roster board stage them?
- **`fabrica-assistida` has no `run_id` and no topology** in `_uiux.md` / `_dx.md`. S6 may need secondary id text. Parent must decide: omit the id, or supply one outside this slice — this slice must not mint it.
- **Two durations for `looprun-8f3ab2c1d4e5f607`:** briefing `9m40s` vs roster `22m`. Which number does each board paint?
- **ADR-002 four questions vs S4 five-part default-read line (includes spend):** confirm Usage stays rail-only so the main column stays four elements.
- Incumbent **wait-kind sentences** are locked in graph-eng (out of this slice). Slice 03 must supply the exact strings before needs-you/wait copy is drawn; this slice only knows the decision words and the `aplicar-correcoes` briefing sentences.
- Node-kind glyph inventory is pointed at `web/src/systems/loops/lib/loop-palette.ts` + `loop-body-dag.tsx` — **not read here** (slice 02). Designer must not drop kinds; the concrete glyph set comes from that adjacent slice.
- Signal palette hex proposal in `_uiux.md` is "final call: design pass" and must resolve **inside** the graph-eng lock — owned by slice 03, not this file.

## Evidence

- `/Users/pedronauck/Dev/compozy/compozy/.compozy/tasks/loop-task-legibility/_uiux.md`
- `/Users/pedronauck/Dev/compozy/compozy/.compozy/tasks/loop-task-legibility/_user_stories.md`
- `/Users/pedronauck/Dev/compozy/compozy/.compozy/tasks/loop-task-legibility/_dx.md`
- `/Users/pedronauck/Dev/compozy/compozy/.compozy/tasks/loop-task-legibility/adrs/adr-001.md`
- `/Users/pedronauck/Dev/compozy/compozy/.compozy/tasks/loop-task-legibility/adrs/adr-002.md`
- `/Users/pedronauck/Dev/compozy/compozy/.compozy/tasks/loop-task-legibility/adrs/adr-003.md`
- `/Users/pedronauck/Dev/compozy/compozy/.compozy/tasks/loop-task-legibility/task_04.md`
- `/Users/pedronauck/Dev/compozy/compozy/.compozy/tasks/loop-task-legibility/task_05.md`
- `/Users/pedronauck/Dev/compozy/compozy/docs/_memory/glossary.md`
- `/Users/pedronauck/Dev/compozy/compozy/docs/_memory/standing_directives.md`
- `/Users/pedronauck/Dev/compozy/compozy/docs/_memory/lessons/L-036-default-read-needs-its-own-gate.md`
