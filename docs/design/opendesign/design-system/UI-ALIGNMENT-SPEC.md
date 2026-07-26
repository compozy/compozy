# UI Alignment Spec — `packages/ui` → AGH Design System

**Date:** 2026-07-22 · **Authority:** `design-system/` (this folder) is the visual reference; `packages/ui/src/tokens.css` remains the token *source of truth* — every token change below lands there first and flows to `DESIGN.md` via `make codegen` (DESIGN.md is generated and currently in-sync; never hand-edit its token regions).
**Scope:** visual alignment of the base kit (`packages/ui/src/**`) to the quality bar set by `design-system/ds-core.css` + chapters. Implementation follows repo workflow (designer agent, `agh-design` + `ui-craft`, Storybook capture via `agh-ui-screenshot`).
**Method:** three parallel audits (controls · surfaces/chrome · tokens+DESIGN.md) against the DS canon. All file:line refs are to `packages/ui/src` at audit time.

---

## 0 · Executive summary

The token layer is mature — warm-dark ramp, motion ladder, focus ring, shell chrome all present. The gap is **discipline at the component layer**: geometry hardcoded past the tokens, near-duplicate primitives with drifting values, and a handful of rule violations the DS forbids outright.

Systemic findings (each expanded below):

1. **Control-height chaos** — five different "default" heights (26 / 36 / 32 / 28 / 44), form controls on raw `h-9`/`h-8` literals; a Button next to an Input does not align.
2. **Icon-well fragmentation** — six well identities (34/8/elevated · 24/6/glaze · 38/10/soft · 36/10/canvas · 22/5/badge-fill · bare accent icons); `--radius-icon-well` is consumed by exactly one component.
3. **Accent used decoratively** — `Card activeRail` accent left-bar, Sparklines painting generic data in `accent-tint-strong`, charts mapped to signal colors. The DS rule is absolute: accent = state or primary action; viz ink is neutral.
4. **Four disabled models, two hover treatments, one missing focus ring** — interaction states are inconsistent across peers.
5. **Triplicated vocabularies** — 3 crumb systems, 3 empty-state grammars, 3 KPI voices, ≥7 hand-rolled tone maps, 6+ copies of the same surface-tile classes.
6. **Off-token literals** — `h-9`, `size-[34px]`, `size-[22px]`, `min-h-16`, `text-lg`, `tracking-[-0.01em]`, gutters `120/140/180px`, avatar inline px.
7. **Token gaps** — no `--height-input`, no 32/34 height tiers, no neutral `--color-viz-*` ink, no z-scale, no 17/24px text tiers, two orphaned tokens, stale comments.
8. **Legacy chrome overlap** — `Topbar` is already the OS window-head target; `DetailHeader` and `Breadcrumb` duplicate its identity and must be demoted/consolidated.

---

## A · Token layer (`packages/ui/src/tokens.css`)

### A1. Add

| Token | Value | Rationale |
|---|---|---|
| `--height-input` | `36px` | Ends `h-9` literals in Input/Textarea/Select/NativeSelect/CommandSelect trigger. Matches modal library (`--height-input: 36px`, verify-locked). |
| `--height-control-compact` | `32px` | The `h-8` tier (Select sm, NativeSelect sm, Command input, settings `.ctl`). |
| `--height-search` | `28px` | DS search-field height (context strip / toolbars). SearchInput moves 26 → 28. |
| `--height-button-cta` / `--height-button-cta-lg` | `36px` / `44px` | Tokenizes `h-9`/`h-11` in `button-variants.ts:26,28`. |
| `--height-textarea-min` | `84px` | DS/modal default; replaces `min-h-16` (64px). Keep `--code 120` / `--tall 160` as modal-side values. |
| `--color-viz-line` / `-fill` / `-bar` / `-other` / `-grid` | `rgba(255,255,255,.42 / .055 / .30 / .26 / .045)` | Neutral data ink (DS Foundations §02 / dashboard canon). Charts stop borrowing signal colors. |
| `--z-menu` / `--z-scrim` / `--z-sheet` | `40 / 60 / 61` | Prototype z-scale, tokenized. Document shell tiers (menubar 500 · dock 600 · popover 900 · overlay 1000 · toasts 1200) alongside. |
| `--font-weight-display` | `620` | The numeric-display weight the approved dashboard uses for KPI values; today it exists nowhere. |
| `--text-kpi-compact` | `1.0625rem` (17px) | In-window KPI value (DS Components §09); fills the missing 17px text tier. |
| `--size-icon-well-row` | `34px` | Tokenizes `size-[34px]` (listing-row well). |
| `--size-topbar-glyph` | `22px` | Tokenizes `size-[22px]` (window-head glyph). |
| `--size-status-dot` / `--size-status-dot-sm` | `7px` / `6px` | DS signal vocabulary (`.d` = 7px); fixes StatusDot's inert size prop with real values. |
| `--tracking-row-title` | `-0.01em` | Replaces `tracking-[-0.01em]` arbitrary in listing-row title. |
| `--width-kv-label` | `140px` | Unifies the remaining metadata gutters: ContextBox + MetadataList converge on one shared 140px token. |
| `--size-avatar-sm` / `-default` / `-lg` | `20 / 24 / 32px` | OwnerAvatar drops inline `style` px. |

### A2. Change

| Token | From → To | Rationale |
|---|---|---|
| `--text-kpi-value` | `1.75rem` (28px) → `1.5rem` (24px) | Align to the user-approved dashboard scale (24/620). 28px was never shipped in an approved surface. Pair with `--font-weight-display: 620`. |

### A3. Fix / clean

- **Stale comments:** L218 radius comment omits `xxs 3` (start the ladder at 3); L231 "single fast tier + one ease curve" → rewrite to "fast/base/slow + shell tier; ease-out / ease-in-out / spring".
- **Duplicate xs/sm:** `--height-button-xs` ≡ `--height-button-sm` ≡ 22px (and icon twins). Collapse to `-sm` and delete `-xs`, or give xs a real value; today the distinction is padding-only and lives in the variant, not the token.
- **Orphans:** `--text-form-input` (12.5px, unused — Input uses `text-small-body`): delete or wire it into Input as the semantic name. `--height-form-textarea` (136px, unused): delete in favor of `--height-textarea-min`.
- **Prefix split:** `--space-*` (`:root`) vs `--spacing-*` (`@theme`) — document the rule in the file header (utility-generating = `--spacing-*`) or migrate `:root` names; today it reads as accidental.
- **Name collision:** `--height-modal-md 760` vs `--width-modal-md 720` share a stem across axes — acceptable, but add a comment block pairing them so nobody "fixes" one against the other.
- **DESIGN.md generator:** add regions for font families, the weight ladder (400/510/600/700 + new 620), and `--leading-*`; these exist in tokens.css but are prose-only in DESIGN.md. Run `make codegen` after every change above; `make codegen-check` gates drift.

---

## B · Cross-cutting contracts (the rules components must converge on)

**B1. Control-height ladder.** One ladder, token-backed, no `h-*` literals on controls:
`22 sm-button/chip · 24 pill-segment · 26 button/icon-button/property-row · 28 search · 30 lg-button · 32 compact-control · 36 form-input/select/command-trigger · 40 lane-tabs · 44 cta-lg/mobile-target`.
Toggle joins the **button** ladder (26/22/30) — a toggle is a button, not a form field. Button `cta`/`cta-lg` keep 36/44 via the new tokens.

**B2. Radius rules.** Field-like controls = `radius-md` 8 (SearchInput 6→8; NativeSelect sm 5→8). Panel-like floating surfaces (popover, menu, select content, command) = `radius-lg` 10; tooltip = `radius-md` 8 (documented exception, small surface). Value chips = `radius-chip` 5; mono badges/counts = `radius-mono-badge` 4. Pills = `radius-xs` 4.

**B3. One disabled model, two classes.** Field-like (Input, Textarea, Selects, SearchInput, Command input): `opacity-100` + swap (`bg-canvas`, `border-line-soft`, `text-disabled`) — value stays readable. Button-like (Button, Toggle, Pill, Switch, menu items): `opacity-50`. Kill the stray `opacity-60` (SearchInput) and Textarea's `opacity-50` (it's field-like → swap model).

**B4. Hover ladder.** Rows/menu items rest on `row-hover` (2.2%); **click-target controls** (outline/secondary/ghost buttons, toggles, command-select trigger) hover on `btn-default-hover` (7%) — 2.2% on a 26px button is imperceptible. Keyboard-highlighted rows in ALL menus (Dropdown item focus included) = `bg-elevated` + `text-fg-strong`, matching Select/Command; hover ≠ highlight.

**B5. Focus.** `shadow-focus-ring` (2px white .5) on every interactive `:focus-visible`, no exceptions — add it to `CommandInput` (`command.tsx:60`). Pointer focus on fields = border strengthen only (already correct).

**B6. Hairline rule.** Rows inside list shells/cards = `border-line-soft` (ListingRow ✓); table rows, section dividers, card footers = `border-line` (Table ✓). This is a *rule*, not drift — write it into DESIGN.md prose so it stops being "fixed" back and forth. First/last-child resets standardize on `last:border-b-0` / `first:border-t-0`.

**B7. Icon-well ladder** (size / radius / bg): `22 topbar-glyph · radius 6 · badge-fill + border-line` — note: today it ships `rounded-sm` 5, move to 6; `24 card-logo · radius 6 · surface-glaze`; `34 listing-row · radius-md 8 · elevated`; `36 modal/route well · radius-icon-well 10`; `38 empty · radius-lg 10 · canvas-soft`. Section/KPI bare accent icons (`size-5 text-accent`, no bg) are allowed only as section-label glyphs — never as row identity.

**B8. One tone map.** Create `lib/tone.ts` exporting the `tone → {text, bg, tint, solid}` map and replace the ≥7 hand-rolled `Record`s (pill, status-line, live-badge, stacked-progress, status-breakdown, connection config, catalog-card, metric). Decisions baked in: toned *text* on dark uses `accent-strong` for the accent tone (LiveBadge is right, StatusLine is wrong); "neutral" text = `neutral-ink`, never `muted`/`subtle` ad hoc.

**B9. Accent budget (enforced).** Accent = primary action, live/running state, current selection. Consequences: remove `Card activeRail` (`card.tsx:20-21`); Sparkline + QueueHealthSparkline move to `viz-bar` ink (stuck segments may keep `accent-tint-strong` — "stuck" is state); shadcn `--color-chart-1..5` stop being the default data palette — generic magnitude uses `--color-viz-*`, signal colors appear only when the series *is* that state. Tabs `liveLabel` keeps accent (live = state). `PillLink`'s dead `hover:border-accent` gets a `border-transparent` base or is dropped.

---

## C · Per-component spec (current → target)

### Controls

| Component (file:line) | Current | Target |
|---|---|---|
| **Button** `button-variants.ts:26,28` | `cta h-9`, `cta-lg h-11` literals | `h-(--height-button-cta)` / `-cta-lg` tokens |
| Button xs/sm `:20-25` | both 22px, padding-only difference | collapse to `sm` (keep alias `xs` → sm during migration) |
| Button outline/secondary/ghost hover | `bg-hover` 2.2% | `bg-btn-default-hover` 7% (B4) |
| **Input** `input.tsx:12` | `h-9` literal; `text-small-body` | `h-(--height-input)`; adopt `--text-form-input` as the semantic size name |
| **Textarea** `textarea.tsx:23` | `min-h-16` (64px); disabled `opacity-50`; `px-2.5` | `min-h-(--height-textarea-min)` (84px); field disabled model (B3); `px-3` to match Input |
| **SearchInput** `search-input.tsx:36-68` | 26px, `rounded` 6, disabled `opacity-60` | `h-(--height-search)` 28, `rounded-md` 8, field disabled model |
| **NativeSelect** `native-select.tsx:23` | `h-9`/`h-8` literals; sm `rounded-sm` 5 | height tokens; sm keeps `rounded-md` 8 |
| **Select** `select.tsx:44` | `h-9`/`h-8` literals | `--height-input` / `--height-control-compact` |
| **CommandSelect trigger** `command-select.tsx:63` | `h-9`; hover `bg-hover` | `--height-input`; hover 7% (B4) |
| **Command input** `command.tsx:60` | `h-8`; **no focus ring** | `--height-control-compact`; add `focus-within:shadow-focus-ring` |
| **Toggle** `toggle-variants.ts:12-15` | default `h-9` 36, sm `h-7` 28, lg `h-11` 44 | button ladder: 26 / 22 / 30 (`--height-button-*`) |
| **DropdownMenu item** `dropdown-menu.tsx:85-88` | focus `bg-hover` | highlight = `bg-elevated text-fg-strong` (B4) |
| **Tooltip vs Popover radius** | 8 vs 10 | keep — documented rule (B2) |
| **Kbd** `kbd.tsx:10` | `h-5`/`min-w-5` literals | fine to keep 20px; tokenize only if a second consumer appears |
| **StatusDot** `status-dot.tsx:33-36` | size prop inert (both 6px) | default 7px / sm 6px via new tokens; `ring` variant keeps hollow style |
| **Pill heights 17/19/22** | flagged off-grid by audit | **no change** — intentional canon (tag 17 · pill 19 · chip/status 22) |
| **Switch** | 32×18 token-clean | no change (already canon) |
| **PriorityBars** `priority-bars.tsx` | 3 bars, state colors | keep (priority *is* state); do **not** use as a reasoning meter |
| **IntensityMeter** | absent from kit (lives in `web/src/systems/runtime`) | promote the 7-bar meter into `packages/ui` as a generic primitive (story + test per repo rule); accent-strong fill only |
| **Checkbox/Radio** | absent (only RadioCard + menu items) | acknowledged P2 gap — needed before any bulk-select table work |

### Surfaces & composition

| Component (file:line) | Current | Target |
|---|---|---|
| **ListingRow.Icon** `listing-row-parts.tsx:51` | `size-[34px] rounded-md` | `size-(--size-icon-well-row)`; keep radius-md 8 + `bg-elevated` (B7) |
| **ListingRow.Title** `:83-86` | `text-sm` + `tracking-[-0.01em]` | `text-card-title` + `tracking-(--tracking-row-title)` |
| **CatalogCard.Logo** `catalog-card-parts.tsx:54-59` | 24 / `rounded` 6 / glaze | no change — matches DS card icon; tokenized already |
| **Card.activeRail** `card.tsx:20-21` | accent left-rail on content card | **delete the variant** (B9); selected state = `bg-surface-glaze + shadow-inset-strong` like CatalogCard |
| **Topbar glyph** `topbar.tsx:170-202` | `size-[22px] rounded-sm` | `size-(--size-topbar-glyph)` + `rounded` 6 (DS w2-glyph) |
| **Empty** `empty.tsx:65-70` | title `text-lg`; well 38/lg/soft | title `text-empty-h1`; well unchanged (canon) |
| **RouteState** `route-state.tsx:36-53` | bordered, well 36/icon-well/canvas | converge on Empty grammar: same 38/lg/soft well + title token; keep border as `framed` variant of Empty; then delete RouteState |
| **KpiCard + Metric** `kpi-card.tsx` / `metric.tsx` | near-duplicates (label case + size) | merge into one `Metric` (label `eyebrow\|sentence`, value `compact 17 · default 24 · lg 24`); value = `--text-kpi-value` 24 / `--font-weight-display` 620; delete KpiCard after migration |
| **Surface tile** (StatusCard/RunCard/DescriptionCard/KpiCard/Metric ×`px-5 py-4`, FormSection `py-5`, Card `p-4`) | 6+ hand-copies | new `Surface` primitive: `rounded-lg bg-canvas-soft` + `default px-5 py-4` / `compact px-4 py-3`; consumers compose it |
| **Sparkline** `sparkline.tsx:39-54` | bars `bg-accent-tint-strong` | `bg-(--color-viz-bar)`; accent only when the series encodes live/selected state |
| **QueueHealthSparkline** `:28-39,64` | accent stuck-fill ok; inline tooltip style literals | keep stuck accent; move tooltip literals to tokens (`--color-*`, `text-eyebrow`) |
| **StackedProgress / StatusBreakdown** | own tone Records; tracks `bg-canvas` | tone map from `lib/tone.ts` (B8); track = `bg-canvas-tint` (DS meter track) |
| **OwnerAvatar** `owner-avatar.tsx:10-14` | inline `style` px 20/24/32 | `size-(--size-avatar-*)` classes; palette unchanged |
| **MetadataList / ContextBox gutters** | 120 / 140px arbitrary | `--width-kv-label` 140 for both metadata layouts. |
| **Section vs FormSection heads** | semibold-uppercase vs medium sentence | keep both, but name the rule: Section label = page-zone eyebrow (uppercase); FormSection title = form group (sentence). Add one shared `SectionLabel` part so the two stop drifting |
| **Table** | hairlines `line`, hover `bg-hover` | no geometry change; document B6; selected `bg-elevated` stays |
| **Dialog/Sheet literals** | `max-w-[calc(100%-2rem)]`, `2.5rem` offsets | acceptable; tokenize only the recurring `2rem` page-edge inset if a third consumer appears |
| **Toaster** | dark sonner, radius-lg | no change |

---

## D · Consolidations (delete-list included)

1. **Crumbs: one system.** `Topbar` crumbs (slash-separated, 150px truncation, `…` collapse) are canonical. **Delete `breadcrumb.tsx`** (shadcn) after migrating its few call sites; **demote `DetailHeader`**: strip back/crumbs/H1 (Topbar owns identity — its own doc at `page-shell.tsx:20-25` already says so), keep it as a body **summary strip** (pills ≤2 + meta + actions), rename accordingly (`DetailSummary`). This mirrors the DS rule "identity renders once" and pagehead-redesign §"chrome-before-content 184px → 44–82px".
2. **Empty states: one grammar.** `Empty` absorbs `RouteState` (framed variant + `cause` slot); `DataSurface` keeps routing states to it. Delete `RouteState`.
3. **KPI: one voice.** `Metric` absorbs `KpiCard` (§C). `MetadataTile` stays (different job: dense k/v tile).
4. **Tone map: one module** (`lib/tone.ts`, B8).
5. **Avatar: one size scale** — OwnerAvatar and Avatar keep their distinct jobs (identity-palette monogram vs image avatar) but share `--size-avatar-*`.

Every deletion follows the greenfield rule: hard cut, migrate call sites in the same change, no compat aliases beyond one release.

---

## E · Legacy chrome migration (OS window-head)

`Topbar` (`topbar.tsx:212`) already implements the DS window-head: 44px, controls slot, 22px glyph OR back+crumbs, 13px H1, RouteNav, trailing status · vsep · ≤2 actions. Verdict per component:

| Component | Verdict |
|---|---|
| `Topbar` | **Keep — it is the target.** Only fixes: glyph radius 5→6 + size token (§C); enforce ≤2 trailing actions + `···` overflow at the API level (today unbounded). |
| `DetailHeader` | Demote to `DetailSummary` (§D1). Its `text-detail-h1` 22.4px H1 disappears from chrome — the DS reserves that size for in-content heroes only. |
| `Breadcrumb` | Delete (§D1). |
| `RouteNav` | Keep — sanctioned peer-route segmented links (= DS `.w2-tabs` role). Ensure it hides at drill-in depth like the DS head tabs. |
| `PageShell` / `PageContent` | Keep — layout infra. `max-w-content-max` 1320 already canon. |

---

## F · Priorities & verification

**P0 — rule violations (visible wrongness):** B9 accent items (Card activeRail, Sparkline ink, chart palette → viz tokens); B4 hover ladder on buttons; B5 CommandInput focus ring; StatusDot inert prop; control-height tokens for form fields (`--height-input` + literals swap — mechanical, high leverage).
**P1 — coherence:** Toggle onto button ladder; SearchInput 28/radius-md; disabled model unification; icon-well ladder (topbar glyph, RouteState convergence); tone-map module; crumb consolidation + DetailHeader demotion; Empty/RouteState merge; KpiCard/Metric merge; kpi-value 24/620.
**P2 — polish & gaps:** Surface primitive refactor; k/v gutter tokens; avatar size tokens; orphan-token cleanup; stale comments; DESIGN.md generator additions; Checkbox/Radio primitives; IntensityMeter promotion.

**Verification:** every P0/P1 change ships a Storybook story diff captured with `agh-ui-screenshot` (before/after per component); `make codegen-check` green after token edits; `compozy-ui-reuse` lint stays green (no shadowed primitives); the DS chapter demos (`design-system/components.html`) are the visual parity reference — a changed primitive must read identical to its DS demo at 100% zoom.

**AGH Impact Audit:** Native tools — no impact (pure UI kit + tokens; no `agh__*` surfaces touched). Extensibility and hooks — no impact (no extension/hook/config surfaces; `packages/ui` internal). Workspace data isolation — no impact (no data paths). Official AGH skill — no impact (no public behavior/CLI/tool change; visual-only). Web/Docs impact — `web/` consumers of renamed/merged primitives (`KpiCard`, `RouteState`, `Breadcrumb`, `DetailHeader`) migrate in the same change; `DESIGN.md` regenerates via codegen.
