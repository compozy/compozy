# Loops design pass — lessons learned

> Source material for future design directives. Each lesson records what went wrong (or right)
> during the loops `loop-node-lifecycle` design cycle (2026-08-01/02), the correction the user
> drove, and a **directive candidate** written as a checkable rule. Evidence lives in the git
> history of `docs/design/opendesign/loops/` and in this folder's final pages.
>
> Cycle summary: 4 review rounds — (1) operator-heavy labs rejected, (2) editor rejected for
> diverging from production `web/`, (3) timeline/icons/density rejected against canonical
> components, (4) approved direction consolidated. Every lesson below traces to one of them.

---

## A. Authority & process

### L1 — Production is the skeleton; prototypes that drift cause rollbacks

**What happened.** The first editor artboard was seeded from an archived prototype and invented
its own chrome (invariant chip strip, 48px topbar, palette note cards, save-draft button). The
real editor (`web/src/systems/loops/components/editor/`) had none of it. The user flagged the
drift as a rollback risk for shipped code.

**Lesson.** A prototype for an existing surface is a *transcription plus deltas*, never a
reinterpretation. Before designing, extract the production anatomy at transcription grade:
bar stack with heights, grid columns, component geometry (px), token names, icon names,
default states (e.g. validation dock **collapsed by default**), and what deliberately does
NOT exist (no invariant chips, no minimap, no arrowheads).

**Directive candidate.** Any prototype of an implemented route starts from a written anatomy
report of the production component tree (file:line cited). Every deviation is annotated in the
HTML as a comment naming its source: `production`, `spec <id>`, or `authorized delta (user)`.
Un-annotated divergence is a defect.

### L2 — Explore with parallel subagents before drawing

**What happened.** The reconciliation only became possible after three parallel deep-dives
(production editor, external UI reference, spec web scope) produced transcription-grade
reports. Designing before that produced two rejected rounds.

**Directive candidate.** For any surface with an implementation or a named reference, run the
exploration first and design second. The report must include exact values (px, tokens, class
names, lucide names), not impressions.

### L3 — The authority chain, explicit

**What happened.** Conflicts appeared between the spec ("do not design editor surfaces"), the
archived artboards, and the user's ask (a complete final editor). Resolution order that worked:

1. the user's explicit request,
2. production (`packages/ui` + `web/src`),
3. `design-system/` (ds-core tokens, GUIDE),
4. the active spec/TechSpec (truth for *data*, gates for *scope*),
5. archived `_done/` artboards (seed material only).

A spec's "out of scope" never overrides the user's need to *see* the final result — render it,
but mark it as a proposal (`new · lifecycle` tags), never as existing behavior.

### L4 — Ship final pages, not only labs

**What happened.** The first delivery was five variation/state-matrix files with no page
showing the product as it ships. The user could not evaluate the result.

**Directive candidate.** An active domain folder contains the final pages of the flow
(list/detail/editor/run as applicable), cross-linked into one coherent story (same entity and
run ids across pages), plus an `index.html` mapping **finals × labs**. Labs alone are an
incomplete delivery. *(Already a verified rule: "Prototype sets ship final pages".)*

---

## B. Visual language

### L5 — Color is state; density is the enemy; badges have a budget

**What happened.** Rounds 1–3 all flagged the same family: "carnaval de cor", "excesso de
badges", "separação visual densa". Fixes that worked:

- Enumerations (terminal outcomes, start kinds) render as **plain mono text**, not chip rows.
- Type/category markers (criterion types, node classes) render as **icon + text**, not badge boxes.
- Zero-count chips do not render at all — absence is the signal.
- Tone (warning/danger/info/success) appears only where the daemon reports that state.
- One accent per screen: primary CTA + live pulse. Never card/panel borders.

**Directive candidate.** Badge budget per panel: state pills only. Anything constant
(types, enums, defaults) is text or icon+text. A zero count renders nothing.

### L6 — Visual richness comes from structure, not decoration

**What happened.** "Node cards are not visual" was fixed without adding color, by adopting
reference patterns (Sim Studio): a neutral 24px icon tile whose **icon carries identity**,
label→value body rows summarizing real config, tab-shaped connection handles that grow on
hover, a 1.75px state ring as the only colored signal, and surface-step elevation instead of
shadows.

**Directive candidate.** To make a component "more visual", add structure (icon tile, kv rows,
real data summaries, micro-interactions) before adding any hue. Color remains state-only.

### L7 — Icons are wayfinding, and they come from Lucide

**What happened.** Pages with no icons read as walls of text; hand-drawn inline SVGs read as
inconsistent. The fix: Lucide via CDN in every prototype —

```html
<i data-lucide="shield-check"></i>
<script src="https://unpkg.com/lucide@latest"></script>
<script>lucide.createIcons();</script>
```

with icons on section headers, node kinds, criterion types, run origins, and menu items.
Production parity holds: the run-story icon map is lucide (`play/check/x/bell/pause/eye`), and
new event kinds get lucide names too (`rotate-ccw`, `zap-off`, `copy-x`, `radio`, `send`).

**Directive candidate.** Prototypes use Lucide (current canonical names) exclusively; size via
container CSS; each recurring concept keeps one fixed icon across all pages of the domain.

### L8 — Collapse is the density control; the summary carries the gist

**What happened.** "Tudo expandido sem opção de colapsar" — especially the detail sidebar.
The fix: every section is a `details/summary` with a fixed anatomy — icon + title + one-line
summary + chevron — and deliberate defaults: the primary section open, secondary rail sections
closed. A closed section must still inform (`Limits · 50 generations · no budgets set`).

**Directive candidate.** Sidebar/rail sections are collapsible cards whose summary line states
the takeaway; at most one rail section opens by default. Long histories fold behind a count
(`Generation 1 · 12 events`).

### L9 — Demote operator data, never delete it

**What happened.** Making pages "user friendly" must not break SD-007 truthfulness. The working
pattern: plain-language titles and sentences up front; machine truth demoted to micro
typography — the event kind in 9px mono on each timeline row, `digest` in the rail foot,
lint codes inside the validation dock, Inspect behind a quiet button.

**Directive candidate.** Every plain-language row keeps its machine identity visible at micro
scale (mono, ≤10.5px, faint). Removing the machine truth entirely is as wrong as leading with it.

---

## C. Components

### L10 — Reuse canonical components exactly; never hand-roll a parallel version

**What happened.** The run-page timeline was hand-rolled and immediately flagged: "nada a ver
com o `packages/ui/src/components/custom/timeline.tsx` que temos". The canonical anatomy
(hairline spine at 8–10.5px, 22px tone-ring dot with an 11px lucide icon, `22px | 1fr | auto`
grid, relative time + micro mono trail, Pill xs metadata) had to be transcribed, not
approximated.

**Directive candidate.** Before building any list/timeline/selector/meter in a prototype, read
the exported primitive (or its closest production consumer) and copy its geometry. This
generalizes the existing runtime-selector and workspace-select rules to every primitive.

### L11 — Product UI is not documentation

**What happened.** The palette carried three explainer note cards ("Kinds are ToolIDs…",
"Fork & edit only…"). The user called them "totalmente desnecessários". Explanations belong in
hints/tooltips at the point of use, docs pages, or nowhere.

**Directive candidate.** No explainer boxes inside product chrome. A concept that needs a
paragraph gets a `title` tooltip, an inline field hint, or a docs link — never a card.

### L12 — Interactive states are part of the design, and they are gated by daemon truth

**What happened (carried through all rounds, verified rule).** Controls render only for states
the payload declares: Pause only on `running`, Kill as the destructive escape in overflow,
`stop` deleted with the contract. Deterministic answers render as information, not errors.
Demos stay functional (fixing `max_fan_out` clears the dock, the node badge, the DSL highlight,
and enables Publish) so reviewers can *feel* the contract.

**Directive candidate.** Every prototype control cites (in an HTML comment) the payload field
or route that gates it; interactive demos exercise the real state machine, not fake toggles.

---

## D. Working agreements distilled (candidate directive list)

1. Transcribe production first; annotate every delta with its authority.
2. Parallel deep-dive reports (with exact values) precede any redesign of an implemented surface.
3. Final pages + labs + index per domain; one coherent cross-linked story.
4. Color = state. Badge budget. Zero counts render nothing.
5. Richness = structure (icon tiles, kv rows, handles, rings), never hue.
6. Lucide only, one icon per concept, sized by container.
7. Every section collapsible; summaries carry the takeaway; calm defaults.
8. Machine truth demoted to micro mono, never removed.
9. Canonical primitives are copied, not reinvented.
10. No explainer cards in product chrome.
11. Controls gated by daemon truth, cited in comments; demos exercise real transitions.
12. Specs gate implementation scope, not what the user may preview — mark proposals as proposals.
