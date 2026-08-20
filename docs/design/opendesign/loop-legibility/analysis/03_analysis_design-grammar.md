# Analysis: design-grammar

Read-only exploration of the slice `design-grammar` (ordinal `03`) for the research prompt:

> What visual contracts, incumbent grammar, production seams, and canonical data must the loop-legibility design set honor so the six artboards under docs/design/opendesign/loop-legibility/ can be built without inventing states, copy, chrome, or entities? Extract only design-actionable facts for S1 (tasks list), S4 (run default + needs-you), S5 (DAG + roster), and S6 (runs roster).

## Scope

- Slice question: What locked visual grammar — tokens, shell/lab layout, needs-you request tones, set-file conventions, reusable primitives — must every loop-legibility board inherit so the set continues graph-eng rather than forking a second language?
- Primary sources: `docs/design/opendesign/graph-eng/` (DESIGN-NOTES.md, graph-eng.css, needs-you / timeline / progress / node-verbs / index), `docs/design/opendesign/design-system/` (ds-core.css, ds-shell.css, GUIDE.md), `docs/design/opendesign/_done/loops_v2/` (DESIGN-LESSONS.md, loop-run-detail.html, loop-inventories.html, index.html), `docs/design/opendesign/_done/tasks/` (task-detail.html, TASK-DETAILS-REDESIGN-PLAN.md), `docs/design/opendesign/command-palette/` (DESIGN-NOTES.md + index.html; CSS chaptering only), `packages/ui/src/tokens.css`, `packages/ui/src/index.ts` (via its re-export barrels), `DESIGN.md` (token tables + anti-patterns + signal palette), `COPY.md` (loop/task/status label maps).
- Sources read in full vs. sampled: read in full — graph-eng DESIGN-NOTES.md, graph-eng index.html, GUIDE.md, command-palette DESIGN-NOTES.md, command-palette index.html, loops_v2 DESIGN-LESSONS.md, loops_v2 index.html, tokens.css color/type block, DESIGN.md §§1–2 + §10, COPY.md §§6 + 8–9, primitives.ts + listing/foundation/viz/conversation export barrels. Sampled — graph-eng.css chapters 1–12 headers + lab scaffold + append mark; needs-you / timeline / progress / node-verbs HTML provenance + locked copy; ds-core.css `:root` + signal + buttons; ds-shell.css waiver + wallpaper; command-palette.css header + chapter marks; loops_v2 run-detail + inventories heads/needs/listing; task-detail.html tokens + 44px head; TASK-DETAILS-REDESIGN-PLAN.md principles.
- Total candidate sources surveyed: 24 files in-scope (plus export barrels required to name `index.ts` symbols). Did not read `web/src/systems/`, `_spec.md`, or `UI-ALIGNMENT-SPEC.md`.

## Overview

Loop-legibility must continue the graph-eng visual language, not open a sibling dialect. The locked contract lives in `graph-eng/DESIGN-NOTES.md`: one request→tone/glyph/word map, exact wait-kind sentences, exact decision verbs, and a lab geometry of full-viewport labs, staged fragments at fluid ≤1240, and a 320px rail that stays pixel-true. Token truth is `packages/ui/src/tokens.css` (`--color-canvas`, `--color-fg`, `--color-muted`, `--color-accent` as the action hue, `--color-success` / `--color-warning` / `--color-danger` / `--color-info`). Prototype boards speak the unprefixed `ds-core.css` aliases of those same values. There is no `--color-action` token.

Set-file convention is already standardized across graph-eng and command-palette: `DESIGN-NOTES.md` owns semantics, `index.html` is the 880px hub, each board is final surface then states lab, domain CSS is chaptered and append-only after a marked point, artboard CSS is a contract never imported into production. New loop-legibility CSS either appends after graph-eng’s mark or starts a new set-scoped file that does not restyle chapters 1–12.

Adjacent slice 01 owns daemon states; slice 02 owns production TSX seams. This slice only locks what the four named boards must *look and read like*: S1 listing chrome, S4 needs-you + waits, S5 DAG/progress/node roster grammar, S6 runs roster. `loops_v2` and `tasks` supply process directives and some reusable chrome (44px `w2head`, Rows|Cards, collapsible sections) but their wait vocabularies, focus rings, topbar heights, and invented editor chrome are anti-references.

## Mechanisms / Patterns

- **Request→tone/glyph/word map (run-page, do not propose alternatives):** pending human request = warning + `triangle-alert` + `pending`. Near-expiry = same warning + `clock` + shared mono countdown (`expires 4m`). Expired = danger + `circle-alert` + `expired` (route taken, never an answer form). Answered / amended = info + `check` / `pencil` + `answered` / `amended` (actor + decision stay). Partial join = warning and the chip always contains the word `partial` plus coverage (`2 of 3 lanes`). Canceled-by-strategy / route-not-taken / never-materialized = neutral + `minus` / `git-branch` / `circle-dashed` + the literal state word. Fork lineage = info + `git-fork` + `fork`. Color never travels alone (WCAG 1.4.1): tone + glyph + state word. Source: `graph-eng/DESIGN-NOTES.md:11-34`, rendered as `.req-chip[data-tone]` in `graph-eng.css:96-107` and the legend at `graph-eng-needs-you-requests.html:241-292`.

- **Bell re-ink is not a run-page tone:** ADR-006 reserves danger for bell contexts. The same pending request that is warning on the run page re-inks to danger in the bell. Chapter 12 paints `.sig[data-k="request"]` on `--danger-tint`; the run-list indicator stays warning. Do not leak the re-ink onto S4 cards or S6 roster chips. Source: `DESIGN-NOTES.md:88-95`, `graph-eng.css:1403-1418`.

- **Wait-kind sentences (exact):** ask node `"{label} is waiting for an answer"`; review node `"{label} is waiting for a decision on its proposed action"`. Parked-row anatomy is icon · title · sentence · micro · age. Source: `DESIGN-NOTES.md:43-44`, `graph-eng.css:273-278`, board copy at `graph-eng-needs-you-requests.html:623-651`.

- **Decisions vocabulary (exact):** `approve / edit / reject / respond`. Buttons render only the persisted `decisions` set. An absent decision is omitted, never disabled. `edit` opens the editor pre-filled; `respond` opens an output form against the node output shape. Submit disables the form and waits for the daemon ack — no optimistic paint. Source: `DESIGN-NOTES.md:40-50`, `graph-eng.css:242-246`.

- **Waits rail counts (authorized delta vs older production):** this-run tallies are `pending / escalated / near-expiry`. A zero renders the glyph `0`, never hides. Non-zero values stay warning on the run page. This replaces — for graph-eng and therefore loop-legibility S4 — production’s older `waits / attention / quarantined` and the loops_v2 inventory filters `waiting | quarantined | attention | retrying`. Source: `graph-eng.css:279-282`, `graph-eng-needs-you-requests.html:29-31,208-218`.

- **Truthful request fields:** prompt, redacted context preview + “fetch full”, expected shape, deadline. Redacted values render `••• redacted`, never blank. A request whose run has terminated shows the resolved outcome, never an answer form. Warning lives on the glyph + chip, never a card/panel border. Source: `DESIGN-NOTES.md:35-39`, `graph-eng.css:132-135`.

- **Lab layout (locked):** labs are full-viewport (authorized delta vs the herdr 960px scaffold). Staged page fragments render at production content width — fluid ≤1240 (`--req-win-w:1240px`). The about/waits rail is `--req-rail-w:320px`, pixel-true (matches `WIDTH_DETAIL_INSPECTOR_INLINE = "320px"`). Run page grid is `minmax(0,1fr) 320px`. Editor stages are the one authorized full-bleed exception. Source: `DESIGN-NOTES.md:97-99`, `graph-eng.css:26-34,37-38,151,239`, `command-palette/DESIGN-NOTES.md:355-360`, `packages/ui/src/lib/layout-widths.ts:1-2`.

- **CSS authority and chaptering:** `graph-eng.css` opens with authority chain + semantic contract, then lab scaffold, then chapters 1–12 (1–5 requests, 6 timeline, 7 progress, 8 node verbs, 9 diff, 10 lineage, 11 editor, 12 bell). Later runs append after `/* ==== APPEND POINT — later runs add chapters after this line ====` (`graph-eng.css:1442`); they never restyle earlier chapters. `command-palette.css` is the sibling convention: chapter 0 = set-scoped production-parity token lane + lab-scaffold deltas + shared chrome; chapters 1–12 are surfaces; bug/parity edits in place; new feature runs append-only; chapter 13 is a review appendix that may cross-override only when labeled. Link order: `ds-core.css` → `ds-shell.css` → domain CSS (graph-eng last; palette also inserts `herdr.css`). Artboard CSS is a visual contract, never a stylesheet to import. Source: `graph-eng/DESIGN-NOTES.md:134-138`, `graph-eng.css:1-24,380,987-996,1442`, `command-palette/DESIGN-NOTES.md:392-400`, `command-palette.css:39-75,1311`.

- **Set-file convention (index + boards, not palette UI):** `index.html` is an 880px hub: uppercase group labels, 2-col cards, `tag--final` = “final + lab”, foot cites ds-core/ds-shell + the domain CSS, “Content illustrative; runtime truth owns values.” Each board HTML comment records `production / spec / authorized delta`. `data-od-id` on regions. Fonts Geist + JetBrains Mono. `color-scheme: dark`. Iterate; do not regenerate. Source: `graph-eng/index.html:1-117`, `command-palette/index.html:1-117`, `GUIDE.md:19-28,41-52`.

- **Token names (never invent hex; no `--color-action`):** from `packages/ui/src/tokens.css:16-56` — `--color-canvas` `#131211`; `--color-fg` `#ececef`; `--color-muted` `#9a9a9f`; action hue is `--color-accent` `#e8572a` (CLAUDE/DESIGN “action” slot); `--color-success` `#5fbf85`; `--color-warning` `#d6a647`; `--color-danger` `#e0635a`; `--color-info` `#8e8eb5`; tints `--color-*-tint` as in the file. Prototype alias set in `ds-core.css:27-38` is the same hex without the `--color-` prefix (`--canvas`, `--fg`, `--muted`, `--accent`, …). `--color-subtle` / `--color-faint` are oklch in tokens.css; ds-core still ships hex `--subtle:#76767c` / `--faint:#545458` — production wins on conflict (`GUIDE.md:3`).

- **Signal vocabulary (state, never decoration):** ds-core dots — `.d--run` accent, `.d--ok` success, `.d--needs` warning, `.d--fail` danger ring, `.d--idle` faint (`ds-core.css:81-90`). Session shapes: `running` = accent, `idle` = success, `waiting-for-auth` = warning diamond, `failed` = danger square, `stopped` = neutral ring (`ds-core.css:92-98`). Progress segments: `clean` = success, `active` = `--accent-dim`, `failed` = danger, `parked` = info inset, `canceled`/`never` = neutral, `pending` = `--badge-fill` (`graph-eng.css:500-511`). Partial count chip is warning and says `partial` (`graph-eng.css:534-535`). Accent budget: one primary action + live pulse per screen; no accent card/panel borders (`GUIDE.md:32-33`, `DESIGN.md:1094-1100`).

- **Shell / window chrome:** content stays on the production radius ladder 3/4/5/6/8/10; glass + 12–22px radii are a `ds-shell.css` waiver for menubar/dock/window frame only (`ds-shell.css:7-21`, `DESIGN.md:519-524`). Unified head is 44px, identity once, ≤2 actions, optional 38px tools strip (`GUIDE.md:25,33`; `.w2head` in `graph-eng.css:39-50`). Focus-visible is 2px white @ 50% (`ds-core.css:51-54`); task-detail’s 1px `--line-strong` ring is deprecated drift.

- **`@compozy/ui` names from the `index.ts` barrel (no invented `Badge` / `Disclosure`):** `index.ts` re-exports `./primitives` + `exports/{shell,menus,listing,viz,editors,conversation,foundation,animation}`. Likely reuse for S1/S4/S5/S6: `Button` + `buttonVariants` (`default`/`primary`, `outline`, `secondary`, `ghost`, `destructive`, `destructive-solid`, `success`, `link`, `neutral`); `Card` (+ Header/Title/Content/Footer); `Table` (+ Header/Body/Row/Cell); `Progress` (+ Track/Indicator/Label/Value) and `StackedProgress`; `Collapsible` / `Accordion` / `Section` (there is no `Disclosure` export — closest listing name is `CatalogEmptyDisclosureRow`); `Pill` / `PillDot` / `PillGroup` (there is no generic `Badge`; `LiveBadge` and `AvatarBadge` are the only Badge-named exports); `Alert`; `Empty`; `ListingRow`; `ListingToolbar`; `ListingPage`; `DataSurface`; `StatusCard`; `StatusLine`; `StatusDot`; `Eyebrow`; `MonoId`; `Time`; `Timeline` / `TimelineEvent`; `DetailInspector`; `PropertyRow`; `MetadataList`; `Dialog`; `RunCard` (from `exports/conversation.ts`, not listing). `PillTone` = `neutral | accent | success | warning | danger | info`.

- **COPY.md has no loop/task/status label map.** Locked product nouns: `task run`, `session` (not chat), `workspace`, `capability` (never recipe/workflow/procedure/playbook). UI labels are sentence case and match backend nouns. Empty formula: `No task runs yet. Publish a task or let a coordinator enqueue work for this workspace.` (`COPY.md:225-261,443-451,556-568`). Request/wait/decision words are owned by graph-eng DESIGN-NOTES, not COPY.md.

- **loops_v2 / tasks — reusable chrome vs anti-references:** Reuse — 44px `w2head` drill-in; `details/summary` = icon · title · one-line gist · chevron; Lucide only, one icon per concept; Rows|Cards toolbar order (search/filters → spacer → view pills); color = state; machine truth as micro mono ≤10.5px; no explainer cards; annotate every delta (`production` / `spec` / `authorized delta`). Anti-references — loops_v2 first editor (48px topbar, invariant chips, palette note cards) was rolled back (`DESIGN-LESSONS.md:16-27`); hand-rolled timelines vs `Timeline` (`L10`); loops_v2 Needs you is an MCP-pause “nothing to do yet” row with a bell section icon, not the graph-eng request card (`loop-run-detail.html:244-255`); inventories deep-link `#waiting / #quarantined / #attention / #retrying` and older wait rail (`loop-inventories.html:332-397`) — superseded on run-page by pending/escalated/near-expiry; loops_v2 inlines a ds-core `:root` instead of linking the files; task-detail uses a 1px focus ring and the plan still cites a 48px topbar (`TASK-DETAILS-REDESIGN-PLAN.md:100`) against the locked 44px head; tasks plan documents 10 competing status tone maps as the defect to not repeat.

## Relevant Sources

- `docs/design/opendesign/graph-eng/DESIGN-NOTES.md:11-50` — locked request map, decisions, wait-kind sentences, truthful fields.
- `docs/design/opendesign/graph-eng/DESIGN-NOTES.md:88-99` — bell danger re-ink + full-viewport / ≤1240 / pixel-true lab layout.
- `docs/design/opendesign/graph-eng/DESIGN-NOTES.md:101-138` — shared data story + chapter map + append-only CSS rule.
- `docs/design/opendesign/graph-eng/graph-eng.css:1-34` — authority chain, `--req-chip-h` / `--req-rail-w:320px` / `--req-win-w:1240px`.
- `docs/design/opendesign/graph-eng/graph-eng.css:36-50` — full-viewport `.shell` / `.main` / 44px `.w2head`.
- `docs/design/opendesign/graph-eng/graph-eng.css:91-246` — chapters 1–3: chips, cards, decision bar.
- `docs/design/opendesign/graph-eng/graph-eng.css:273-330` — chapter 5: parked rows + waits rail (zero stays `0`).
- `docs/design/opendesign/graph-eng/graph-eng.css:382-420` — chapter 6: 22px tone-ring timeline grid.
- `docs/design/opendesign/graph-eng/graph-eng.css:479-535` — chapter 7: strategy progress + `partial` chip.
- `docs/design/opendesign/graph-eng/graph-eng.css:591-598` — chapter 8: node-verb dialogs, absent ≠ disabled.
- `docs/design/opendesign/graph-eng/graph-eng.css:987-996` — chapter 11: do not restyle 1–10; editor full-bleed exception.
- `docs/design/opendesign/graph-eng/graph-eng.css:1403-1442` — chapter 12 bell re-ink + APPEND POINT.
- `docs/design/opendesign/graph-eng/graph-eng-needs-you-requests.html:12-31,72-74,107-187,241-292,623-651` — provenance, pending chip, decisions, legend, wait sentences.
- `docs/design/opendesign/graph-eng/graph-eng-timeline-rows.html:14-27` — timeline production seam + live-ring delta.
- `docs/design/opendesign/graph-eng/graph-eng-progress-strategies.html:14-27` — progress production seam + partial/canceled rules.
- `docs/design/opendesign/graph-eng/graph-eng-node-verbs.html:14-27` — amend/rerun verbs, ConfirmDialog anatomy.
- `docs/design/opendesign/graph-eng/index.html:20-117` — set hub convention.
- `docs/design/opendesign/design-system/GUIDE.md:3-52` — production > this folder; starting a prototype; hard rules; L-034 directives.
- `docs/design/opendesign/design-system/ds-core.css:27-59,81-130` — prototype token aliases, signal dots, button variants.
- `docs/design/opendesign/design-system/ds-shell.css:7-21,66-75` — glass waiver; 44px menubar; wallpaper teal `#225555` depth-only.
- `docs/design/opendesign/command-palette/DESIGN-NOTES.md:160-186,355-400` — shared status-tone dictionary; lab layout; CSS chaptering.
- `docs/design/opendesign/command-palette/index.html:12-18,115` — identical index pattern (set-file convention).
- `docs/design/opendesign/command-palette/command-palette.css:39-75,1311` — chapter 0 parity lane; append-only vs in-place edits.
- `docs/design/opendesign/_done/loops_v2/DESIGN-LESSONS.md:16-32,72-84,140-182` — production-first, badge budget, copy-canonical primitives.
- `docs/design/opendesign/_done/loops_v2/index.html:56-108` — finals × labs hub; Configure/runs-history archived.
- `docs/design/opendesign/_done/loops_v2/loop-run-detail.html:45-47,244-255` — 44px head reuse; pre-graph-eng Needs you anti-ref.
- `docs/design/opendesign/_done/loops_v2/loop-inventories.html:267-278,332-402,546-549` — Rows|Cards + superseded inventory filters.
- `docs/design/opendesign/_done/tasks/task-detail.html:34,56-58` — 1px focus-ring drift; 44px head (do not copy the ring).
- `docs/design/opendesign/_done/tasks/TASK-DETAILS-REDESIGN-PLAN.md:82-91,100` — one status pill; exception-based pills; 48px topbar is the plan’s stale chrome (superseded by 44px).
- `packages/ui/src/tokens.css:16-56` — canvas / fg / muted / accent (action) / signal hex.
- `packages/ui/src/index.ts:1-13` — barrel; names resolved via `primitives.ts` + `exports/*`.
- `packages/ui/src/primitives.ts:5-49,196-247` — Button, Card, Progress, Table, Collapsible, Accordion, Pill, Section, Empty.
- `packages/ui/src/components/button-variants.ts:7-24` — Button variant list.
- `packages/ui/src/exports/listing.ts:19-41,106-122` — ListingRow, ListingToolbar, DataSurface, StatusCard.
- `packages/ui/src/exports/viz.ts:21-37` — StackedProgress, Timeline.
- `packages/ui/src/exports/conversation.ts:12-16` — RunCard.
- `packages/ui/src/exports/foundation.ts:2-12,27-28` — MonoId, StatusDot, AvatarBadge.
- `packages/ui/src/lib/layout-widths.ts:1-8` — 320px inspector vs 468px default right rail.
- `DESIGN.md:511-550,562-635,1067-1108` — atmosphere, surface/signal tables, anti-patterns.
- `COPY.md:225-261,443-451,556-568` — runtime nouns + empty-state formula; no status map.

## Transferable Patterns

- **Request map → S4 run default + needs-you** because that board is the same surface as graph-eng S1/S4. Copy `.req-chip` / `.req-card` / `.park-row` / `.waits-rail` grammar, the wait sentences, and `approve / edit / reject / respond`. Do not restyle chapters 1–5; append if the set shares `graph-eng.css`.
- **320px rail + ≤1240 stage → S4 and S5** because `--req-rail-w` and `--req-win-w` are the production run-page column. S5 DAG in the run-page column stays ≤1240; only an editor-canvas board may go full-bleed (chapter 11 exception). Do not use `WIDTH_RIGHT_RAIL_DEFAULT` (468px) for the waits/about rail.
- **44px `w2head` + `data-od-id` + provenance comment → every new board** because graph-eng and command-palette already ship that set file. Replaces loops_v2 inlined `:root` and the tasks-plan 48px topbar.
- **Chaptered append-only CSS → loop-legibility stylesheet** because command-palette chapter 0 + graph-eng APPEND POINT is the incumbent way to extend without forking tokens. New rules go after the mark; parity fixes edit the owning chapter.
- **`Pill` + `ListingRow` + `ListingToolbar` + `DataSurface` + `Empty` → S1 tasks list and S6 runs roster** because GUIDE listing routes ship Rows|Cards and the barrel has no `Badge`. Status chips use `Pill` tones from the shared dictionary. `RunCard` is available if the roster is card-shaped; do not hand-roll a second card.
- **`Progress` / `StackedProgress` + chapter 7 segment states → S5** because fan-out progress already encodes clean/active/failed/parked/canceled/never/pending. Partial always says `partial`. Wide fan-outs aggregate; they never paint N rows.
- **`Timeline` / `.story-row` 22px ring → S5 story, not a new DAG row** because chapter 6 forbids a second row geometry. Live = ring pulse; historical = settled.
- **`Collapsible` / `Section` / `details` gist → S4 section stack** because `loop-section.tsx` (icon · eyebrow · gist · chevron) is the production seam cited on the needs-you board. Closed sections still inform.
- **`Button` variants → decision bars and roster actions** because graph-eng already maps Approve = primary/neutral, Reject = `destructive`, Edit/Respond = default/outline. Absent verbs stay absent.
- **Badge budget + exception-based pills → S1/S6** because DESIGN-LESSONS L5 and the tasks plan both forbid taxonomy chips and zero-count pills. Enums/types render as text or icon+text.
- **COPY empty formula → S1/S6 empty** because COPY.md’s only task-facing sentence is `No task runs yet. …`. Do not invent cute empties; loops_v2 “Nothing is waiting on you” is inventory-specific and must not be rewritten into a new dialect without slice-01 state authority.
- **Index hub pattern → loop-legibility `index.html`** because both incumbent sets already use the same 880px / group / `tag--final` card. New boards cross-link with the same entity ids as the graph-eng story (`compozy`, `release-train`, `run_7f3a`) unless slice 01 names a different canonical fixture.

## Risks / Mismatches

- **Inventing `--color-action` or a second signal map** would violate `tokens.css` + `DESIGN.md` signal tables. Action = `--color-accent` `#e8572a`. Command-palette already says attention/needs-you reuse the existing badge→tone dictionary (`DESIGN-NOTES.md:164-165`).
- **Painting pending as danger on S4/S6** would leak the ADR-006 bell re-ink. Chapter 12 is explicit: run-list indicator stays warning.
- **Reusing loops_v2 inventory filters (`waiting / quarantined / attention / retrying`) as S4 rail or S6 chips** would fork the authorized graph-eng rail (`pending / escalated / near-expiry`) and the older production `waits / attention / quarantined` in the same set.
- **Copying loops_v2 Needs you (MCP pause, “nothing to do yet”, bell section icon)** onto S4 would replace the locked ask/review request cards and wait sentences with a different story (`r-8f3a2b` / `software-delivery` vs `run_7f3a` / `release-train`).
- **Hand-rolling Badge or Disclosure** would violate reuse-before-create: those names are not on the `index.ts` barrel. Use `Pill` and `Collapsible`/`Section`.
- **Importing artboard CSS into production** is forbidden by both DESIGN-NOTES files. Boards are visual contracts.
- **Restyling graph-eng chapters 1–12** to “fit” loop-legibility would break the append-only rule and the “iterate, don’t regenerate” contract.
- **48px topbar / 1px focus ring / accent side-stripes / glass on content** — tasks plan 48px, task-detail 1px ring, DESIGN.md `no-side-stripe-accent`, and shell-only glass. Content stays flat on `--color-canvas` / `--color-canvas-soft`.
- **Using `WIDTH_RIGHT_RAIL_DEFAULT` (468px) for the waits rail** would miss the locked 320px production about/waits rail.
- **Putting S5 run-page DAG on editor full-bleed geometry (188px nodes, collapsed rails)** would apply the chapter 11 exception outside the editor. Run-page stages stay fluid ≤1240.
- **COPY.md cannot supply status words.** Filling S1/S6 labels from marketing copy would invent a map this slice does not have. Status words for requests are graph-eng; task/run list labels belong to slice 01 / production, not this file.
- **`ds-core` `--subtle`/`--faint` hex vs tokens.css oklch** — boards that paste ds-core verbatim will miss production text-ladder values unless they add a chapter-0 parity lane (command-palette pattern). GUIDE: production wins.
- **`RunCard` lives in `exports/conversation.ts`.** Treating it as a listing primitive without checking conversation semantics risks a chat-shaped card on a runs roster.

## Open Questions

- COPY.md has no loop/task/status label map. Which authority (slice 01, production formatters, or graph-eng request words) supplies S1 task-list and S6 runs-roster status strings?
- Does loop-legibility append chapters to `graph-eng.css` after the mark, or open a new set-scoped CSS file that must not restyle chapters 1–12?
- For S6, is the roster primitive `ListingRow`+`Table`, `RunCard`, or both (Rows|Cards)? `RunCard` is a conversation export, not a listing export.
- Do S6 filter chips follow graph-eng waits-rail (`pending / escalated / near-expiry`), loops_v2 inventories (`waiting / quarantined / attention / retrying`), or a third production run-status enum owned by slice 01?
- Should new boards add a command-palette-style chapter 0 that rebinds `--subtle`/`--faint` to tokens.css oklch, or keep linking ds-core aliases as graph-eng does?
- S5 “DAG + roster”: inherit run-page body-DAG (≤1240) or editor node card (188px, full-bleed)? Chapter 11 says those are different stages.
- Confirm no new `Badge`/`Disclosure` names: parent prompt listed them; the barrel has `Pill` and `Collapsible`/`Section`/`CatalogEmptyDisclosureRow` only.

## Evidence

- `/Users/pedronauck/Dev/compozy/compozy/docs/design/opendesign/graph-eng/DESIGN-NOTES.md`
- `/Users/pedronauck/Dev/compozy/compozy/docs/design/opendesign/graph-eng/graph-eng.css`
- `/Users/pedronauck/Dev/compozy/compozy/docs/design/opendesign/graph-eng/index.html`
- `/Users/pedronauck/Dev/compozy/compozy/docs/design/opendesign/graph-eng/graph-eng-needs-you-requests.html`
- `/Users/pedronauck/Dev/compozy/compozy/docs/design/opendesign/graph-eng/graph-eng-timeline-rows.html`
- `/Users/pedronauck/Dev/compozy/compozy/docs/design/opendesign/graph-eng/graph-eng-progress-strategies.html`
- `/Users/pedronauck/Dev/compozy/compozy/docs/design/opendesign/graph-eng/graph-eng-node-verbs.html`
- `/Users/pedronauck/Dev/compozy/compozy/docs/design/opendesign/design-system/GUIDE.md`
- `/Users/pedronauck/Dev/compozy/compozy/docs/design/opendesign/design-system/ds-core.css`
- `/Users/pedronauck/Dev/compozy/compozy/docs/design/opendesign/design-system/ds-shell.css`
- `/Users/pedronauck/Dev/compozy/compozy/docs/design/opendesign/command-palette/DESIGN-NOTES.md`
- `/Users/pedronauck/Dev/compozy/compozy/docs/design/opendesign/command-palette/index.html`
- `/Users/pedronauck/Dev/compozy/compozy/docs/design/opendesign/command-palette/command-palette.css`
- `/Users/pedronauck/Dev/compozy/compozy/docs/design/opendesign/_done/loops_v2/DESIGN-LESSONS.md`
- `/Users/pedronauck/Dev/compozy/compozy/docs/design/opendesign/_done/loops_v2/index.html`
- `/Users/pedronauck/Dev/compozy/compozy/docs/design/opendesign/_done/loops_v2/loop-run-detail.html`
- `/Users/pedronauck/Dev/compozy/compozy/docs/design/opendesign/_done/loops_v2/loop-inventories.html`
- `/Users/pedronauck/Dev/compozy/compozy/docs/design/opendesign/_done/tasks/task-detail.html`
- `/Users/pedronauck/Dev/compozy/compozy/docs/design/opendesign/_done/tasks/TASK-DETAILS-REDESIGN-PLAN.md`
- `/Users/pedronauck/Dev/compozy/compozy/packages/ui/src/tokens.css`
- `/Users/pedronauck/Dev/compozy/compozy/packages/ui/src/index.ts`
- `/Users/pedronauck/Dev/compozy/compozy/packages/ui/src/primitives.ts`
- `/Users/pedronauck/Dev/compozy/compozy/packages/ui/src/components/button-variants.ts`
- `/Users/pedronauck/Dev/compozy/compozy/packages/ui/src/exports/listing.ts`
- `/Users/pedronauck/Dev/compozy/compozy/packages/ui/src/exports/viz.ts`
- `/Users/pedronauck/Dev/compozy/compozy/packages/ui/src/exports/conversation.ts`
- `/Users/pedronauck/Dev/compozy/compozy/packages/ui/src/exports/foundation.ts`
- `/Users/pedronauck/Dev/compozy/compozy/packages/ui/src/lib/layout-widths.ts`
- `/Users/pedronauck/Dev/compozy/compozy/DESIGN.md`
- `/Users/pedronauck/Dev/compozy/compozy/COPY.md`
