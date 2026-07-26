# Analysis: modal-reference-system

> **Living authority (post-research):** `modals/MODAL-STANDARD.md`, `modals/` 16 surfaces, and `design-system/patterns.html` § Modals. The former `systems/design-system.html` was deleted 2026-07-22 (absorbed into `design-system/`). Keep this file as a research snapshot; do not treat deleted paths as open links.

Read-only exploration of the slice `modal-reference-system` (ordinal `03`) for the research prompt:

> Nós temos alguns modais aqui que foram criados pra melhorar os modais de criação e edição das entidades... Pra task, trigger e job, nós criamos modais mais bonitos, o cabeçalho e tudo mais, o conteúdo, o formulário. E agora a gente precisa alinhar esse nível, esse padrão de modal pra criação e edição de itens pra outras páginas... Se você vê que um modal está muito complexo, muito denso, crie uma versão simplificada, melhore o I/O/X dele... e vamos tentar seguir ao máximo os padrões. References: create-task-redesign.html, create-job-redesign.html, create-trigger-redesign.html.

## Scope

- Slice question: Extract the reusable modal design system from the task/trigger/job references — anatomy, sizing, header/footer contracts, icon treatment, simple/advanced disclosure, live summary/preview, responsive transforms, a11y/interaction states — compare against exported AGH primitives + tokens, and define a transferable modal matrix for create/edit flows without inventing backend behavior.
- Primary sources: `docs/design/opendesign/modals/{create-task,create-trigger,create-job}-redesign.html`, then-`docs/design/opendesign/systems/design-system.html` (since deleted → `design-system/`), `packages/ui/src/tokens.css`, `packages/ui/src/index.ts`, and dialog/form/tabs/select/switch/field components under `packages/ui/src/components/`.
- Sources read in full vs. sampled: **Full** — all three reference modals (task 686 ln, trigger 1035 ln, job 754 ln markup + sampled JS), `tokens.css` (477 ln), `index.ts` (663 ln), `dialog.tsx`, `field.tsx`, `switch.tsx`, `tabs.tsx`, `form-section.tsx`, `editor-footer.tsx`, `detail-header.tsx`, `radio-card.tsx`, `confirm-dialog.tsx`, `pill-group.tsx`. **Sampled (grep-targeted)** — then-`design-system.html` (1470 ln: Principles/Tokens/Class-contract/Checklist/Button spec regions; now `design-system/{foundations,components,patterns}.html`), trigger/job preview JS.
- Total candidate sources surveyed: ~20 files; component directory of 48 primitives enumerated.

## Overview

The three references are **not three separate designs** — they are one modal design system rendered three times. All three inline the identical `:root` token block (warm-dark ramp, single operator-orange accent, flat depth) copied verbatim from `packages/ui/src/tokens.css`, and all three share pixel-identical shell CSS: the same `.scrim`, `.modal`, `.modal__head`, `.modal__foot`, `.field`, `.pill`, `.switch`, `.coll`, and `.note` rules. The system is therefore already extractable — the variation between the three is *body layout and disclosure strategy*, not chrome. This slice's job is to name the invariant shell contract and the two body archetypes, then map each pattern onto what `@agh/ui` already exports.

Two body archetypes emerge. **(A) Single-column form** (task): a `grid-template-rows: auto auto 1fr auto` shell with a pinned **Simple | Advanced** mode toolbar between header and body, numbered `.sec` sections, and progressive disclosure via `#adv-groups` + `<details>` collapsibles. **(B) Two-pane form + live preview** (trigger, job): a `grid-template-rows: auto 1fr auto` shell whose body is `grid-template-columns: 1fr 468px`, a stepped `.flow` "spine" on the left (For → When → If → Then / For → Run → When), and read-back `.pv-card`s on the right (summary sentence, sample-event JSON with match badge, rendered prompt, webhook endpoint / next-runs / request payload). Archetype A is the answer to the operator's "muito denso → versão simplificada" directive; archetype B is the answer to "melhore o I/O/X" (input on the left, output previewed on the right, in real time).

The operator will most likely act on: (1) a **shared modal shell contract** (header/footer/scrim/responsive) that every entity modal conforms to, ideally promoted to a `@agh/ui` composite since the three references duplicate ~250 lines of identical shell CSS; (2) the **Simple/Advanced disclosure machinery** as the standard density-reduction lever; (3) a **primitive mapping table** so new modals compose `@agh/ui` (Dialog, PillGroup, Switch, Field, FormSection, Stepper, RadioCard, EditorFooter) instead of re-hand-rolling controls; and (4) resolution of the handful of drifts where the references diverge from the shipped primitives/tokens (accent-tint choice cards vs `RadioCard`, modal `radius-xl` vs `rounded-lg`, scrim alpha/blur).

This slice overlaps adjacent slices at the seam of *which entities* need modals and *what backend fields* they carry (out of scope here — I extract the reusable form/preview grammar, not the field lists). It does not resolve edit-mode anatomy: **all three references are create-only**; edit deltas are an inference from the create shell, flagged in Open Questions.

## Mechanisms / Patterns

- **Shared shell contract (invariant across all 3):** `.scrim` (fixed inset-0, `var(--scrim)`, `backdrop-filter: blur(2px)`, `display:grid; place-items:center; padding:28px`) hosts a `.modal` (`background:var(--surface)`, `border:1px solid var(--line)`, `border-radius:var(--radius-xl)` = 14px, `box-shadow:var(--shadow-overlay)`, `overflow:hidden`, CSS-grid rows). `role="dialog"` + `aria-label` on `.modal`. A dimmed `.backdrop` renders the product context behind. Source: `task…:66-72`, `trigger…:62-68`, `job…:63-69`.
- **Header contract (`.modal__head`, identical geometry all 3):** `flex; align-items:flex-start; gap:16px; padding:20px 24px 18px; border-bottom:1px solid var(--line-soft)`. Three slots: **icon well** (`.head-icon` 34×34, `radius-lg`, `background:var(--accent-tint)`, `color:var(--accent-strong)`, `box-shadow:inset 0 0 0 1px var(--accent-dim)`, 18px glyph) → **text block** (`.eyebrow` 11/600 uppercase accent-strong + `.modal__title` 16/600 fg-strong + `.modal__sub` 12.5 muted, max 62–64ch) → **close** (`.head-close` 30×30, `radius`, line border, hover→`--hover`). Source: `task…:73-84`, `job…:70-81`.
- **Footer contract (`.modal__foot`, identical all 3):** `flex; align-items:center; gap:12px; padding:14px 24px; border-top:1px solid var(--line-soft); background:var(--surface)`. Slots: `.foot-note` (flex:1 info line, 11.5 subtle, leading `ⓘ` icon) → `.btn--outline` Cancel → `.btn--primary` CTA (30px = `button-lg`, check glyph, state-reactive label). Source: `task…:213-223,473-483`, `trigger…:231-241,567-578`.
- **Simple/Advanced disclosure (task, the density lever):** a pinned `.toolbar` PillGroup (`#mode-seg`, `aria-pressed`) drives `state.mode`. Advanced un-hides `#adv-groups` (Placement, Queue & ownership, Ingress & identity, Execution) and reveals numbered `[data-adv]` ordinals; the template grid re-renders `cols3→cols2` with a different label/description set. Dropping to Simple while on an advanced-only template snaps back to `one_shot`. Source: `task…:265-301,619-653`.
- **Stepped `.flow` spine (trigger/job):** a `padding-left:42px` column with a gradient connector line (`.flow::before`) and numbered `.node` circles (29px, active = `accent-tint` fill + `accent-dim` ring). Each `.step` = kicker + title + sub + control. Nodes toggle `data-on="1"` as the step becomes satisfied (e.g. `#node-if` lights only when filters exist). Source: `trigger…:108-120,342-476`, `job…:115-128,387-641`.
- **Live preview / read-back (trigger/job):** right `.preview` pane (`background:var(--canvas)`, `border-left`) stacks `.pv-card`s. **Summary sentence** compiles a plain-language read-back ("When `session.stopped` happens in **checkout-api**, if **stop_reason** = **error**, run *summarizer*"). **Sample-event JSON** highlights the filtered keys and shows a match badge (`fires on every event` / `matches this sample` / `won't fire`). **Rendered prompt** interpolates Go `text/template` vars against the sample, painting misses `--danger`. Job swaps in **Next runs** (computed from cron) + **Request payload** (`POST /api/automation/jobs`). Source: `trigger…:525-563,910-969`, `job…:692-733`.
- **Choice cards (all 3, contested selected-state):** `.tpl` (task templates), `.seg` (job output mode), `.event` (trigger catalog). Resting = `surface-2` + `line-soft`; **selected = `background:var(--accent-tint); border-color:transparent; box-shadow:inset 0 0 0 1px var(--accent-dim)`** + accent-strong title + fade-in check. Source: `task…:134-154`, `job…:150-165`, `trigger…:142-158`.
- **Inline conditional sub-config (trigger):** `.subcfg` blocks are re-parented as live DOM directly beneath the selected event card, tied by an accent left-rail (`border-left:2px solid var(--accent-dim)`). Keeps listeners/focus across re-parenting. Source: `trigger…:160-167,743-758`.
- **Collapsible advanced groups (`.coll` = native `<details>`):** chevron + label + right-aligned state badge summary (Execution / Reliability & state / Reliability & limits). Job opens it by default (`<details open>`). Source: `task…:438-464`, `trigger…:479-521`, `job…:646-688`.
- **Switch row (`.switch-row`):** 32×18 track (matches `--width/height-switch-default`) + label/desc block; `role="switch"` `aria-checked`. Source: `task…:181-190,446-462`.
- **Field primitives (all 3):** `.field`/`.label`(+`.req`/`.opt`)/`.hint`/`.input`/`.textarea`/`select.input`; focus = `border-color:var(--line-strong); box-shadow:var(--focus-ring)` (= `--shadow-focus-ring`). Custom SVG chevron on selects. Source: `task…:98-116`.
- **Responsive collapse (all 3, `@media max-width:980px`):** modals → 94–95vh; two-pane bodies collapse `grid-template-columns:1fr; grid-template-rows:1fr auto`, preview becomes a bottom strip (`border-top`, `max-height:300–320px`); task template grid `cols3→cols2`. Source: `task…:225-228`, `trigger…:291-295`, `job…:336-340`.

## Relevant Sources

- `docs/design/opendesign/modals/create-task-redesign.html:66-116` — shell + header/footer + field primitives (archetype A).
- `docs/design/opendesign/modals/create-task-redesign.html:265-301,356-466,619-653` — Simple/Advanced toolbar + `#adv-groups` + template re-render.
- `docs/design/opendesign/modals/create-trigger-redesign.html:62-120,160-167,242-295` — two-pane shell, `.flow` spine, `.subcfg` rail, preview CSS, responsive.
- `docs/design/opendesign/modals/create-trigger-redesign.html:525-563,910-969` — live preview cards + summary/match/rendered-prompt render logic.
- `docs/design/opendesign/modals/create-job-redesign.html:149-165,179-241,433-451,646-733` — output-mode `.seg` cards, cron builder, reliability collapsible, preview cards.
- ~~`docs/design/opendesign/systems/design-system.html:456-517,1201-1260`~~ *(deleted)* — Principles / token ladder / class contract; see `design-system/foundations.html` + `patterns.html` § Modals.
- `packages/ui/src/tokens.css:33-333` — canonical color/type/radius/motion/geometry tokens (`--width-modal-*`, `--height-modal-*`, `--width-right-rail-default:468px`, switch/button geometry, `--color-overlay-scrim`, `--overlay-blur`).
- `packages/ui/src/index.ts:62-166,196-267,353-435,568-626` — exported primitive inventory (Dialog, Tabs, Select, Switch, Field, PillGroup, FormSection, EditorFooter, DetailHeader, RadioCard, ConfirmDialog, Stepper).
- `packages/ui/src/components/dialog.tsx:83-206` — `DialogContent unframed` + `DialogHeader`/`DialogFooter variant="ruled"` chrome contract.
- `packages/ui/src/components/custom/radio-card.tsx:18-63` — selected-state contract (glaze + inset ring, **accent reserved for CTAs**).
- `packages/ui/src/components/custom/confirm-dialog.tsx:110-198` — reference composition of the ruled dialog (header/note/footer) already shipped.
- `packages/ui/src/components/custom/form-section.tsx`, `editor-footer.tsx`, `detail-header.tsx`, `pill-group.tsx`, `switch.tsx`, `field.tsx`, `tabs.tsx` — the composites the reference patterns map onto.

## Transferable Patterns

- **Shared `EntityModal` shell** → applies to every create/edit modal on other pages, replacing bespoke per-page dialog markup. Compose `Dialog` + `DialogContent unframed` + a `DialogHeader variant="ruled"` carrying the icon well + `Eyebrow` + `DialogTitle` + `DialogDescription`, and an `EditorFooter`/`DialogFooter variant="ruled"` carrying info-note + Cancel + primary CTA. `ConfirmDialog` already proves this exact composition ships today (`confirm-dialog.tsx:110-198`).
- **Two-archetype matrix** → the reusable decision the operator needs. **Archetype A (single-column + Simple/Advanced)** for entities with many fields but *no derivable artifact* (e.g. settings-like records). **Archetype B (form | 468px preview)** for entities whose config *produces something readable back* (a rendered value, a schedule, a computed target). Right rail width = `--width-right-rail-default:468px` (already a token).
- **Simple/Advanced density lever** → `PillGroup` (`pill-group.tsx`) driving a `mode` state that toggles advanced field groups + collapsibles. This is the canonical answer to "modal muito denso → versão simplificada": one control, two field sets, snap-back safety when leaving Advanced.
- **Numbered/stepped section grammar** → archetype A sections map to `FormSection` (title + icon + rightLabel + description); archetype B spine maps to the exported **`Stepper`/`StepperRail`/`StepperItem`/`StepperIndicator`** (reui), which already provides the connector-line + numbered-node visual the `.flow` hand-rolls.
- **Choice cards** → `RadioCard` (`radio-card.tsx`) for the `.tpl`/`.seg`/`.event` selection pattern — *provided* the selected-state divergence (below) is resolved.
- **Progressive disclosure** → `Collapsible`/`Accordion` for the `.coll` advanced groups; inline conditional sub-config maps to a `Field`-nested block gated on selection (the `.subcfg` accent-rail treatment needs the P2-checklist reconciliation below).
- **Controls** → `Switch` + `Field`/`FieldTitle`/`FieldDescription` for switch rows; `Select`/`NativeSelect` for scope/owner/agent pickers; `Input`/`Textarea`/`FieldLabel`/`FieldError` for text fields; `SearchInput` for the trigger event search; `Pill`/`StatusDot` for match/priority badges; `JsonViewer`/`CodeBlock` for the preview JSON/curl/payload.
- **State-reactive CTA + footnote** → the `submit-label`/`foot-note` machinery (label swaps "Enqueue task"↔"Save draft" from state) is exactly the mechanism an **edit mode** would reuse to relabel "Create"→"Save changes", so the create shell already contains the edit affordance.

## Risks / Mismatches

- **Accent-tint choice cards vs `RadioCard` contract.** References fill selected cards with `--accent-tint` + `--accent-dim` inset ring; the shipped `RadioCard` documents the opposite — selected = `bg-surface-glaze shadow-focus-ring-inset`, with an explicit rule "**No accent border, no `--accent-tint` fill — accent stays reserved for true CTAs**" (`radio-card.tsx:18-23`). Transferring the reference look verbatim contradicts the primitive; one must win. This compounds with `design-system.html:1257` (P2: "No accent side-stripes, gradients…").
- **Accent left-rail on `.note`/`.subcfg` vs migration checklist.** The `.note` and `.subcfg` blocks use `border-left:2px solid var(--accent-dim)` (`task…:194`, `trigger…:163`), which reads as an accent side-stripe — the exact treatment `design-system.html:1257` forbids. Reconcile toward the `Alert` primitive (`ConfirmDialog` already routes notes through `Alert`, `confirm-dialog.tsx:132-143`).
- **Modal radius drift.** References use `border-radius:var(--radius-xl)` = 14px on `.modal`; the exported `DialogContent` uses `rounded-lg` = 10px (`dialog.tsx:86`). Same for choosing modal width/height — references use raw `min(760px…)` / `min(1180px…)` where tokens already define `--width-modal-md:720px`, `--width-modal-xl:1180px`, `--height-modal-xl:840px` (`tokens.css:329-350`). Decide whether to add a modal-`radius-xl` token or conform to `rounded-lg`.
- **Scrim alpha + blur drift.** References: `--scrim: rgba(0,0,0,0.62)`, `blur(2px)`. Canonical: `--color-overlay-scrim: rgba(0,0,0,0.55)` (`tokens.css:90`), `--overlay-blur: 3px` (`tokens.css:341`, consumed by `DialogOverlay`, `dialog.tsx:77`). Static artifacts drifted from the runtime token.
- **Title weight/size drift.** Reference `.modal__title` = 16/600; `DialogTitle` renders `text-item-title` (15px) `font-medium` (500) (`dialog.tsx:213`, `tokens.css:171`). A dedicated `--text-modal-title` (13.5px) also exists (`tokens.css:193`) and matches neither — the modal title tier is unsettled.
- **Footer button tier inconsistency.** Reference `.btn{height:30px}` = `button-lg`; `design-system.html:242` canonizes the product button at `h26` (`button-default`). Both are valid tokens but the system is internally inconsistent about which tier a modal CTA uses.
- **Hand-rolled controls re-implement solved behavior.** The references paint `.pill`/`.switch`/`select`/cards as raw HTML with only `aria-pressed`/`aria-checked`. They omit the `focus-visible` rings the exported `PillGroup`/`RadioCard`/`Tabs`/`Switch` already ship, and none of the base-ui Dialog affordances (focus trap, `aria-modal`, escape/close wiring, exit animation via `AnimatePresence`). Transferring the *look* without composing the *primitives* would regress a11y — build modals from `@agh/ui`, use the references only as visual spec.
- **`FormSection` grammar gap.** The task `.sec` uses a **mono numbered ordinal** (`.sec__num` "01"/"02") on a flat top-border divider; `FormSection` (`form-section.tsx`) is a `bg-canvas-soft` card with no ordinal slot. Adopting the numbered look requires extending `FormSection` (add ordinal) or accepting the card grammar instead.
- **Preview composites don't exist yet.** Summary sentence, rendered-prompt-with-miss-highlight, next-runs list, and request-payload card have no `@agh/ui` export. Building them per entity risks violating the "Truthful UI > plausible UI" rule (`CLAUDE.md`) — a preview must only render a derivation the runtime actually produces. Archetype B should be reserved for entities with a *real* derived artifact, not retrofitted decoratively.
- **No canonical modal contract to conform to.** `design-system.html` canonizes topbar/page-head/listing/rows/cards/empty but has **no Modal/Dialog section**. The three references are the de-facto contract, and they already diverge (task = single-column + toolbar + `radius-xl` rows-4 grid; trigger/job = two-pane + `radius-xl` rows-3 grid; task has no preview). Without a written modal contract, "seguir os padrões" has no single source of truth — producing one is arguably the slice's most load-bearing deliverable.

## Open Questions

- **Edit-mode anatomy is unspecified.** All three references are create-only ("Create trigger", "Enqueue task"). The operator explicitly wants *criação e edição*. Edit deltas (title/eyebrow/CTA copy, prefilled state, destructive actions, dirty-state guards) are an inference from the create shell + the state-driven relabel machinery — a parent decision, not evidenced by these sources.
- **Which selected-card state is authoritative** — the reference accent-tint fill or the `RadioCard` glaze-only contract? This gates every choice-card in every future modal.
- **Add modal geometry tokens or conform?** Should `tokens.css` gain a modal-`radius`/`--text-modal-title`/scrim-blur reconciliation, or should the references be corrected to existing `--width-modal-*` / `rounded-lg` / `--overlay-scrim` values?
- **Which non-task/trigger/job entities warrant archetype B (preview) vs A?** Depends on whether each entity produces a derivable artifact — a backend-field question owned by adjacent slices, out of scope here.
- **Promote a shared `<EntityModal>` composite to `@agh/ui`?** The three references duplicate ~250 lines of identical shell CSS; a single header/footer/responsive shell composite would enforce the contract, but that is an authoring decision for the parent, not this read-only slice.
- The then-`design-system.html` file was grep-sampled only; modal grammar now lives in `design-system/patterns.html` § Modals + `modals/MODAL-STANDARD.md`.

## Evidence

- `docs/design/opendesign/modals/create-task-redesign.html`
- `docs/design/opendesign/modals/create-trigger-redesign.html`
- `docs/design/opendesign/modals/create-job-redesign.html`
- ~~`docs/design/opendesign/systems/design-system.html`~~ → `docs/design/opendesign/design-system/`
- `packages/ui/src/tokens.css`
- `packages/ui/src/index.ts`
- `packages/ui/src/components/dialog.tsx`
- `packages/ui/src/components/field.tsx`
- `packages/ui/src/components/switch.tsx`
- `packages/ui/src/components/tabs.tsx`
- `packages/ui/src/components/custom/form-section.tsx`
- `packages/ui/src/components/custom/editor-footer.tsx`
- `packages/ui/src/components/custom/detail-header.tsx`
- `packages/ui/src/components/custom/radio-card.tsx`
- `packages/ui/src/components/custom/confirm-dialog.tsx`
- `packages/ui/src/components/custom/pill-group.tsx`
