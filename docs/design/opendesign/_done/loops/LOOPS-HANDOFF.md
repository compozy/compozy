# Loops feature — design handoff

> **Purpose of this file.** A cold-start briefing so a _new chat with another LLM_
> can continue designing AGH's "Loops" feature screens without re-deriving anything.
> Paste this whole file into the new conversation as context. It records what was
> built, what's left, the design system, and the spec model. **HTML mockups are the
> deliverables — this doc only describes them.**
>
> **Last update:** redesign pass complete. The first five screens were rebuilt to
> match the real AGH operator UI (much quieter, color = state only) and then stripped
> down to **content + header only** (the app rail + nav sidebar live in the real app,
> so the mockups must NOT include them). Read §4 and §10 carefully before building more.
>
> **Start-bindings pass (ADR-007, 2026-07-02):** loop-detail gained a Start bindings rail
> panel, the catalog a binding badge, the editor graph view a read-only start chip strip,
> run-detail automation-named triggers, and the run form a hint line. The trigger/job create
> sheets' `Run loop` target is spec-only (TechSpec §9.14; no design HTML for those sheets).
> See LOOPS-DESIGN-SPEC.md §5.6 + §9.14.
>
> **Round-5 spec pass (ADR-020..023, 2026-07-03):** the DSL pinned one reference grammar
> (Go-template `{{ }}` values + CEL boolean conditions over a single validated namespace;
> snake_case node IDs, kebab kinds). The action model is now 3 reserved kinds (`run-agent`,
> `run-loop`, `transform`) plus any literal ToolID; `call-tool` and `channel-post` do not
> exist as kinds (channel posting = `agh__network_send` + `channel_result` harvest, kept as
> a palette shortcut). The run status enum grew to 11 states (adds terminal `no_op` and
> `blocked`; `queued` is a live status for deferred starts under `concurrency: queue`,
> ADR-021); gates became `criteria[]` + `verdict_policy`; set budgets are enforced with
> `on_exceeded: halt | escalate`; the write surface gained `POST /loops/:name/validate` and
> `expected_version` CAS on PATCH. §3 below reflects the round-5 model, and the HTML
> artboards are being updated to the same rules.

---

## 1. Mission

Design **all UI screens** for AGH's new **Loops** feature — a mix of the new
"loop engineering" paradigm with **static DAG workflows**. Deliverables are
high-fidelity, **self-contained HTML** mockups (one file per screen, inline CSS/JS).

**Scope of each mockup = page CONTENT + its in-page header (topbar) only.** The
workspace rail (56px) and nav sidebar (244px) already exist in the real app and were
intentionally removed from these references. Do NOT re-add them. (See §10.)

**Ground truth = the spec only. Do not invent features the spec doesn't describe.**
Spec folder (this repo is linked read-only in the design project):

```
/Users/pedronauck/dev/compozy/agh/.compozy/tasks/loops/
  _prd.md  _techspec.md  product-ux.md  use-cases.md  requirements.md
  adrs/adr-001..023   analysis/ (incl. the round-5 set + summary-round5.md)
```

The AGH design system is the other source of truth:
`/Users/pedronauck/dev/compozy/agh/DESIGN.md` + `PRODUCT.md`
(token source: `packages/ui/src/tokens.css`). Real UI patterns to copy live in
`web/src/systems/*` and `packages/ui/src/components/*`.

## 2. Where the artifacts live

The mockups are in the Open Design project workspace (NOT in the agh repo):

```
/Users/pedronauck/Library/Application Support/Open Design/namespaces/release-stable/data/projects/bb1d96e1-f5c1-4478-a88b-10abd624f30a/
```

Each screen is one standalone `.html`, previewable on its own.
(`loops-index-2.html` — the old phantom duplicate — has been **deleted**.)

## 3. Product model (keep this accurate — verified against the spec)

- **Two nouns only:** **Loops** (the catalog) and **Runs** (the executions). No third
  noun — no "ready to run", "spec", or "workflow" screens.
- A **Loop** = a **contract** (goal → verify → stop) + a **body** (a static DAG of
  steps) + typed **declared inputs**. It runs on AGH's autonomy kernel (not a 2nd executor).
- **Arrive-and-use is the hero path:** pick a Loop → fill an auto-generated input form →
  Run → watch it iterate live. Built-ins run with zero authoring.
- Runs iterate across **generations**. Re-attempt strategy: `failed-only` (default —
  re-runs failed/pending nodes **plus their transitive downstream dependents**; carries
  succeeded node outputs forward as read-only) | `full-body`.
- **Run statuses (11 total, ADR-021/022), always explicit & honest.** Terminal: `done` ·
  `no_op` (ran, nothing to do; neutral, never fake work) · `blocked` (external
  dependency; warning family) · `failed` · `exhausted` · `stalled` (work possible, no
  progress). Live: `running / watching / needs-approval / paused / queued` (queued = a
  deferred start under `concurrency: queue`, promoted FIFO at terminal);
  **`needs-approval` is a LIVE pause, not terminal.** `ready` is a node-level
  state only, never a run status.
- **Node classes (ADR-003/021):** `action` = 3 reserved kinds (`run-agent`, `run-loop`,
  `transform`) plus any literal ToolID (`agh__*`/`ext__*`/`mcp__*`; params = the tool's
  input schema, 181 native tools) · `control` (fan-out, collect, branch, gate, sub-loop) ·
  `source` (watch-source, file-import, input). `call-tool` and `channel-post` do NOT
  exist as kinds: channel posting is `kind: agh__network_send` + optional
  `harvest: {kind: channel_result, window, responder?, content_rule?}` (the
  converse-and-decide result convention), with a "Channel post" palette shortcut that
  inserts the pre-filled tool node. `run-agent` params: `{agent (REQUIRED, interpolable),
  prompt (REQUIRED template), output_schema?, cwd?, model?, allowed_tools?, max_turns?}`.
  Node IDs are snake_case (`load_tasks`, `fetch_issues`); kinds stay kebab-case.
- **Verification (ADR-005/022):** a gate (and `contract.verification[]`) declares
  `criteria[]`, 1..N typed checks `{id, type: command|agent-judge|human|extension, ...}`,
  plus `verdict_policy: revise_until_clean | fixed_passes` (revise_until_clean requires a
  judge/human criterion), `on_result`, `max_revisions`. `agent-judge` emits a structured
  verdict `{verdict, blocking_issues[{id, note}], confidence, evidence}`; malformed
  output degrades to revise, never pass.
- **Reference grammar (ADR-020):** one namespace, two surfaces, both compile-validated.
  String values interpolate Go-template `{{ }}` (`{{ .inputs.slug }}`,
  `{{ .nodes.load_tasks.output.tasks }}`); boolean condition fields
  (`branch.condition`, `contract.stop_when`, collection `filter`) are CEL
  (`nodes.review.output.verdict == 'pass'`). No `${...}` refs, no bare dotted refs.
  Namespace: `inputs.*` · `nodes.<id>.output.*` · `nodes.<id>.status` · `item`/`index`
  (fan-out only) · `trigger.*` · `generation`.
- **Limits & daemon ceilings** (per-loop default / hard ceiling): `iteration_cap` 50/100 ·
  `budget.tokens` off (0)/20M (0 = unlimited; opt-in) · `budget.wall_clock_sec` off (0)/7d
  (opt-in) · cost display-only (tokens × price; no USD cap input, §9.5.2) ·
  `no_progress.window` 3/10 · `fan_out_ceiling` ≤ tasks/64 · `gate.max_revisions` 3/10.
  Ceilings are hard backstops, never editable in the UI. A SET budget is ENFORCED
  (ADR-022): `budget.on_exceeded: halt` (default, → terminal `exhausted`) | `escalate`
  (→ `needs-approval`). Watch loops default to `iteration_cap` 0 (unbounded).
- **DSL `agh.loop/v1`:** `meta`, `concurrency (forbid|allow|queue, default forbid,
  ADR-021)`, `inputs`, `contract { goal, definition_of_done, constraints?, boundaries?,
  stop_when?, verification[], terminal_states (the 6 terminal outcomes), iteration_cap,
  no_progress, budget { tokens, wall_clock_sec, on_exceeded } }`,
  `graph { nodes[], edges[] }`, `start[]`. Declared-input types:
  `string/number/boolean/file/agent/ref`.
- **Start bindings (ADR-007):** the `start[]` allowlist
  (`manual|cli|http|uds|trigger|schedule|webhook|network|extension|native_tool`) declares how
  a Loop may be initiated. Event-driven starts (trigger/schedule/webhook) ride AGH's EXISTING
  automation primitives: a Trigger or Job gains a discriminated target, `Run agent` (today) or
  `Run loop` `{workspace, loop, static inputs, event-payload → input mapping}` (spec-only,
  TechSpec §9.14; no design HTML for those sheets). Surfaced in the mocks: loop-detail Start
  bindings rail panel (declared kinds as neutral mono chips + attached automations with
  enabled dot + inert `Add trigger` / `Add schedule` CTAs), catalog binding badge
  (a nightly schedule `0 3 * * *`), editor read-only start chip strip (graph view only;
  edited in the DSL view), run-detail automation-named trigger
  (`schedule · nightly · 0 3 * * * (job)`; bare kinds for web/cli/agent), run-form hint.
  A watch-source is a node INSIDE the body, never a start binding; never merge the two.
- **Linter invariants** (the builder _surfaces_ them, never re-implements): acyclicity ·
  reachability · termination · fan-out finiteness, plus reference validation
  (`unknown_reference`), snake_case IDs (`node_id_invalid`) and gate policy
  (`verdict_policy_requires_judge`). Publish (`PATCH`, requires `expected_version`;
  mismatch → `409` + current version) returns `422` with per-node errors and persists
  the compiled resolved form; `POST /loops/:name/validate` (and `agh loop validate` / a
  `validate` mode on `agh__loop_create`) runs the same lint+compile without saving
  (ADR-023).

## 4. Design system — REBUILT to match real AGH (read before coding)

The screens now mirror the **real AGH operator UI** (`web/` + `packages/ui`), per
`DESIGN.md`. The atmosphere: warm near-black canvas, quiet/dense/intentional, flat depth,
a **single** accent, signal colors as **desaturated tint + text** (never solid banners).

**The golden rule: color carries STATE, not category.** This was the #1 problem with the
first draft. Node _type_ (action/control/source) is a **neutral** mono label — it is NOT
color-coded. Only run/node _state_ gets color (success/warning/danger/accent/info), and
even then as a small dot or a tint pill.

Tokens (these are the AGH tokens — copy verbatim into every screen's `:root`):

```
surfaces  --rail #0c0b0b · --canvas #131211 · --canvas-soft #1a1918 · --canvas-tint #1c1b1a · --elevated #232220
text      --fg #ececef · --fg-strong #f6f6f8 · --muted #9a9a9f · --subtle #76767c · --faint #545458
lines     --line rgba(255,255,255,.055) · --line-soft .03 · --line-strong .09
accent    --accent #e8572a · --accent-hover #d14e25 · --accent-strong #f6874f · --accent-ink #17110f · --accent-tint .10
          (SCARCE — primary CTA, active nav icon, running/active state, brand mark only)
status    success #5fbf85/.08 · warning #d6a647/.08 · danger #e0635a/.09 · info #8e8eb5/.12 · neutral #7a7a80/.06
          (always tint-bg + saturated text; NEVER solid fills)
glaze     --row-hover .022 · --row-selected .03 · --input-fill .025 · --badge-fill .05 · --btn-fill .04 · --bar-fill .18
type      Inter Variable (font-feature cv01,ss03) + JetBrains Mono
scale     body 13.5px · detail-h1 22px/-.028em · section label 10.5-11px uppercase · mono ids ~11px
radii     4/5/6/8/10/14 · pill 9999 · motion 140ms cubic-bezier(.2,0,0,1)
```

- **Layout = content + header only.** Each file is `.shell{height:100vh}` → `.main`
  (flex column: a 48px `.topbar` header that's `flex:none` + a scrolling region
  `flex:1;min-height:0`). NO workspace rail, NO nav panel. The in-page right rails that
  carry per-loop/per-run info (`run-detail` events/facts column, `loop-detail` inputs/limits
  column) ARE content — keep them.
- **Header (topbar):** 48px, `border-bottom:1px solid --line`. Inventory pages (catalog, vault-style) use **icon + title + count** on the left (Vault/Bridges shell) — not a breadcrumb. Detail/editor pages keep breadcrumb (`Loops › software-delivery`). Trailing actions: **ghost secondary + primary CTA** (`btn--ghost btn--sm` + `btn--primary`); listing search lives in the page toolbar, not the topbar.
- **Page head (in body):** H1 at `detail-h1` (22px, weight 560), a mono count-chip, and a
  thin muted meta line. No marketing lead paragraphs, no glowing eyebrow dots.
- **Status → visual (shared `.pill--*` vocabulary, tint + text):** running = accent + pulse ·
  watching = info + pulse · needs-approval = info · done = success · failed = danger ·
  exhausted = warning · stalled = neutral. In dense tables a 6px state dot inside the pill
  is enough; reserve the pulse for live states only (wrap pulses in `prefers-reduced-motion`).
- **DAG / nodes:** neutral surfaces (`--canvas-tint`), node class shown as a small mono
  uppercase label (`--faint`/`--subtle`); gates may tint their verdict text by tone.
  Build with positioned divs/SVG (ReactFlow-style), not a real graph library.

**Hard rules (impeccable — these were violated in the first draft, do not repeat):**

- One accent, used sparingly. No rainbow category colors. Color = state only.
- **No side-stripe borders** (`border-left/right > 1px` as accent). Use full borders + tint.
- **No gradients** (no radial hero glows, no gradient card/gate/header backgrounds).
- **Flat depth** — no shadows on content cards; only overlays/modals may use `--shadow-overlay`.
- **No em dashes in copy** — use commas/colons/periods/parens. (`—` as a "no value"
  placeholder, e.g. `Cost — unset`, is fine.)
- Real product UI only — no demo/viewport/theme/platform toggles, no metadata/process cards
  ("Ready/Next", "the screenshot that sells the product", etc.). No AI-slop emoji rows.
  Honest placeholders over invented stats.

## 5. DONE (all rebuilt this session: quiet AGH pattern + content-only)

| File                 | Screen                          | Contents                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| -------------------- | ------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `loops-index.html`   | Overview / launcher             | Calm 2-noun explainer + a neutral grid of screen cards (Built/Planned tags), design-system note. This one is a standalone launcher (no app shell by nature); links to the screens below.                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| `loops-catalog.html` | Loops catalog                   | Vault-style topbar (icon + Loops + count · Runs ghost + New primary). Listing toolbar: compact **SearchInput** before **reui Filters** (kind / category / status / mode / name) + **PillGroup Rows\|Cards** (no “Sorted by”). Default = rows; cards = `CatalogCard` grid. Empty state, binding badge, Run → run-form. Listing standard in `LISTING-STANDARD.md` (also applied on `vault-redesign.html` — flat list, `Pill mono` type in trail/footer, date under card title, delete always visible). |
| `loop-detail.html`   | Loop definition page            | DetailHeader (Run = the one accent CTA · Configure · Fork & edit). Section pattern (label + hairlines): Contract (goal/DoD/verification kinds/terminal chips), read-only body DAG (neutral nodes; 8 nodes for software-delivery), recent runs. Right rail (content): declared inputs, start bindings (declared kinds + attached automations + Add trigger / Add schedule CTAs), limits vs ceilings, versions, 30d stats.                                                                                                                                                                                                                                                                                               |
| `run-detail.html`    | **Live run monitor** (heaviest) | Sticky contract header (goal + running pill) + 5 neutral live meters (attempts/tokens/wall/cost/breadth; warn color only near ceiling). Generation timeline on a flat node spine: G1 revised (slug → load_tasks file-import → implement fan-out over the authored tasks in dependency order as batch branches, batch_size 1 so each branch is one task e.g. `task_01 · schema migration`, 1 failed branch → collect → review agent-judge → request_changes → revise; node IDs snake_case); G2 running (carry-forward, execute_task re-running only the failed task via run-agent, embedded channel (`agh__network_send` + `channel_result` harvest, the converse-and-decide convention) with desaturated avatars, pending verify, human approval gate). Right info rail (content): live events, run facts (automation-started runs name their trigger: `schedule · nightly · 0 3 * * * (job)`; bare kinds for web/cli/agent), terminal-outcome legend. Gates are flat tint cards (no side-stripe/gradient). JS ticks meters/events + collapsibles, guarded by reduced-motion. |
| `runs.html`          | Runs history                    | KPI strip (AGH metric scale), outcome filter segment (JS) + Loop/date selects, Active vs Past tables across the full outcome spectrum, neutral budget mini-bars (warn/danger only near limit), → run-detail.                                                                                                                                                                                                                                                                                                                                                                                                                 |

Cross-screen consistency is locked: identical `:root` token block, shell, `.pill--*`
vocabulary, type scale. Spec numbers fact-checked (limits, states, node count = 8 for
software-delivery, sidebar "Runs" count = 3 active).

## 6. REMAINING (priority order — build content + header only, same tokens/shell)

1. **`loop-editor.html` — Visual DAG builder (fork & edit; the other heavy screen).**
   Canvas of action/control/source nodes (neutral, class as mono label) + edges; node palette
   (reserved run-agent / run-loop / transform, curated tool shortcuts incl. "Channel post" →
   pre-filled `agh__network_send`, and a searchable "Call tool..." picker over the 181-tool
   registry); right node-inspector rendered FROM the DSL types + registry tool schemas
   (ADR-023; run-agent: `params.agent`/`params.prompt`/`output_schema`/`cwd`/`model`/
   `allowed_tools`/`max_turns` + envelope `session`/`timeout`/`retry`/`harvest`; ToolID
   actions: params form generated from the tool's input schema; gate:
   `criteria[]`/`verdict_policy`/`on_result`/`max_revisions`; fan-out: `collection`
   template/`batch_size`/`max_parallel`/`max_fan_out` (ADR-006); branch: validated raw CEL
   condition with autocomplete); inline
   **linter panel**
   (acyclicity/reachability/termination/fan-out finiteness) with per-node error highlight;
   toolbar (add/validate/publish); version sidebar. Positioned divs/SVG, not a graph library.
   A read-only viewer variant is reused on the detail/run pages. NB: opens EXISTING loops only
   (fork-and-edit) — ADR-008 forbids a blank-canvas from-scratch builder. Graph view carries a
   read-only start chip strip pinned at the canvas origin (`start: manual · cli · http · uds ·
   native_tool · schedule`, muted note: edited in the DSL view); no graph-side start editing in v1.
2. **`loop-run-form.html` — Run a loop (hero path).** Auto-generated input form from the
   Loop's declared inputs (typed fields: string/number/boolean/file/agent/ref) with required
   validation; **Advanced** panel = per-run limit overrides bounded by the daemon ceiling
   (greyed at ceiling); live contract/preview pane. Two-column form+preview, matching the
   `create-trigger-redesign.html` / `create-task-redesign.html` / `create-job-redesign.html`
   pattern (but content-only). Run + Dry-run. Under the form, a muted start-bindings hint:
   "This loop also starts from: schedule. Manage in Start bindings on the loop page."
3. **`loop-configure.html` — Configure (light; power ceiling, never the hero).** No-fork
   tweaks: verification-check select, human-approval gate toggle, re-attempt granularity
   (`failed-only` | `full-body`), per-loop limit overrides. Sheet/modal; no structural change;
   Save / Reset to defaults.

Optional later: version diff, node-run drilldown modal, watch-source quiet/stall state, empty states.

## 7. How to continue (workflow)

1. Read this file + `DESIGN.md`/`PRODUCT.md` + the spec folder. Don't invent — every
   term/limit must trace to the spec. **Use the `impeccable` skill** (it loads the AGH
   register + rules); the user expects it applied extensively.
2. Copy the `:root` token block + the content-only shell (`.shell{height:100vh}` + `.main`
   topbar/scroll) from `loop-detail.html` or `run-detail.html`. Keep each file fully
   self-contained (inline CSS/JS). Do NOT add a workspace rail or nav panel.
3. Build the screen, grounding all data in §3 and reusing the `.pill--*` status vocabulary.
4. **Flip its card on `loops-index.html`** from "Planned" → "Built".
5. Self-check against §4's hard rules (color=state, no side-stripe/gradient/em-dash, flat,
   one accent), then a quick 5-dim critique (philosophy/hierarchy/execution/specificity/
   restraint) before shipping.

## 8. Open decisions / flags

- **Global Runs route:** `product-ux.md` puts run history **under each Loop** and says there
  is **no separate global Runs nav route**. `runs.html` was kept as an operator convenience
  (and as the `run-detail` "All runs" target). If strict spec-IA is wanted, fold it into the
  catalog/loop-detail instead. (Decision pending the user.)
- **Palette:** AGH **operator** (warm-dark + orange), matching the real app. A Linear-indigo
  reskin would be a pure token swap.
- **App shell removed by request:** mockups are content + header only; the real app supplies
  the rail/sidebar. New screens must follow suit.

## 9. Suggested skills (for the next session)

- `impeccable` — UI quality (hierarchy, states, accessibility, anti-slop). **Use it.**
- `agh-design` — AGH branded UI/asset generation.
- `agh-ui-screenshot` — deterministic PNGs for visual-parity checks.
- `compozy` — repo conventions, for when this moves to real implementation.

## 10. Key lessons from the redesign pass (don't regress)

- **Color = state, never category.** The first draft rainbow-coded node classes
  (action=blue, control=amber/purple, source=green) and per-category catalog icons. All
  neutralized. Keep nodes/icons neutral; let success/warning/danger/accent/info mark state only.
- **Quiet > dense-and-loud.** Dropped marketing-scale headlines (34–58px → 22px), glowing
  eyebrow dots, marketing lead paragraphs, hover-lift transforms, and nested bordered cards.
  Prefer the Section pattern (uppercase label + hairline-separated rows) over stacked cards.
- **Banned and removed:** side-stripe borders, gradient backgrounds (incl. the launcher's
  radial glow), peach/off-palette inline-code colors, em dashes in copy.
- **Content + header only:** no workspace rail, no nav panel. In-page info rails are content.
- These screens are the reference for the real `web/` implementation, so they must look like
  AGH already — earned familiarity, the tool disappearing into the task.
