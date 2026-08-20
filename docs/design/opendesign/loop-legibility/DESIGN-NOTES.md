# Loop & task legibility — visual contract

Design contract for the six boards in this folder. Companions:
`.compozy/tasks/loop-task-legibility/_uiux.md` (surface map),
`_user_stories.md` (ACs/ECs), `_dx.md` (canonical transcripts).
This file is the locked semantic contract — every chip, briefing
sentence, and decision button traces back here.

Exploration: `analysis/summary.md`.

## Locked decisions

### Two registers, no toggle (ADR-002)

One run page. Default register answers, in order: what is running,
what needs you, how far along, what came out. Spend (tokens · cost ·
budget · rounds · duration) lives on the **Usage rail only** — it is
not a fifth main-column section. Operator register (live DAG, complete
node roster, generation history, raw events, ids) sits one disclosure
deeper behind **Inspect**. Failure and needs-you never collapse.

**One decision surface (remediation 2026-08-19):** the needs-you card
owns Approve/Reject — the only primary on the page. The briefing
strip's inline unblock action is a quiet **Review the request**
that leads to the card; it never duplicates the decision buttons.
Two primaries for the same decision in one viewport is a defect.

### Where the live DAG lives (ADR-003, made concrete)

The graph is not a screen and not a floating component. **Inspect
opens the operator register, and the register is four lanes over one
read model: Graph · Nodes · Generations · Events.** Graph is the
default lane. It renders in the run page's main column, at the
column's width, inside `.panelbox.ll-insp` — never the editor's
full-bleed exception, never a route change.

While Inspect is open the default read stays above it. The briefing
and the needs-you card never collapse; Progress and Story fold to
their gists, and a folded gist still carries the takeaway.

**What "live" means, exactly.** Node states reconcile from the
roster read; the stream only accelerates them. So an update flips
node state, lights the edge that carried data, moves the pulse to
whatever is running now, and re-centres the foot — without
re-mounting the lane or losing scroll position. When the stream
drops, the live badge leaves the disclosure head, a `reconnecting`
chip takes its place, and the lane keeps rendering the last
reconciled read with its age stated. Stale state is never painted
as live, and the graph is never blanked.

A real topology is wider than the column. The graph lane scrolls
horizontally with a faded right edge, and the foot names what sits
past the fold.

### Production metrics this set is pinned to

Measured from `loop-run-page-body.tsx` and its rails. These are not
preferences — a prototype that rounds them looks off.

| Metric | Value | Source |
| --- | --- | --- |
| page grid gap | 32px | `gap-8` |
| rail column | 320px | grid template |
| main-column section rhythm | 26px | `gap-6.5` |
| section header | 24px min, 8px icon gap, 10px bottom pad | `min-h-6 gap-2 pb-2.5` |
| rail section padding | 16px / 18px | `py-4 px-4.5` |
| gap between rail sections | 0 — one card, 1px `line-soft` dividers | — |
| rail footer | 10px / 12px, Inspect left, digest right | `py-2.5 px-3` |
| property row | 26px min | `--height-property-row` |
| progress panel body | 16 / 18 / 17px | `pt-4 px-4.5 pb-4.25` |
| story row | 22px medallion, 1.5px ring, 11px glyph, 9px row pad | `size-5.5`, `py-2.25` |

**Icon ladder.** 14px is the run page's default — section headers,
chevrons, card lead glyphs, button glyphs. 12px is the rail and
inline-link size — rail section headers, `arrow-up-right` on "open
elsewhere" links, pill glyphs, lane-tab glyphs. 11px is the story
medallion glyph, the only 11px in the system. **Nothing on the run
page renders larger than 16px.** Rail header glyphs are `--muted`;
section header glyphs are `--subtle`; chevrons are `--faint`.

Rail header icons, exactly: **Coins** on Usage, **Info** on About.
Section header icons: **Bell** Needs you, **Gauge** Progress,
**History** Story, **Search** Inspect.

**Button ladder.** `.btn--sm` (22px) is the workhorse and the only
size on the run page — decision bars included. 24px (`min-height`)
is reserved for the five dense inline call sites production marks
that way; the rail-footer Inspect is one. 26px `.btn` survives only
on page-level empty-state actions. Nothing larger appears.

### Needs-you tone (graph-eng lock, confirmed)

Pending human request on the **page** = **warning**. Danger is reserved
for failed / quarantined chips and the **bell** re-ink (ADR-006). Do
not leak bell danger onto S4 cards or S6 roster chips.

- pending = warning + `triangle-alert` + `pending`
- near-expiry = warning + `clock` + shared mono countdown
- expired = danger + `circle-alert` + `expired`
- answered / amended = info + `check` / `pencil` + the word
- partial = warning + the literal word `partial` + coverage numbers
- canceled / not-taken / never-materialized = neutral
- fork lineage = info + `git-fork` + `fork`

Color never travels alone: tone + glyph + literal state word.

**Decisions (exact):** `approve / edit / reject / respond`. Buttons
render only the persisted set. An absent decision is omitted, never
disabled. `requeue` is a node verb on the operator panel, not a
needs-you decision.

**Wait-kind sentences (exact):** ask `"{label} is waiting for an answer"`;
review `"{label} is waiting for a decision on its proposed action"`.

Gate `aplicar-correcoes` in the canonical story is a review-shaped
approval: persisted set is `approve` + `reject`.

### Structure vs status

Node *kind* (agent / gate / collect / route / fan-out) = glyph +
neutral border from `loop-node-kind-icons.ts`. *Status* = signal
`Pill` + icon + text. The two never share a channel. Kind inventory
is additive; boards may stage a subset as story.

### Pending ≠ not-taken (SI-14)

- `pending` — reachable, upstream unsettled: neutral, hollow/outline
  glyph, literal word `pending`.
- `not_taken` — durable route evidence: neutral-dim + `git-branch` /
  `minus` / `circle-dashed`, literal word. Taken path lights.

### Attempts, fan-out, progress

Attempts are metadata (`attempt 2`) on the step, never sibling steps.
Recovered nodes read by current state. Progress = settled action steps
out of total action steps in the current round; hide the round counter
on round 1 / single-pass. Route-not-taken and never-materialized leave
the denominator.

**Parallelism grammar (consolidation 2026-08-20).** Fan-out workers
never become sibling graph entities (US-011.EC-1) — but width, fate,
and convergence must be drawn, not implied by a fraction. Both
drawings speak the shared `.prog__seg` segment vocabulary
(`clean / active / parked / failed / canceled / never / pending`);
neither invents a second bar primitive.

- **S4 progress** = one `.prog` panel: goal line, segmented
  `.prog__bar`, left/right meta, then the step list as graph-eng
  `.strat-row`s. A fan-out is ONE step row whose middle cell carries
  a `.ll-band` — a hanging left band with a `.ll-lanes` strip and
  named workers up to six, a rollup sentence beyond.
- **S5 DAG** = `.ll-fan`: one bordered band per fan-out entity, sitting
  in the `.ed-dag__row` beside ordinary `.ed-dag__card` nodes.
  Branches render *inside* its boundary — named `.ll-branch` rows up
  to four, `.ll-lanes` + rollup text beyond (`.ll-fan--wide`). The
  join node wears `git-merge`.
- **Filtered fan-out**: skipped branches go `never` (neutral tint +
  inset ring) and leave the denominator — absence stays calm.
- **Drained descendants**: leftover branches settle as `canceled`
  lanes with the cause in text — neutral, never an alarm.

### Two channels in the progress panel

The segmented `.prog__bar` is magnitude, not signal: it says how
much of the round has settled. Its `parked` segment keeps the
graph-eng info ink — the bar never shouts. The alarm-bearing state
lives one line below, on the step row's `.pill`, where tone, glyph
and the literal word travel together. Colouring the segment warning
as well would be signal overload for one fact.

### Fixes to the canon, recorded here

- `ds-core.css` `.meter` declares no `display`, so a `<span class="meter">`
  collapses to inline and loses its 6px height. This set declares
  `display:block` for its own callers; the root fix belongs in ds-core.
- `ds-core.css` `.btn` paints no `[aria-pressed]` state, so a lone
  pressed button is invisible. Every toggle in this set is a
  `.pill-group` with both states named instead.
- `ds-shell.css` sizes `.w2-glyph svg` and `.w2-back svg` but not
  `.w2-status svg`, so the window-head status glyph rendered at
  Lucide's intrinsic 24px. Chapter 13b sizes every container this
  set puts an icon in; no board carries an inline icon size.
- `ds-core.css` `.empty > svg` sets the GLYPH to 38px inside 11px of
  padding — a 60px block. Production's `Empty` is a 38px well with a
  16px glyph. Corrected locally; the root fix belongs in ds-core.
- graph-eng's `.run-page` was authored at 20px grid gap / 14px
  section rhythm. Production ships 32 / 26. This set overrides both
  and uses its own `.win--run` rather than restyling `.win--req`.
  Consequence: on the identical production surface this set now runs
  12px looser than the graph-eng boards. Production wins; the graph-eng
  numbers are the drift, and they should be corrected there.
- `ds-core.css` pairs `.btn--icon{width:26px}` with `.btn--sm{height:22px}`
  and never reconciles them, so a small icon button is a 26×22
  rectangle. Squared locally.
- `ds-core.css` defines `.lane-tab` with no `svg` rule, so a lane-tab
  glyph would fall back to 24px. Sized locally, unscoped.

### Two rhythms, both sourced

Top-level spacing is 26px on the run pages (`gap-6.5`, from
`loop-run-page-body.tsx`) and 14px between listing groups in a window
(`.app{gap:14px}`, ds-core). They are different surfaces with
different owners, not an inconsistency — but every future change to
either must cite its source rather than splitting the difference.

### Where an icon belongs, and where it does not

Icons are wayfinding, not decoration. This set puts one on: rail
section headings (Coins / Info), main-column section headings
(Bell / Gauge / History / Search), node **kind** — on the DAG card,
in the roster's sub-line and on the node panel head, always a 12px
`--muted` glyph that never carries status — lane tabs, buttons whose
verb has a canonical glyph, and "opens elsewhere" links
(`arrow-up-right`, 12px).

It deliberately puts none on **group headings** (Needs you · Active ·
Recent · In progress · Ready). Production's runs-table groups are a
bare `Eyebrow` + count, and an icon beside every heading is the
decoration the charter bans. An earlier pass shipped CSS hooks for
those glyphs before deciding this; the hooks were deleted rather
than filled.

### Canceled is calm

Strategy-canceled and operator-canceled use the neutral ramp. Cause
and actor stay in text. Never alarm color.

### Vocabulary

| Use | Do not use |
| --- | --- |
| **step** (plain register) | loop cell as primary copy |
| **Loop run** (grouped label) | Loop coordinator as primary text |
| meaning titles | title-cased enums, `node_failed g2` |
| Inspect (one disclosure) | simple / advanced toggle |
| `no loop records in this workspace` | generic empty when reveal is on |
| `run no longer available` | broken link |
| `content no longer stored` / `session no longer available` | hide the row |

S1 reveal filter is quiet, hidden by default, **off on every
navigation**. Not a `config.toml` key.

### Signal palette (tokens.css, no `--color-action`)

Action / live running = `--color-accent` `#e8572a`. Success
`#5fbf85`. Warning `#d6a647`. Danger `#e0635a`. Info `#8e8eb5`.
Prototype boards speak `ds-core` aliases and rebind nothing — a set
file must never redefine a ramp token.

**Known canon drift (reported, not patched here):**
`packages/ui/src/tokens.css` ships `--color-subtle` /
`--color-faint` as `oklch(0.663 0.009 286.106)` /
`oklch(0.638 0.006 286.136)`, while `ds-core.css` still mirrors the
older hex pair `#76767c` / `#545458`. The earlier revision of this
set patched the two tokens locally, which silently re-inked every
graph-eng component on these pages and made the set look unlike its
siblings. The fix belongs in `ds-core.css`, applied once for every
set; until then these boards inherit ds-core like everyone else.

Reduced motion **unmounts** the live edge's travelling dot
(`display:none`) and stills the running dot and story ring; it never
leaves a frozen animation mid-frame.

### Lab layout

Full-viewport labs. Staged fragments at production content width
(fluid ≤1240). Rail **320px** pixel-true. DAG stays in the run-page
column — not the editor full-bleed exception. Window head is 44px.

### CSS

`loop-legibility.css`. Link order: `ds-core.css` → `ds-shell.css`
→ `../graph-eng/graph-eng.css` → this file. It carries the domain
layer only — run-state variants on the read-only DAG, the fan-out
band, the two roster tables, the kanban frame, and the lab widths.
Everything else composes from the canon. Boards contain no `<style>`
block and no inline `style` attribute except a bar's own data value.
Artboard CSS is a contract — never imported into production.

### Primitives — reuse before create

Compose from `@compozy/ui`: `Pill` (not Badge), `Section` /
`Collapsible` (not Disclosure), `ListingRow`, `ListingToolbar`,
`Table`, `Progress` / `StackedProgress`, `Empty`, `Button`, `Card`,
`PropertyRow`, `Alert`, `PillGroup`, `Metric`. The DAG canvas is
the only domain-specific renderer.

The prototype mirrors that inventory 1:1. Board class → canon:

| Board element | Class used | Owner |
| --- | --- | --- |
| briefing strip | `.notice` (+ `--calm` / `--lead`) | ds-core |
| state chip | `.pill[data-tone]` | ds-core |
| pending / not-taken chip | `.pill[data-form="hollow"|"absent"]` | this set |
| decision card | `.req-stack` / `.req-card` / `.req-bar` | graph-eng 2–3 |
| request vocabulary legend | `.req-legend` | graph-eng 1 |
| progress panel | `.prog` / `.prog__bar` / `.prog__seg` | graph-eng 7 |
| step rows | `.strat-list` / `.strat-row` | graph-eng 7 |
| story | `.story-stack` / `.story-row` / `.story-ring` | graph-eng 6 |
| “load more” footer | `.park-foot` | graph-eng 5 |
| generation entries | `.gen-list` / `.gen-row` | graph-eng 10 |
| disclosure | `.lsec` | graph-eng 2 |
| page grid + rail | `.run-page` / `.run-cols` / `.railbox` | graph-eng 2 · ds-core |
| rail sections | `.rail-sec` / `.prow` / `.meter` | ds-core |
| task + artifact rows | `.list-shell` / `.listing-row` | ds-core |
| window tool strip | `.win-toolbar` | ds-shell |
| toolbar + toggles | `.listing-toolbar` / `.field` / `.pill-group` | ds-core |
| empty states | `.empty` | ds-core |
| node panel | `.panel` / `.panel-h` | ds-core |
| DAG shell + node card | `.ed-dag` / `.ed-dag__card` | graph-eng 11 |
| skeleton | `.sk` | ds-core |

**Retired vocabulary (consolidation 2026-08-20).** The first
revision shipped 30 bespoke families; 28 duplicated something the
canon already defined. Deleted, with the reason:

`.chip-st` (third chip vocabulary) · `.beat` / `.story` (second
timeline geometry) · `.flow` / `.fnode` (duplicate of `.strat-row`,
misaligned by 0.5px against the story spine) · `.lanes` / `.lane` /
`.lane-key` (verbatim copy of `.prog__seg`) · `.usage-*` /
`.about-*` / `.rail-foot` (third key/value grammar) · `.usage-bar`
and `.gantt` (fourth and fifth bar primitives; `.gantt` also
hardcoded a 55% data value in CSS) · `.trow` / `.tgroup` (bespoke
row and group head) · `.tb` (bespoke toolbar with an unfocusable
input) · `.reveal` and `.rfilter` (toggles that painted no pressed
state) · `.insp` (fifth disclosure) · `.ll-empty` (fourth empty) ·
`.npane` (bespoke panel) · `.brief` (tone-coloured panel border,
which breaks the graph-eng anti-wash lock) · `.arti` · `.fan` ·
`.att` (which also collided with the ds-core `.att` attention block
and turned inline text into a column flex container) · `.dnode` /
`.dfan` / `.dbranch` / `.dedge` (third node-card system) ·
`--ll-win-w` (duplicate of `--req-win-w`) · the `:root` rebind of
`--subtle` / `--faint`.

Surviving domain families, all namespaced: `.ll-node` · `.ll-fan` ·
`.ll-branch` · `.ll-edge` · `.ll-dag` · `.ll-band` · `.ll-lanes` ·
`.ll-att` · `.ll-table` · `.ll-kanban` · `.ll-grp` · `.ll-sk` ·
`.ll-npanel`.

### No seventh board

S2 dashboard/inbox, S3 task-detail provenance, S7 node inventory:
no dedicated HTML. S3 reuses S1 revealed-row grammar.

## Canonical data story

Exactly two loops, two run ids. Do not mint a third.

| Entity | Facts |
| --- | --- |
| Loop `revisao-paralela` | `implementar` → fan-out `revisores` ×3 (`revisor-seguranca`, `revisor-perf`, `revisor-estilo`) → `sintetizador` → `saida`. Gate `aplicar-correcoes`. Input `tema="rate limiting"`. |
| Loop `fabrica-assistida` | Running, step 2/9, started 18:41, duration 13m. **No run id, no topology.** |
| Run `looprun-8f3ab2c1d4e5f607` | Live. Needs-you beat: round 1, step 4/6, approval waiting 3m. Usage rail: 82.4k · $0.31 · 12% · 9m40s. S6 row duration: 22m. |
| Run `looprun-77aa01b2c3d4e5f6` | Done after 2 rounds. 214.5k · $0.87 · 38% · 18m12s. Artifact `post-final.md` via `saida`. |
| Work items | `tsk-92ab41` Escrever post sobre memoria (in progress); `tsk-88cf02` Revisar landing page (ready). |

Healthy briefing beat (earlier clock): implementar running, “Nothing
needs you. Step 1 of 6 is running.” (Copy fixed in remediation — a
running step 1 cannot have “1 of 6 done”.) Needs-you beat owns the
later clock.

## Staging that is not a VC

- S4 reduced-motion: DESIGN-NOTES only; staged lab is `task_05/VC-24` (DAG pulse unmounts).
- Budget exhausted: extra lab panel on the default board, labeled design-lab.
- US-014 concurrent / stale-view: implementation toast — not staged.

## Chapter map (`loop-legibility.css`)

0 local tokens · 13 hollow/absent chip forms · 13b icon sizing for
containers the canon leaves unsized · 14 briefing strip (`.notice`
extensions) · 15 progress state inks + fan-out band · 16 read-only
DAG (state variants, fan band, edges) + node panel · 16b operator
register shell (lane tabs, graph lane, live foot) · 17 roster
tables · 17b group heading + degraded block · 18 kanban frame ·
19 lab fit + the `.win--run` window.

Consolidation 2026-08-20 rewrote the file against the canon rather
than extending it: 430 lines and 30 bespoke families became 362
lines and 13 namespaced ones, and the boards lost every inline
`style` except bar data values. The append point sits at the end of
chapter 19 for later runs.

## Board budget

| Board | Sections | Note |
| --- | --- | --- |
| `…-tasks-list.html` | 3 | default · reveal + edges · kanban |
| `…-run-default.html` | 5 | default read · healthy · briefing gallery · story · Inspect |
| `…-needs-you.html` | 5 | card anatomy · multiple · expiry · resolved · vocabulary |
| `…-run-dag.html` | 3 | **the register in place (live · updated · degraded)** · node states · node panel |
| `…-run-roster.html` | 3 | roster · states · generations |
| `…-runs-roster.html` | 2 | needs-you first · roster states |

Nine terminal briefing variants stage as one gallery inside a single
section rather than nine lab sections — the composition is identical
in each, so repeating the scaffold nine times was noise, not evidence.
