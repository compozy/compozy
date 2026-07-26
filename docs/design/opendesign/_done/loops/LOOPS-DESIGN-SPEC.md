# Loops - Design Specification

> **What this document is.** A complete, implementation-facing specification of the
> AGH "Loops" feature *as designed* in the high-fidelity HTML mockups. It records the
> screens, their components, the data each surface reads/writes, the interactions, and
> the domain model the UI assumes.
>
> **What it is for.**
> 1. **PRD / TechSpec review** - section 9 lists every place where the design surfaces
>    a control, metric, default, or flow that the PRD/TechSpec must confirm, define, or
>    change. Treat section 9 as the actionable diff against the spec.
> 2. **Implementation context** - sections 4 to 8 are the build brief: component
>    inventory, per-screen data contracts (what the daemon must expose), states, and
>    interaction behavior.
>
> **Sources of truth (do not contradict).**
> - Spec: `/Users/pedronauck/dev/compozy/agh/.compozy/tasks/loops/` (`_prd.md`,
>   `_techspec.md`, `product-ux.md`, `use-cases.md`, `requirements.md`, `adrs/`, `analysis/`).
> - Design system: `agh/DESIGN.md` + `PRODUCT.md` (tokens from `packages/ui/src/tokens.css`).
> - Design intent + history: `LOOPS-HANDOFF.md` (this folder).
> - The mockups: `loops-index.html`, `loops-catalog.html`, `loop-detail.html`,
>   `loop-run-form.html`, `run-detail.html`, `runs.html`, `loop-editor.html`,
>   `loop-configure.html`.
>
> **Status legend used below:** `[shipped-in-spec]` traced to spec/ADR · `[design-assert]`
> a deliberate design choice consistent with spec · `[VERIFY]` design surfaces something
> the spec must confirm or define · `[UNCONFIRMED]` not found in spec, flagged.

---

## 1. Scope of the design

Eight self-contained screens, each a content + in-page-header reference (the 56px
workspace rail and 244px nav sidebar live in the real app shell and are intentionally
absent from the mockups). All eight share one token block, one `.shell`/`.main` layout,
one `.pill--*` status vocabulary, and one type scale.

| # | File | Surface | State |
|---|------|---------|-------|
| 1 | `loops-index.html` | Overview / launcher (design-doc only; not a product route) | Built |
| 2 | `loops-catalog.html` | Loops catalog (the home of the feature) | Built |
| 3 | `loop-detail.html` | A single Loop's definition page | Built |
| 4 | `loop-run-form.html` | Run a Loop (the hero arrive-and-use path) | Built |
| 5 | `run-detail.html` | Live run monitor (the heaviest surface) | Built |
| 6 | `runs.html` | Runs history (global) | Built |
| 7 | `loop-editor.html` | Visual DAG builder (fork & edit) | Built |
| 8 | `loop-configure.html` | Configure sheet (no-fork tweaks) | Built |

---

## 2. Product model (the design's mental model)

- **Two nouns only:** **Loops** (the catalog of definitions) and **Runs** (their
  executions). No third noun. `[shipped-in-spec]`
- A **Loop** = a **contract** (goal, definition-of-done, verification, stop conditions)
  + a **body** (a static DAG of typed nodes) + typed **declared inputs**. It executes on
  AGH's autonomy kernel, not a second executor. `[shipped-in-spec]`
- **Arrive-and-use is the hero path:** pick a Loop, fill an auto-generated input form,
  Run, watch it iterate live. Built-ins run with zero authoring. `[shipped-in-spec]`
- Runs iterate across **generations**. Default re-attempt strategy `failed-only` (re-runs
  failed/pending nodes plus their transitive downstream dependents, carrying succeeded
  outputs forward as read-only); override `full-body`. `[shipped-in-spec]`
- **Terminal outcomes are explicit and honest:** `done` · `no_op` · `blocked` · `failed` ·
  `exhausted` · `stalled` (ADR-022). `no_op` = the run/tick completed with nothing to do
  (neutral, never fake work); `blocked` = an external dependency prevents progress
  (warning family), distinct from `stalled` (work possible, no progress made).
  **`needs-approval` is a LIVE pause, not terminal.** Live states:
  `running / watching / needs-approval / paused / queued` (the run enum is exactly these
  11, ADR-021/022). `queued` is a live run status (a deferred start under
  `concurrency: queue`, promoted FIFO when the active run reaches terminal); `ready` and
  `awaiting_child` are node-level states only, never run statuses. `[shipped-in-spec]`
- **Authoring is fork-and-edit only** (ADR-008): the builder opens an existing Loop or
  built-in. There is no blank-canvas from-scratch builder in v1. `[shipped-in-spec]`
- **Start bindings (ADR-007):** a Loop declares how it may be initiated via the DSL
  `start[]` allowlist (`manual|cli|http|uds|trigger|schedule|webhook|network|extension|native_tool`).
  Event-driven starts (trigger/schedule/webhook) are carried by AGH's existing automation
  primitives: a Trigger or Job gains a discriminated target, `Run agent` (today) or `Run loop`
  `{workspace, loop, static inputs, event-payload → input mapping}` (spec-only, TechSpec §9.14).
  A watch-source is a node INSIDE the body, never a start binding; the two concepts are never
  merged. `[shipped-in-spec]`

---

## 3. Design system (as applied)

### 3.1 Atmosphere and hard rules
Warm near-black operator canvas; quiet, dense, intentional; flat depth; a single scarce
accent; signal colors as desaturated tint + text (never solid banners). The golden rule:
**color carries STATE, not category.** Node *class* (action/control/source) is a neutral
mono label; only run/node *state* gets color.

Hard rules honored across all screens (and required of any new screen): one accent used
sparingly; no side-stripe accent borders; no gradient backgrounds; flat depth (shadows
only on overlays/modals via `--shadow-overlay`); no em dashes in copy; real product UI
only (no demo/viewport/theme toggles, no metadata/process cards).

### 3.2 Tokens (copy verbatim into every screen `:root`)
```
surfaces  --rail #0c0b0b · --canvas #131211 · --canvas-soft #1a1918 · --canvas-tint #1c1b1a · --elevated #232220
text      --fg #ececef · --fg-strong #f6f6f8 · --muted #9a9a9f · --subtle #76767c · --faint #545458
lines     --line rgba(255,255,255,.055) · --line-soft .03 · --line-strong .09
accent    --accent #e8572a · --accent-hover #d14e25 · --accent-strong #f6874f · --accent-ink #17110f · --accent-tint .10
status    success #5fbf85/.08 · warning #d6a647/.08 · danger #e0635a/.09 · info #8e8eb5/.12 · neutral #7a7a80/.06
glaze     --row-hover .022 · --row-selected .03 · --input-fill .025 · --badge-fill .05 · --btn-fill .04 · --bar-fill .18
type      Inter Variable (cv01,ss03) + JetBrains Mono · body 13.5px · detail-h1 22px/-.028em · section label 10.5-11px upper · mono ids ~11px
radii     4/5/6/8/10/14 · pill 9999 · motion 140ms cubic-bezier(.2,0,0,1)
```

### 3.3 Shared status vocabulary (`.pill--*`, tint bg + saturated text)
| State | Token | Pulse | Kind |
|-------|-------|-------|------|
| running | accent | yes | live |
| watching | info | yes | live |
| needs-approval | info | no | live pause |
| paused | neutral | - | live |
| queued | neutral | no | live (deferred start under `concurrency: queue`) |
| done | success | no | terminal |
| no_op | neutral | no | terminal (ran, nothing to do) |
| blocked | warning | no | terminal (external dependency) |
| failed | danger | no | terminal |
| exhausted | warning | no | terminal |
| stalled | neutral | no | terminal |

The run status enum is exactly these 11 states (ADR-013/017/021/022). `queued` renders as
a run status only for a deferred start under `concurrency: queue`, promoted FIFO when the
active run reaches terminal (ADR-021). `ready` and `awaiting_child` are node-level pills
only (per-node state on the run timeline and in the editor,
`loop_generation_outputs.status`; `awaiting_child` marks a `run-loop await` node parked
until its child loop terminates, ADR-021); neither renders as a run status.

In dense tables a 6px state dot inside the pill is enough; the pulse is reserved for live
states and wrapped in `prefers-reduced-motion`.

### 3.4 Shared component vocabulary (build these once, reuse everywhere)
Topbar (inventory: icon + title + count; detail: breadcrumb); `.btn` / `.btn--primary` / `.btn--ghost` / `.btn--danger` /
`.btn--icon`; inventory trailing = ghost secondary + primary CTA (not outline); `.pill--*`; `count-chip`; section label (uppercase + hairline);
`.panelbox`; key-value rows (`.kv`); reui Filters + SearchInput in `.listing-toolbar` (catalog + vault);
listing view `PillGroup` (`.pill-group`); category is a Filters field (not pills);
status legend; form controls (`.input`, `select.input`, `.textarea`, `.switch`,
agent-picker, pill-group/segmented, `<details>` collapsible); meters/progress bars; node
spine (run timeline) and node canvas (editor); gate card (flat tint); embedded channel;
approval gate.

---

## 4. Screen specifications

> Each screen: **Purpose · Route/IA · Layout · Key components · Data shown ·
> Interactions · Spec terms surfaced · Data contract (what the daemon must expose).**

### 4.1 `loops-catalog.html` - Loops catalog
- **Purpose.** The home of the feature: browse built-in and forked Loops, filter, see
  last outcome + success rate, and launch a run. Arrive-and-use entry point.
- **Route/IA.** Inventory topbar (Vault shell): icon + `Loops` + count · trailing `Runs`
  (`btn--ghost btn--sm`) + `New from template` (`btn--primary`). No breadcrumb on this page.
  `[VERIFY]` "New from template" = fork-and-edit entry (ADR-008), not a blank builder.
- **Layout.** Page head (title + `count-chip` + meta line) · listing toolbar · grouped
  listing (Built-in, Custom). Default view = **rows**; optional **cards** via PillGroup.
  Shared contract: `LISTING-STANDARD.md` (reuse for skills / bridges catalogs).
- **Key components.**
  - **SearchInput** — compact (26px / ~200px), placed **before** Filters in the listing
    toolbar (`/` shortcut). Not in the topbar.
  - **Filters** — packages/ui reui Filters (same HTML recreation as `vault-redesign.html`):
    chip bar with Add Filter → field menu → value submenu. Fields: `kind` (select),
    `category` (select), `status` (select, 10-state enum), `mode` (select: delivery|watch),
    `name` (text: contains / starts with / is). Replaces the old kind `segment` + category
    pills. AND semantics; one chip per field; empty groups hide.
  - **View toggle** — `PillGroup` sm: Rows | Cards (elevated active, never accent fill).
    No “Sorted by …” label unless a real sort control ships.
  - **Rows** — 3-col grid: neutral icon well · main (name + kind tag + slug, one-line goal,
    meta `N nodes` / `N inputs` / iteration cap, optional binding badge) · trail (category,
    last-outcome pill, 30d success-rate, `Run`).
  - **Cards** — `CatalogCard` shape: logo well · title + tags · meta eyebrow · 2-line
    description · actions row (status pill + rate + `Run`). Grid 1/2/3 cols.
  - Binding badge: clock glyph for schedule / webhook glyph for webhook on loops with an
    attached loop-target automation (e.g. `software-delivery` · `0 3 * * *`). The
    `reviews-watch` watch-source tag is a body-node concept, not a start binding — no badge.
- **Data shown (the 2 built-in Loops).** `software-delivery` (Engineering · delivery,
  8 nodes, 5 inputs, cap 50) and `reviews-watch` (Engineering · watch, watch-source,
  6 inputs, cap ∞). The Custom group is empty until a fork exists (JS hides empty groups);
  never invent extra loops.
- **Interactions.** Search + filter chips (AND); Rows|Cards toggle; row/card click →
  detail; `Run` → run form (stops propagation); `/` focuses search; Escape closes filter menus.
- **Spec terms surfaced.** kind (built-in/custom/fork), category, mode, node count, input
  count, iteration_cap (incl. unbounded `∞`), last terminal/live state, success rate.
- **Data contract.** `GET /loops` returning, per Loop: name, kind, source-of-fork,
  category, goal, node count, declared-input count, iteration_cap, last-run state, a
  30d success-rate aggregate, and an attached-binding summary (kind + short label) for the
  binding badge (section 9.14). `[VERIFY]` success-rate and run-count aggregates exposed by
  daemon vs computed view (section 9.11).

### 4.2 `loop-detail.html` - Loop definition page
- **Purpose.** Everything about one Loop: contract, body graph (read-only), recent runs,
  declared inputs, limits vs ceilings, versions, 30d stats.
- **Route/IA.** `Loops › software-delivery`. Topbar: `View DSL`, overflow.
- **Layout.** DetailHeader (icon, name, `Built-in` + `v4 · published` tags, meta line;
  actions `Configure`, `Fork & edit`, `Run loop` = the one accent CTA) · two-column:
  main (Contract, Body·DAG, Recent runs) + 320px right rail (Declared inputs, Start
  bindings, Limits & budget, Versions, Last 30 days).
- **Key components.** `.kv` contract rows; verification rows (`command` / `agent-judge` /
  `human` with method line); terminal-outcome chips; read-only DAG (`.graph` of neutral
  `.gnode`, gate nodes tint verdict text, fan-out cluster of `.gbranch`); recent-runs rows
  with state pill; right-rail input rows (name, required `*`, type badge, desc, default),
  limits rows (`default / ceiling`), versions list (current badge), 4-up stat grid;
  **Start bindings panel** (between Declared inputs and Limits & budget): row 1 renders the
  DSL `start[]` kinds as neutral mono chips (`manual · cli · http · uds · native_tool ·
  schedule`, read-only, labeled `declared`); then one row per attached loop-target automation with mono name,
  neutral kind tag (`schedule` / `webhook` / `trigger`), an enabled dot (success when enabled,
  neutral when disabled) and a mono meta line (schedule: cron + next fire, e.g.
  `0 3 * * * · next in 6h`; webhook: endpoint slug; trigger: event name, e.g.
  `hook.review.completed`); footer CTAs `Add trigger` and `Add schedule` (secondary buttons
  that open the existing automation sheets pre-targeted at this Loop; inert in the mock).
  Empty state copy: "Runs manually. This loop also declares a schedule start: attach an
  automation to run it hands-free."
- **Data shown.** software-delivery: goal and definition_of_done (verbatim from the
  definition), 3 gates (review acceptance agent-judge · verify project command
  `{{ .inputs.verify_command }}` · approve human), 6 terminal chips (done / no_op /
  blocked / failed / exhausted / stalled), 8-node body, 5 declared inputs
  (slug*, implementer, verify_command, auto_commit, target_branch), 6 declared start
  kinds (manual · cli · http · uds · native_tool · schedule) + 1 attached automation
  (a nightly schedule `0 3 * * *`, disabled), 7 limit rows, v1-v4, 30d stats
  (92% / 48 runs / 1.9 avg gens / 8m median).
- **Interactions.** Links to editor, run form, runs. (Mostly static.)
- **Spec terms surfaced.** contract{goal, definition_of_done, verification[],
  terminal_states}; graph node classes/kinds; declared input types + defaults; the full
  limits/ceilings table; version model; run aggregates; `start[]` allowlist + attached
  loop-target automations (section 9.14).
- **Data contract.** `GET /loops/:name` (full definition + computed stats), plus the attached
  loop-target automations (`GET /loops/:name/bindings` or `GET /automations?target=loop:<name>`):
  name, kind, enabled, cron + next fire / endpoint slug / event name. The Add CTAs deep-link the
  existing trigger/job create sheets pre-targeted at the Loop. The right-rail
  limits show both per-loop default and daemon ceiling - the daemon must expose both.

### 4.3 `loop-run-form.html` - Run a Loop (hero path)
- **Purpose.** The arrive-and-use moment: an auto-generated, validated input form + a live
  contract preview, with optional per-run limit overrides.
- **Route/IA.** `Loops › software-delivery › Run`. Sticky bottom action bar.
- **Layout.** Page head (`Run software-delivery`) · two columns: form (left) + sticky
  contract preview (right) · sticky action bar (Cancel · Dry run · Run loop).
- **Key components.**
  - **Inputs (auto-generated from declared `inputs`).** `slug` string (required, mono),
    `implementer` agent (avatar-prefixed picker, default `code_implementer`),
    `verify_command` string (default `""`, empty = check disabled), `auto_commit` bool
    (switch, default `false`), `target_branch` string (default `main`). Each field shows
    a type badge; required marker `*`; inline required error.
  - **Advanced · per-run limit overrides** (`<details>` collapsible). 6 number fields, each
    showing the per-loop default value and the daemon ceiling, clamped at the ceiling:
    iteration cap 50 /100, token budget off /20M, wall clock off /7d, no-progress window
    3 /10, fan-out ceiling ≤ tasks /64, gate max revisions 3 /10 (cost is display-only, no
    input, §9.5.2). Badge flips "using Loop defaults" → "overrides set".
  - **Preview.** Live "what will run" summary (updates from inputs); read-only contract
    (goal, DoD, 4 verification rows, 6 terminal chips); lifecycle line
    (running → verify → done; node-level queueing renders on nodes, never as a run
    status).
  - **Start-bindings hint.** Under the form, a muted line: "This loop also starts from:
    schedule. Manage in Start bindings on the loop page."
- **Interactions.** Live summary recompute on every field; `auto_commit` toggles the
  per-task commit line in the preview; override clamp at ceiling; required
  validation (Run disabled until `slug`); `Dry run` validates inputs + renders the plan
  without starting a run (toast).
- **Spec terms surfaced.** declared input types (string/agent/bool shown; number/file/ref
  defined in the type system); per-run overrides; daemon ceilings; dry-run; lifecycle
  states; terminal outcomes.
- **Data contract.** `GET /loops/:name` (inputs schema + contract + defaults + ceilings);
  `POST /loops/:name/runs` with `{inputs, overrides}`; a dry-run mode (section 9.1).
  The form is generated entirely from the declared-input schema, so the schema must carry
  per-input: name, type, required, default, description. `[VERIFY]` ref/file/agent picker
  data sources (section 9.10).

### 4.4 `run-detail.html` - Live run monitor (heaviest)
- **Purpose.** Truthful, real-time view of a running Loop: contract, live meters,
  generation-by-generation timeline, fan-out, gate verdicts, multi-agent channel, and the
  human approval gate.
- **Route/IA.** `Loops › software-delivery › r-8f3a2b`. Topbar: `Graph view`, `DSL`.
- **Layout.** Two columns: main (sticky contract header + meters, then timeline) + 332px
  right rail (Live events, Run facts, Terminal-outcome legend).
- **Key components.**
  - **Sticky contract header.** Live `Running` pill (pulse), `Generation N · attempt N of
    cap`, started/trigger/actor meta, the goal, `Pause` + `Stop run` actions.
  - **5 meters.** Attempts (2/50), Tokens (412K/2M), Wall clock (14m/30m), Cost ($1.84),
    Breadth (4/64). Bars warn-tint near ceiling only.
  - **Generation timeline.** Collapsible `gen` cards on a flat **node spine**. G1
    revised (slug → `load_tasks` file-import → `implement` fan-out over the authored tasks
    in dependency order rendered as batch branches, `batch_size: 1` so each branch is one
    task e.g. `task_01 · schema migration`, 1 failed branch → collect → review agent-judge
    → request_changes → revise). G2 running (carry-forward of 3 task outputs,
    `execute_task` re-running only the failed task via run-agent, pending review + verify,
    human approval gate). Node IDs are snake_case (ADR-020).
  - **Gate card.** Flat tint (`pass` success / `fail` danger), verdict + reason + route
    (`revise` / `next_generation`). No side-stripe, no gradient.
  - **Channel.** Embedded `#delivery-r8f3a` with implementer/reviewer/decision messages.
  - **Approval gate.** `needs-approval` tag, "Approve merge to `main`?", facts (branch,
    diff, tests, verifier), actions: `Approve & resume`, `Request changes`, `Reject & halt`.
  - **Right rail.** Streaming live events (`node_running`, `channel_msg`,
    `generation_started`, `gate_verdict`, `node_failed`, `node_succeeded`); run facts
    (loop, revision pinned, re-attempt, trigger, workspace, run id); terminal legend.
    Automation-started runs name their trigger in Run facts and the sticky-header meta
    (`Trigger: schedule · nightly · 0 3 * * * (job)`); non-automation starts keep bare
    kinds (`web` / `cli` / `agent`). The mock run `r-8f3a2b` is schedule-started.
- **Interactions.** Collapse generations + node logs; gentle live simulation nudges meters
  + prepends events (guarded by reduced-motion).
- **Spec terms surfaced.** live + terminal states (11-state enum); generations; attempts
  vs iteration_cap; token/wall/cost/breadth meters; no-progress window count; node
  classes/kinds incl. channel posting via `agh__network_send` with
  `harvest: {kind: channel_result, window, responder?, content_rule?}` (the UC2
  converse-and-decide convention, ADR-021; `channel-post` is not a kind); gate verdict +
  routing (`revise`/`next_generation`); carry-forward (failed-only); `needs-approval`
  live pause; pinned revision; trigger source.
- **Data contract.** `GET /runs/:id` + an SSE/event stream. Events the UI binds:
  `node_running|node_succeeded|node_failed`, `gate_verdict`, `generation_started`,
  `channel_msg`, `token_tick`/budget updates, `needs_approval`. Run object must expose:
  state, generation index, attempt/cap, token/wall/cost/breadth usage + caps, pinned
  revision, re-attempt mode, trigger (kind, plus automation name + automation kind when
  automation-started), per-node status + output, gate verdicts + route,
  channel transcript + harvested decision, approval-gate payload. Controls: pause, resume,
  stop, and approval decision (approve / request-changes / reject). (section 9.3, 9.8)

### 4.5 `runs.html` - Runs history (global)
- **Purpose.** Every execution across every Loop, the full outcome spectrum, filterable.
- **Route/IA.** `Loops › Runs`. `[VERIFY]` product-ux currently says no separate global
  Runs route (section 9.4).
- **Layout.** Page head (live "3 active") · KPI strip (4) · outcome filter `segment` +
  Loop/date selects · Active table · Past table.
- **Key components.** KPIs (Active now, Awaiting you, Done today, Needs a look); outcome
  segment over the run-status spectrum (All / Running / Watching / Needs-approval /
  Done / No_op / Blocked / Failed / Exhausted / Stalled with counts; segments render
  data-driven for the statuses present in the window, so Paused / Queued appear once
  such runs exist); table columns
  (Outcome pill, Loop + run-id/trigger, Goal, Gens, Started/Ended, Budget mini-bar with
  warn/danger near limit, chevron).
- **Data shown.** 27 runs (3 active, 24 past) across all states, multiple Loops/triggers
  (web, cli, agent, schedule, webhook).
- **Interactions.** Outcome filter (JS, hides empty sections); row → run detail.
- **Spec terms surfaced.** full state spectrum; gens vs cap; trigger sources; budget
  usage + cap with overflow→exhausted; "awaiting you" = needs-approval queue.
- **Data contract.** `GET /runs?status=&loop=&since=` + the same aggregates the KPIs need.
  If global Runs stays, the daemon needs a workspace-wide run index (section 9.4).

### 4.6 `loop-editor.html` - Visual DAG builder (fork & edit)
- **Purpose.** Fork an existing Loop or built-in and edit its body on a canvas: nodes,
  edges, per-node inspector, inline linter, versions, Graph/DSL views. ADR-008: no
  blank-canvas builder.
- **Route/IA.** `Loops › software-delivery › Fork & edit`. Topbar: `Unsaved changes`
  chip, version selector (`v5 · draft`), `Validate`, `Save draft`, `Publish` (disabled
  while issues exist). Sub-toolbar: auto-layout / zoom / fit; Graph|DSL segmented; fork
  context; the 4 linter invariant chips.
- **Layout.** Three regions: 190px node palette · canvas (scrollable, dot-grid) with a
  bottom linter dock · 344px node inspector.
- **Key components.**
  - **Palette ("Add node").** Grouped by class: Action (the three reserved kinds
    run-agent / run-loop / transform, a curated tool shortlist incl. a "Channel post"
    shortcut that inserts a pre-filled `agh__network_send` node, and a searchable
    "Call tool..." picker over the full 181-tool registry, ADR-021), Control (fan-out,
    collect, branch, gate, sub-loop), Source (watch-source, file-import, input). Drag
    affordance. Fork-and-edit note.
  - **Canvas.** Positioned nodes + SVG edges (ReactFlow-style, not a graph lib). Neutral
    node cards (class·kind label, name, kind). Selected node = accent ring; error node =
    danger ring + badge. Fan-out renders 4 branch chips.
  - **Linter dock.** 4 invariants (acyclicity, reachability, termination, fan-out bounds)
    as pass/fail chips + an issue list. Demo issue: `implement.max_fan_out (80) exceeds
    the daemon ceiling of 64`, code `fan_out_ceiling_exceeded`, "publish returns 422 until
    resolved", with `Reveal node`.
  - **Inspector (per class/kind).** Rendered FROM the canonical DSL types (task 02) and
    the registry tool schemas (ADR-023), never from an editor-local field model:
    - source/input: id, input_ref (must name a declared input), derived produces
      (read-only).
    - source/file-import: id, pattern (template), parse (md_tasks|json|text), produces;
      large payloads become content-addressed artifact refs, never inlined.
    - action/run-agent: id, params.agent (REQUIRED, interpolable), params.prompt
      (REQUIRED template), output_schema (optional, structured harvest + one free
      schema-validation retry), cwd, model, allowed_tools, max_turns, plus the envelope
      (session, timeout, retry, harvest, produces).
    - action/run-loop: id, params.loop (interpolable name), params.inputs (interpolated
      map), mode (await|detach); ancestry cycle-guard hint (depth <= 8).
    - action/transform: id, params.map rows ({from: <namespace path>} | {value: literal}
      | {template: "{{ ... }}"}).
    - action/any ToolID: id, kind (the literal ToolID), params form generated from the
      tool's registry input schema (template-interpolated); optional harvest (e.g.
      `channel_result` on `agh__network_send`).
    - control/gate: id, criteria rows ({id, type: command|agent-judge|human|extension,
      per-type fields incl. rubric templates}), verdict_policy (revise_until_clean |
      fixed_passes; the linter requires a judge/human criterion for revise_until_clean),
      on_result routing (in-body: continue|revise|branch|halt|escalate;
      definition-of-done: continue→done|next_generation|halt|escalate), max revisions,
      hint.
    - control/branch: id, condition (validated raw CEL with autocomplete from the
      compiled reference schema, ADR-020; visual builder deferred).
    - control/fan-out: id, collection (template, e.g.
      `{{ .nodes.load_tasks.output.tasks }}`), batch_size (items per branch, default 1),
      max_parallel (concurrent branches, 1 = sequential), max_fan_out (number, ceiling
      64), paired collect, hint (ADR-006).
    - control/collect: id, joins, hint.
  - **Graph|DSL toggle.** DSL view shows the `agh.loop/v1` YAML on disk (the bijective
    codec / FS-as-truth from ADR-015), with the offending `max_fan_out: 80` highlighted.
  - **Start summary (graph view).** A read-only chip strip pinned at the canvas origin
    (`start: manual · cli · http · uds · native_tool · schedule`) with the muted note
    "edited in the DSL view". The DSL view renders the `start:` block itself; no graph-side
    start editing in v1.
- **Interactions (real).** Click node → inspector swaps; lowering `max_fan_out` ≤ 64 in
  the inspector clears the issue, flips the invariant to pass, hides the node badge, and
  enables Publish (a faithful render of the linter→422→publish gate); dock collapse;
  Graph/DSL switch; zoom.
- **Spec terms surfaced.** ADR-003/021 node classes (action open: 3 reserved kinds +
  any ToolID; control/source closed); per-kind config field names (canonical in the DSL
  types, ADR-023); edges (`blocks` deps, acyclic); start[]; the linter invariants (incl.
  reference validation and snake_case IDs) and the 422-per-node publish contract;
  validate-without-save + `expected_version` CAS (ADR-023); fork-and-edit (ADR-008);
  draft vs published + versions; bijective definition↔graph codec (ADR-015).
- **Data contract.** `GET /loops/:name` (definition → graph); `PATCH /loops/:name`
  (graph → definition; requires `expected_version`, mismatch → 409 with the current
  version; returns 422 with per-node `{node_id, code, message, severity}`; publish
  compiles and persists the resolved form the runtime exclusively consumes, ADR-023);
  `POST /loops/:name/validate` (lint+compile WITHOUT saving, same 422 shape,
  deterministic codes: `unknown_reference`, `node_id_invalid`,
  `verdict_policy_requires_judge`, `fan_out_ceiling_exceeded`, ...); versions list
  (diff/revert deferred, section 9.13). **The editor needs a store for node layout
  positions** keyed `(workspace_id, loop_name, node_id)` per ADR-015 (section 9.7).
  Registry lookups for tools / agents to populate the palette picker and the
  schema-generated params forms (section 9.10).

### 4.7 `loop-configure.html` - Configure (light, no-fork)
- **Purpose.** Adjust how a Loop runs without changing its structure. Power ceiling, never
  the hero.
- **Route/IA.** Right-side **sheet/drawer** over a dimmed loop-detail backdrop; opened
  from `Configure` on detail/catalog. Close → reopen pill (mockup affordance).
- **Layout.** Sheet header (icon, `Configure`, `software-delivery · no-fork tweaks`,
  close) · scroll body (4 groups) · footer (Reset to defaults · Cancel · Save
  configuration; saved toast).
- **Key components / groups.**
  1. **Verification checks** ("declared in the Loop"): per-check enable switch + project
     command field where the check is a generic `command` gate (Test suite `go test ./...`,
     Linter `golangci-lint run`, Acceptance review agent-judge - cannot be removed without
     a fork). Disabling a command check disables its command field.
  2. **Human approval gate**: a single switch (Merge approval) - the human-gate toggle.
  3. **Re-attempt strategy**: two selectable cards `failed-only` (default) | `full-body`,
     with descriptions (ADR-009/011).
  4. **Stop limits** (per-loop defaults): 7 number fields with ceilings, clamped at
     ceiling (same set/values as the run-form Advanced panel).
- **Interactions.** Switches; command-field enable/disable; strategy cards; limit clamps;
  Reset restores defaults + failed-only; Save toast; close/reopen.
- **What CANNOT change here (needs a fork):** node order / DAG structure, node kinds,
  input declarations, terminal states / contract shape, goal / definition-of-done.
- **Spec terms surfaced.** no-fork config layer (ADR-009); verification selection vs
  structural edit boundary; human-gate toggle; re-attempt granularity; per-loop limit
  overrides bounded by ceiling; save/reset semantics.
- **Data contract.** `GET /loops/:name/config` + `PUT /loops/:name/config` writing per-loop
  defaults (limits, enabled checks, human-gate flag, re-attempt mode, project command
  overrides). Distinct from a fork (which writes a new definition). (section 9.6)

### 4.8 `loops-index.html` - Overview / launcher
- **Purpose.** A design-doc launcher that explains the two nouns and links to all screens
  with Built/Planned tags. **Not a product route** - it is scaffolding for review. All
  cards are now `Built`.

---

## 5. Cross-cutting domain model (UI-facing contract)

### 5.1 Run state machine (as rendered)
The run status enum has exactly 11 states (ADR-013/017/021/022). Live: `running` ⇄
`paused`, plus `watching` (watch-driven), `needs-approval` (live pause awaiting a human)
and `queued` (a deferred start under `concurrency: queue`, promoted FIFO when the active
run reaches terminal).
Terminal: `done` | `no_op` | `blocked` | `failed` | `exhausted` | `stalled`.
`no_op` = the generation/watch tick completed with no work to do (empty collection, no
watch event materialized): neutral, never reported as `done`-with-fake-work. `blocked` =
an external dependency makes progress impossible (missing credential, unreachable
resource, refused `run-loop` cycle): warning family, distinct from `stalled` (work
possible, no progress made). `no_op`/`blocked` are never coerced to `done`/`failed`, and
error/exhaustion never reports as `done`. `ready` and `awaiting_child` are node-level
states only (`loop_generation_outputs.status`; `awaiting_child` marks a `run-loop await`
node yielding until its child loop terminates, ADR-021), never run statuses. The UI
renders every state with
the shared `.pill--*` vocabulary and never invents a state outside this set.
`[shipped-in-spec]`

### 5.2 Node taxonomy and per-kind schemas (ADR-003/021/023)
Node IDs are snake_case (`^[a-z][a-z0-9_]*$`, linter-enforced); kinds stay kebab-case.
Every node carries the ADR-014 envelope (`session? / timeout? / retry? / harvest? /
produces?`) plus the per-kind fields below. The DSL types (task 02) and the registry
tool schemas are the canonical schema source; the editor inspector renders FROM them
(ADR-023).

- **action** (open): exactly three reserved kinds plus any ToolID (ADR-021); `call-tool`
  and `channel-post` do not exist as kinds.
  - `run-agent` (reserved): `params: { agent (REQUIRED, interpolable profile ref),
    prompt (REQUIRED template), output_schema? (JSON Schema → structured harvest + one
    free schema-validation retry), cwd?, model?, allowed_tools?, max_turns? }`. The
    agent profile stays the identity/config authority; the overrides are per-node
    deltas.
  - `run-loop` (reserved): `params: { loop (interpolable name), inputs (interpolated
    map), mode: await (default) | detach }`; await output = child `{status, outputs}`,
    detach output = `{loop_run_id}`; runtime ancestry-chain cycle guard, depth <= 8.
  - `transform` (reserved): `params.map: { <key>: {from: <namespace path>} | {value:
    literal} | {template: "{{ ... }}"} }`; pure in-daemon reshaping, the confinement for
    complex mapping.
  - **any ToolID** (`agh__*` / `ext__*` / `mcp__*`, 181 native tools): `params` = the
    tool's input schema (template-interpolated); the editor form is generated from the
    registry schema. Channel posting is `kind: agh__network_send` + optional
    `harvest: {kind: channel_result, window, responder?, content_rule?}` (the UC2
    result convention; the `channel-post` primitive is retired, ADR-021).
- **control** (closed enum): `fan-out` (`collection: "{{ .nodes.<id>.output.<path> }}"`
  template over a finite materialized collection, plus the three orthogonal knobs
  `batch_size` (items per branch, default 1; the branch `item` is the single element at
  `batch_size: 1`, the slice array otherwise), `max_parallel` (concurrent branches; 1 =
  sequential) and `max_fan_out` (structural cap on materialized branches, ceiling 64),
  branch body with `item`/`index` in scope, ADR-006); `collect` (join barrier, no extra fields);
  `branch` (`condition: <CEL>` + true/false edges, ADR-020); `gate` (`criteria[]` +
  `verdict_policy` + `on_result` + `max_revisions`, see 5.3); `sub-loop` (inline nested
  body with its own contract; `run-loop` is the cross-definition composition).
- **source** (closed enum): `input` (`input_ref: <declared input name>`; `produces`
  derived from the declared input type, ADR-023); `file-import` (`pattern: <template>`,
  `parse: md_tasks | json | text`, `produces`; large payloads land in the
  content-addressed store and the output carries artifact refs, never inlined,
  ADR-023); `watch-source` (`WatchSpec`, ADR-016, unchanged).
Node *class* is a neutral mono label; never color-coded.

### 5.3 Verification criteria and gates (ADR-005/022)
A gate (and the contract's `verification[]`) declares a typed **criteria array**: 1..N
checks, all evaluated at one decision point,
`criteria: [{id, type: command|agent-judge|human|extension, ...per-type fields}]`, plus
`verdict_policy: revise_until_clean | fixed_passes`, `on_result`, and `max_revisions`.
Linter invariant: `revise_until_clean` requires at least one `agent-judge` or `human`
criterion as the verdict source (`verdict_policy_requires_judge`); a command-only gate
may declare only `fixed_passes`. Every `agent-judge` criterion emits a structured
verdict `{verdict: pass|revise, blocking_issues: [{id, note}], confidence, evidence}`;
malformed or unparseable output degrades to `revise` plus a warning, never to pass.
A gate's `on_result` routing differs by placement: **in-body**
`continue|revise|branch|halt|escalate`; **definition-of-done**
`continue (→done)|next_generation|halt|escalate`.

### 5.4 Limits and ceilings (the numbers the UI renders)
> These are the values shown consistently across `loop-detail`, `loop-run-form`,
> `loop-configure`, and `run-detail`. Left = per-loop default, right = hard daemon ceiling.

| Limit | Default (delivery) | Ceiling | Notes |
|-------|--------------------|---------|-------|
| iteration_cap | 50 | 100 | watch loops default 0 (unbounded, shown `∞`) |
| budget.tokens | off (0) | 20M | 0 = unlimited; opt-in |
| budget.wall_clock | off (0) | 7d | opt-in |
| budget.usd (cost) | display-only | — | derived tokens × price; no enforced cap (§9.5.2) |
| no_progress.window | 3 | 10 | window count, advances on node completion |
| fan_out_ceiling | ≤ tasks | 64 | bounded by the loaded task count; hard cap 64; overflow → exhausted |
| gate.max_revisions | 3 | 10 | per gate; after limit, gate fails terminal |

Ceilings are hard backstops, never editable in the UI. Budgets stay opt-in (0 =
unlimited; progress-first, ADR-012), but a SET budget is ENFORCED (ADR-022): a durable
per-run accumulator (tokens summed across all node runs; wall clock from `started_at`)
is checked before each node dispatch and at each generation boundary. Exceeding it
applies `budget.on_exceeded`: `halt` (default) → terminal `exhausted`; `escalate` →
`needs-approval` for a human decision.

> **Resolved (TechSpec §9.5.1 / ADR-017):** the canonical defaults are `iteration_cap=50`
> and budgets `0`/unlimited (off). The mockups now render these canonical values across all
> screens; the earlier 10 / 2.0M design figures are retired.

### 5.5 Generations and re-attempt
A run iterates across generations. `failed-only` (default) re-runs failed/pending nodes +
transitive dependents and carries succeeded outputs forward read-only; `full-body` re-runs
everything. The strategy is an author/configure-time choice (ADR-009/011), shown as a
read-only **run fact** on `run-detail`, never chosen on the run form.

### 5.6 Start bindings (ADR-007)
The DSL `start[]` allowlist
(`manual|cli|http|uds|trigger|schedule|webhook|network|extension|native_tool`) declares how a
Loop may be initiated; the definition is the only place it is edited. Event-driven kinds
(trigger/schedule/webhook) are carried by AGH's existing automation primitives via a
discriminated target on Trigger/Job: `Run agent` (today) or `Run loop`
`{workspace, loop, static inputs, event-payload → input mapping}` (spec-only, TechSpec §9.14;
no design HTML for those sheets). Surfacing rules as designed:
- **loop-detail**: Start bindings rail panel = declared kinds (neutral mono chips, read-only,
  labeled `declared`) + one row per attached automation (mono name, neutral kind tag, enabled
  dot, kind-specific mono meta line) + `Add trigger` / `Add schedule` CTAs that open the
  existing automation sheets pre-targeted at the Loop.
- **loops-catalog**: neutral binding badge on rows with at least one attached automation
  (clock glyph for schedule, webhook glyph for webhook).
- **loop-editor**: read-only start chip strip pinned at the graph-canvas origin; the `start:`
  block is edited only in the DSL view (no graph-side editing in v1).
- **run-detail**: automation-started runs name their trigger
  (`schedule · nightly · 0 3 * * * (job)`); non-automation starts keep bare kinds
  (`web` / `cli` / `agent`).
- **loop-run-form**: muted hint pointing to Start bindings on the loop page.
Kind chips and tags are always neutral mono (color = state only); the enabled dot is the only
stateful color in the panel. A watch-source is a node INSIDE the body, never a start binding.

### 5.7 Contract additions (round 5, ADR-021/022)
- `verification[]` uses the same criteria model as gates (5.3).
- `terminal_states: [done, no_op, blocked, failed, exhausted, stalled]` (6 terminal
  outcomes).
- `stop_when` (CEL, optional, ADR-020): a boolean early-terminal condition evaluated at
  the generation boundary; true → terminal `done` (e.g. `reviews-watch`:
  `nodes.fetch_issues.status == 'succeeded' && size(nodes.fetch_issues.output.issues) == 0`).
- `no_progress: { window, hash_fields }` plus a structured blocker-ID signature: the
  `stalled` detector compares the sorted `blocking_issues[].id` set from the
  generation's gate verdicts across the window (same IDs = no progress), never
  free-text signatures. The typed `hash_fields` progress hash covers non-gate progress;
  the always-on inactivity clock is unchanged.
- `budget: { tokens, wall_clock_sec, on_exceeded: halt | escalate }`: opt-in
  (0 = unlimited) but ENFORCED when set (5.4). USD stays out of v1 (cost display-only).
- `concurrency: forbid | allow | queue` (default `forbid`) at the loop level (ADR-021):
  `forbid` rejects a second concurrent start of the same loop (409 + the active
  `loop_run_id`), `queue` enqueues the start until the active run reaches terminal,
  `allow` permits parallel runs.

### 5.8 Reference grammar and node IDs (ADR-020)
One namespace, two surfaces, both compile-validated:
- **Values** (every string-valued field: `params.*`, `pattern`, gate rubrics,
  `input_mapping` values, `run-loop.inputs`) interpolate with Go-template `{{ }}`:
  `{{ .inputs.slug }}`, `{{ .nodes.load_tasks.output.tasks }}`. Curated funcmap only
  (`json`, `join`, `default`).
- **Boolean conditions** (`branch.condition`, collection `filter`,
  `contract.stop_when`) are CEL and must return bool:
  `nodes.review.output.verdict == 'pass'`.
- **Namespace** (shared by both surfaces): `inputs.<name>` ·
  `nodes.<id>.output.<path>` · `nodes.<id>.status` · `item` / `index` (fan-out scope
  only) · `trigger.<path>` (activation payload) · `generation`.
- There is no `${...}`-style interpolation and no bare dotted-ref syntax. References resolve
  against declared output schemas at lint/publish or fail with `unknown_reference`;
  never silent empty strings.
- **Node IDs are snake_case** (`^[a-z][a-z0-9_]*$`), so the same ID is valid verbatim
  in both surfaces; kinds stay kebab-case. The editor's reference picker autocompletes
  from the same compiled reference schema the linter uses.

---

## 6. Component build inventory (for implementation)

Reusable, build-once components implied by the eight screens:

- **Shell:** `Topbar` (inventory pages: icon + title + count; detail/editor: breadcrumb + trailing actions), content `Scroll`, sticky
  `ActionBar`, right `Rail`, `Sheet`/drawer (over scrim, `--shadow-overlay`).
- **Primitives:** `Button` (primary/ghost/danger/icon), `StatePill` (the `.pill--*` set
  + pulse), `CountChip`, `Tag`, `SectionLabel`, `Panelbox`, `KVRow`, `MetaLine`.
- **Forms:** `TextInput` (+ mono), `NumberInput` (+ ceiling clamp + error), `Select`,
  `Textarea`, `Switch`, `SegmentedControl`/`PillGroup`, `AgentPicker`, `RefPicker`,
  `FilePicker`, `FieldRow` (label + required + type badge + hint + error),
  `Collapsible` (`<details>`), `LimitOverridesGrid`.
- **Data display:** `FilterSegment` + `CategoryPills`, `LoopListRow`, `RunTableRow` +
  `BudgetMiniBar`, `KPICard`, `Meter` (with warn state), `StatGrid`, `StatusLegend`.
- **Loops-specific:** `ReadOnlyDAG` (graph view), `DAGCanvas` + `Node` + `Edge` +
  `NodeInspector` (per-class field sets) + `LinterDock` + `Palette` (editor),
  `GenerationTimeline` + `NodeSpine` + `NodeRow`, `GateCard` (pass/fail/route),
  `EmbeddedChannel` + `ChannelMessage`, `ApprovalGate`, `ContractPreview`, `DSLView`
  (YAML render of `agh.loop/v1`), `StartBindingsPanel` (declared-kind chips + automation
  rows + add CTAs), `BindingBadge` (catalog rows), `StartChipStrip` (editor graph view).

These map cleanly to `web/src/systems/loops/*` + `packages/ui` primitives. The
node-inspector field sets are the single most reused data structure, but they are NOT a
schema source: per ADR-023 the DSL types (task 02) plus the registry tool schemas own
the per-kind schema, and the inspector (and the read-only viewer reused on detail/run
pages) RENDERS FROM them. `loop-editor.html`'s local `NODES` model is a mock-only
stand-in, never canonical.

---

## 7. Per-screen data contract summary (daemon surfaces the UI assumes)

| Screen | Reads | Writes / actions | Realtime |
|--------|-------|------------------|----------|
| catalog | `GET /loops` (+ 30d aggregates + binding summary) | run (→ run form) | - |
| detail | `GET /loops/:name` (definition + stats) + attached loop-target automations | - (Add CTAs deep-link the automation sheets) | - |
| run form | `GET /loops/:name` (input schema, contract, defaults, ceilings) | `POST /loops/:name/runs {inputs, overrides}`; dry-run | - |
| run detail | `GET /runs/:id` | pause / resume / stop; approval decision | SSE event stream |
| runs | `GET /runs?status=&loop=&since=` (+ KPI aggregates) | - | active rows live |
| editor | `GET /loops/:name` (→ graph); registry/agent lists; node positions | `PATCH /loops/:name` (requires `expected_version`; mismatch → 409 + current version; → 422 per-node; publish persists the compiled resolved form); `POST /loops/:name/validate` (lint+compile, no save); versions list; save positions | - |
| configure | `GET /loops/:name/config` | `PUT /loops/:name/config` | - |

Round-5 write surfaces (ADR-023):
- `POST /api/workspaces/:wid/loops/:name/validate` runs lint+compile WITHOUT saving and
  returns 422-shaped per-node errors with deterministic codes (`unknown_reference`,
  `node_id_invalid`, `verdict_policy_requires_judge`, `fan_out_ceiling_exceeded`, ...).
- `PATCH /loops/:name` requires `expected_version` (compare-and-swap); a mismatch
  returns 409 with the current version, so concurrent editors never last-write-wins
  each other.
- Publish compiles the definition (templates parsed, CEL compiled, tool schemas
  snapshotted by digest, defaults folded) and persists the resolved form; the runtime
  exclusively consumes the resolved projection, never the author YAML.

Every UI action must have a CLI/HTTP/UDS equivalent over the same daemon state
(agent-manageability, PRODUCT.md principle 3): `agh loop list|inspect|create|edit|validate`,
`agh run start|list|inspect|pause|resume|stop|approve`. Validation is also a native-tool
surface: `agh__loop_validate` (equivalently a `validate` mode on `agh__loop_create`,
alongside the `agh__loop_*` management tools:
`list|describe|run|status|runs|stop|pause|resume|configure|approve|create|delete`)
mirrors `POST /loops/:name/validate` (ADR-021/023).

---

## 8. Interaction fidelity built into the mockups
Not screenshots - the following behave: catalog Filters chips + Rows|Cards view toggle + search; run-form live
preview + required validation + override clamps + dry-run; run-detail collapsible
generations/logs + live meter/event simulation; editor node selection → inspector swap +
the linter (fan-out width > ceiling → issue → publish blocked) + Graph/DSL toggle + zoom;
configure switches + command enable/disable + strategy cards + limit clamps + save/reset +
close/reopen; runs outcome filter.

---

## 9. Spec alignment - what to confirm or change in PRD / TechSpec

> The actionable output. Each item: what the design surfaces, the question, and a proposed
> resolution. Priority: **P0** blocks a truthful implementation; **P1** should be decided
> before build; **P2** polish/IA.

### 9.1 Dry run `[P1]`
- **Design.** Run form has a `Dry run` secondary action ("validates inputs and renders the
  first-generation plan without starting a run").
- **Question.** PRD/TechSpec do not formally define a dry-run verb or endpoint.
- **Proposal.** Either (a) add dry-run to the spec with explicit semantics (validate inputs
  against the input schema + contract, render the gen-1 plan/preview, spend no budget,
  create no Run, return a plan artifact), exposed as `POST /loops/:name/runs?dry=true` and
  `agh run start --dry-run`; or (b) drop the control from the UI. Recommend (a): it is a
  cheap, honest operator affordance.

### 9.2 Branch condition AST `[P0 for the editor]` - RESOLVED (ADR-020)
- **Design.** The editor's `branch` node needs a condition editor; the inspector treats
  it as an honest expression field (no invented AST UI).
- **Resolution (ADR-020).** `branch.condition` is CEL over the pinned single namespace
  (`inputs.*`, `nodes.<id>.output.*`, `nodes.<id>.status`, `item`/`index`, `trigger.*`,
  `generation`), compile-validated at lint/publish (`condition_not_bool`,
  `unknown_reference`). The editor ships a validated raw-CEL field in v1 with
  autocomplete from the compiled reference schema; a visual condition builder is
  deferred. The earlier "read-only / raw-expression until pinned" posture is superseded.

### 9.3 Run controls: pause / resume / stop `[P1]`
- **Design.** `run-detail` exposes `Pause` and `Stop run`; the state machine includes
  `paused`.
- **Question.** Confirm the daemon supports pause/resume/stop of a live run and the
  `paused` state transitions.
- **Proposal.** Confirm in TechSpec + expose `agh run pause|resume|stop` and the HTTP
  equivalents; define what "pause" means mid-generation (boundary vs immediate).

### 9.4 Global Runs route `[P2 / IA]`
- **Design.** `runs.html` is a global, workspace-wide run history with KPIs.
- **Question.** `product-ux.md` says run history lives under each Loop with no separate
  global Runs nav route.
- **Proposal.** Decide: (a) adopt global Runs as an operator convenience (update
  product-ux + add a workspace run index + nav entry), or (b) fold it into
  catalog/loop-detail and keep `runs.html` only as the "All runs" target. Recommend (a):
  the "Awaiting you" / active-across-loops view is operationally valuable.

### 9.5 Limit defaults + cost budget `[P0]`
- **Design.** Renders iteration_cap default **10** (/100) and budget.tokens default
  **2.0M** (/20M) consistently across four screens, plus a `Cost` meter ($) on run-detail
  and a `Cost cap (USD)` field on run-form/configure (opt-in).
- **Questions.**
  1. A spec re-read suggested different config defaults (delivery iteration_cap **50**;
     budgets default **0**/unlimited). Which is canonical?
  2. `budget.usd` is not in TechSpec/ADR-012. Is cost a first-class budget dimension with
     a ceiling, or display-only (derived from tokens × price)?
- **Proposal.** (1) Reconcile the canonical default numbers in TechSpec + `config.toml`
  defaults so the UI copy matches one source; if 50/0 win, update all four screens. (2) If
  cost is display-only, keep the run-detail Cost meter but **remove the `Cost cap (USD)`
  input** from run-form/configure (truthful-UI). If cost caps are real, add `budget.usd`
  to ADR-012 with a ceiling.

### 9.6 Configure write target vs fork `[P1]`
- **Design.** Configure writes per-loop defaults (limits, enabled checks, human-gate flag,
  re-attempt mode, project command overrides) without forking; structural edits require a
  fork.
- **Question.** Confirm the daemon has a per-loop **config** store distinct from the loop
  **definition**, and the exact boundary of what config can change.
- **Proposal.** Define `loop config` as a separate persisted object (`GET/PUT
  /loops/:name/config`, `agh loop config`); document the no-fork boundary verbatim
  (the "what cannot change" list in 4.7).

### 9.7 Node layout persistence `[P1 for editor]`
- **Design.** The editor positions nodes on a canvas.
- **Question.** ADR-015 mentions UI annotations keyed `(workspace_id, loop_name,
  node_id)`. Confirm the daemon persists node x/y layout.
- **Proposal.** Confirm/define a node-annotation store + read/write path so layouts
  survive reload and never collide across same-named forked loops in different workspaces.

### 9.8 Approval-gate decision verbs `[P1]`
- **Design.** `Approve & resume`, `Request changes`, `Reject & halt`.
- **Question.** Confirm the human-gate decision set and that `Request changes` routes to
  `revise` (next generation) while `Reject & halt` → terminal `failed`/halt.
- **Proposal.** Document the decision verbs + their routing in the gate/ADR-005 section and
  expose `agh run approve|request-changes|reject`.

### 9.9 channel-post / converse-and-decide as runtime primitive `[P0]` - REVERSED (ADR-021)
- **Design (round 4).** `run-detail` rendered an embedded multi-agent channel as a
  first-class `channel-post` action node with a harvested decision; the editor palette
  offered `channel-post`.
- **Resolution (ADR-021, delete target).** The `channel-post` primitive is retired and
  does not exist as a kind. Channel posting is `kind: agh__network_send`; the UC2
  converse-and-decide completion contract survives as the **harvest spec**, not a kind:
  the node may declare `harvest: {kind: channel_result, window, responder?,
  content_rule?}` (post the request, await the designated result message, harvest its
  payload; no result within the window → `stalled`, unchanged ADR-014 semantics). The
  editor palette keeps a "Channel post" shortcut that inserts a pre-filled
  `agh__network_send` node, so the UX survives while the dual path is eliminated. The
  run-detail channel surface stays and binds to the tool node plus its harvest.

### 9.10 Picker data sources: agent / ref / file kinds `[P1]`
- **Design.** Run form renders an `agent` picker; the type system includes `ref` and
  `file`. The editor kind dropdowns list registry actions/tools/agents.
- **Question.** What entity kinds can a `ref` target, and what feeds the agent/tool/action
  pickers?
- **Proposal.** Enumerate `ref` target kinds (task/run/channel/...) in the DSL input-type
  section, and define the registry-list endpoints the pickers read.

### 9.11 Computed aggregates: success rate, run counts, 30d stats `[P2]`
- **Design.** Catalog + detail show success rate, total runs, avg generations, median
  duration; runs KPIs show active/awaiting/done/needs-a-look.
- **Question.** Are these daemon-provided aggregates or a computed view?
- **Proposal.** Confirm an aggregate endpoint or document them as a derived read model so
  the UI is not inventing metrics.

### 9.12 Watch loops: unbounded cap + quiet/stall rendering `[P2]`
- **Design.** Catalog shows `reviews-watch` cap `∞`; runs shows watching/quiet/stall.
- **Question.** Confirm watch loops default iteration_cap 0 (unbounded), the quiet-window
  → stalled transition, and how the UI should render "unbounded".
- **Proposal.** Confirm in ADR-012 + the watch-source spec; standardize the `∞` rendering.

### 9.13 Versions + version diff `[P2]`
- **Design.** Editor shows `v5 · draft` + a version selector; detail shows a versions list;
  diff/revert are referenced.
- **Question.** Confirm the version model (draft vs published, revert creates a new
  version) and whether version-diff is in v1 scope.
- **Proposal.** Document the version lifecycle; schedule version-diff as a follow-up
  surface if out of v1.

### 9.14 Start bindings + automation loop-targets `[P1 · surfaced in design]`
- **Design.** ADR-007's `start[]` allowlist and its automation carriage are now surfaced
  across the loop screens (see 5.6): the loop-detail Start bindings panel, catalog binding
  badges, the editor's read-only start chip strip, automation-named triggers on run-detail,
  and the run-form hint. The trigger/job create sheets gain a discriminated **target**,
  `Run agent` (today) or `Run loop` `{workspace, loop, static inputs, event-payload → input
  mapping}`; that change is spec-only (TechSpec §9.14) and intentionally has NO design HTML
  in this pass (the sheets are existing AGH automation surfaces).
- **Question.** Confirm the daemon surface for listing automations by loop target (name,
  kind, enabled state, cron + next fire, endpoint slug, event name) and the pre-targeted
  deep-link contract for `Add trigger` / `Add schedule`.
- **Proposal.** Expose `GET /loops/:name/bindings` (or `GET /automations?target=loop:<name>`)
  returning the attached trigger/schedule/webhook automations with enabled state and
  kind-specific meta, plus create-sheet deep-links pre-filled with the `Run loop` target.
  Keep watch-source strictly a body-node concept, never listed as a start binding.

### 9.15 Validate endpoint + optimistic-concurrency writes `[P1]` - ADDED (ADR-023)
- **Design impact.** The editor's `Validate` action and the 422 publish gate bind to a
  real surface: `POST /api/workspaces/:wid/loops/:name/validate` runs lint+compile
  WITHOUT saving and returns 422-shaped per-node errors with deterministic codes
  (`unknown_reference`, `node_id_invalid`, `verdict_policy_requires_judge`,
  `fan_out_ceiling_exceeded`, ...). `PATCH /loops/:name` requires `expected_version`
  (CAS); a mismatch returns 409 with the current version, so two agents editing the same
  loop can no longer last-write-wins each other. Publish compiles and persists the
  resolved form the runtime exclusively consumes. The editor must surface a
  version-conflict state on save; the same validate surface ships as `agh loop validate`
  and a `validate` mode on `agh__loop_create`.

### 9.16 Run status enum: 8 → 11 states `[P0]` - ADDED (ADR-021/022)
- **Design impact.** The run status enum is exactly 11 states: live `running | watching
  | needs-approval | paused | queued` (queued = a deferred start under
  `concurrency: queue`, ADR-021); terminal `done | no_op | blocked | failed | exhausted |
  stalled`. `no_op` (ran, nothing to do) renders neutral and is never shown as
  `done`-with-fake-work; `blocked` (external dependency) renders in the warning family,
  distinct from `stalled`. Every status surface (pills, runs filters, KPI copy,
  terminal-outcome legend, SSE `status_changed`, CLI, `agh__loop_status`) uses only
  these; `ready` is a node-level pill only, never a run status.

### 9.17 run-agent params: agent + prompt `[P0]` - ADDED (ADR-021)
- **Design impact.** The round-4 editor's `action_ref` field and closed 3-item kind
  select are invalidated and removed. `run-agent` is authored via `params: {agent
  (REQUIRED, interpolable), prompt (REQUIRED template), output_schema?, cwd?, model?,
  allowed_tools?, max_turns?}`; every non-reserved action kind is a literal ToolID whose
  params form renders from the registry schema (181 native tools plus `ext__`/`mcp__`).
  Sections 4.6 and 5.2 carry the new palette and inspector field sets.

### 9.18 Enforced budgets + on_exceeded `[P1]` - ADDED (ADR-022)
- **Design impact.** `budget: {tokens, wall_clock_sec, on_exceeded: halt | escalate}`
  stays opt-in (0 = unlimited) but is ENFORCED when set: `halt` (default) → terminal
  `exhausted`; `escalate` → `needs-approval` for a human decision. Run-form/configure
  budget fields gain the `on_exceeded` selector; run-detail meters can reach the ceiling
  and must render the resulting `exhausted` / `needs-approval` truthfully. Cost stays
  display-only (§9.5.2 unchanged; no USD cap).

### 9.19 run-loop await/detach + `queued` concurrency `[P1]` - ADDED (ADR-021)
- **Design impact.** `run-loop` (reserved kind) starts another loop definition: `mode:
  await` (default) is yield-and-rewake, never a held lease · the node parks in the
  per-node state `awaiting_child` (non-terminal for the generation-finish gate) until the
  child's `loop.terminal` wakes the parent, which resolves the instance to
  `{status, outputs}`; `mode: detach` returns `{loop_run_id}` immediately (required for
  `iteration_cap: 0` watch children). Ancestry is cycle-guarded at runtime (depth <= 8).
  `concurrency: queue` makes a deferred start a real `loop_runs` row in the live status
  `queued`, promoted FIFO when the active run reaches terminal. Run-detail must render
  `awaiting_child` on a run-loop node and `queued` as a live run status; the timeline
  never finalizes a generation while any instance awaits a child.

### 9.20 Fan-out batching: `batch_size` / `max_parallel` / `max_fan_out` `[P1]` - ADDED (ADR-006)
- **Design impact.** `fan-out` carries three orthogonal knobs, never one: `batch_size`
  (items per agent branch, default 1; `software-delivery` = 1 → one task per branch,
  `reviews-watch` = 10 → contiguous chunks of <=10 issues in fetch order), `max_parallel`
  (concurrent branches; both examples = 1 → sequential) and `max_fan_out` (structural cap
  on materialized branches, clamped by the daemon ceiling 64; overflow → `exhausted` /
  `escalate`). Run timelines render fan-out as BATCH BRANCHES
  (`batch 1 · issues 001-010 · 10 issues` for `reviews-watch`; `task_01 · schema
  migration` for `software-delivery`); the editor inspector and the read-only DAG expose
  all three knobs. Only a status dot carries state on a batch chip; the content stays
  neutral mono.

---

## 10. Open decisions (carried from the handoff, now consolidated)
- **Palette:** AGH operator (warm-dark + orange), matching the real app. A Linear-indigo
  reskin is a pure token swap.
- **App shell:** mockups are content + header only; the real app supplies rail/sidebar.
- **Editor canvas texture:** a very subtle dot-grid (functional "this is a canvas" signal)
  is the one place a `radial-gradient` is used; remove if strict no-gradient is preferred.
- **Remaining optional surfaces (not yet designed):** version diff, node-run drilldown
  modal, watch-source quiet/stall detail state, empty states.

---

## 11. How to use this with the spec
1. Walk section 9 top to bottom; for each, open the cited spec file and either confirm the
   design or file a spec change. P0 items (9.5, 9.16, 9.17) gate a truthful build; 9.2
   and 9.9 are resolved by ADR-020/021.
2. During implementation, use sections 4 (per-screen contract), 6 (component inventory),
   and 7 (data surfaces) as the build brief. The canonical per-kind config schema is the
   DSL types (task 02) plus the registry tool schemas (ADR-023); `loop-editor.html`'s
   inspector model is a render of them, never the source.
3. Keep this file in sync: when a screen changes, update its section 4 entry and re-check
   section 9.
