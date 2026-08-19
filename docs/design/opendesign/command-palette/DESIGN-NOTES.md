# Command palette — root, views, input flows, settings

Design contract for the surfaces in `.compozy/tasks/command-palette/_uiux.md`,
delivered as ten boards in this folder. Companions: `_spec.md` (behavior
authority), `_user_stories.md` (ACs/ECs). This file is the locked semantic
contract — every ghost tail, hint slot, and confirm verb on the boards traces
back here.

**Binding visual lineage: production.** `packages/ui/src/tokens.css` and the
shipped `@compozy/ui` / `web/src/systems/os` components are the authority;
the herdr-parity palette grammar supplies the class vocabulary, not the
values. `command-palette.css` opens with a set-scoped production-parity token
lane that redefines the prototype token names to the values production ships,
so every rule in the set reads a token and the boards render production
pixels. Each board class maps to a shipped primitive or a declared domain
composite — see the Production component map below.

## Locked decisions

### Root & results (S1 · S10 · S14)

Group precedence is Pinned → Recents → context group → curated groups.
Entities never interleave into the command group.

- **Ghost tail** renders only for a high-confidence top result, after the
  caret, preserving typed casing. `→` at the end of the input accepts.
- **Every bound row** carries its chord as ONE `.pal-chord` text span
  (`⌘N`, `⌥⇧N`) — the `CommandShortcut` voice, no keycap chrome. See
  Chord tiers below.
- **Alias** renders `Title (alias)` (e.g. `Capture note (cap)`).
- **Settings rows** read `Settings → {page}`. **App rows** read `Open {app}`.
- **Workspace label** is a sub-line suffix (herdr `.pal-srow` precedent)
  plus ONE widened globe scope chip — current-scope rows carry no label.
- **The scope chip rides the palette head**, trailing the input (11px
  globe). A lone chip never earns its own `.pal-chips` band — that band
  exists only for real filter-chip sets.
- **Capped entity sections** show exactly 6 rows + the exact note
  `showing N of M`. Silent truncation is forbidden.
- **Fallback row** `Ask agent: '{query}'` is delegation, not execution —
  own glyph + info-tinted treatment, never a plain command row. Present
  alone on zero-match and alongside results on weak match.
- **Destination mode** heading `Open in this tab`, placeholder
  `Open in this tab…`. Ineligible groups are absent (not disabled).
- **In-palette pending** is a motion token only, never fake progress
  percentages.

### Availability & truthful UI

Availability reasons render verbatim from the runtime in a structured
hint slot (`.pal-hint`), never baked into the label:

- `needs two windows on this desktop`
- `requires an attached shell`
- `extension notes is unhealthy (crash loop)`

Unknown reason → `unavailable right now`, never a fabricated specific.
Daemon down → action commands disabled with `runtime unavailable` while
AvailabilityExempt commands stay live. A failing domain endpoint yields
an inline section error naming the domain; siblings stay unaffected.
Reasons are sentence fragments without trailing periods.

### View stack (S2)

Breadcrumb ≤3 slots, left-truncating. The `…` crumb tooltip is
`Earlier levels`. Per-level scoped search. ⌫ pops one level only on an
empty query. Esc closes the whole stack. Re-push mounts fresh. Live
refresh never steals selection (nearest-neighbor fallback).

Frames:

- **view-unavailable** names the source extension
- **loading**
- **timeout + retry**

Unknown view kind degrades to `view requires a newer CompozyOS`.

### Bands (US-039)

Soft budget 150 ms, hard ack 3 s, circuit-break at 3 consecutive misses.
Busy keeps previous rows visible — the list never blanks for a spinner.
Degraded = last-good rows + inline retry. Circuit-broken until reopen
while Esc / ⌫ / navigation / other views keep working. A program crash
yields an unavailable frame naming the extension. `view reloaded` note
after a dev-mode edit with the view open.

### Domain views & detail (S3 · S4)

Chips are single-select with truthful counts (story fixture: All 12 ·
Queued 3 · Running 4 · Needs-approval 2 · Done 3). ONE shared status-tone
dictionary + shared attention-first comparator — no view-local tone map.
Empty-with-filter names the filter (`No failed loops`) and clears with
one keystroke. Mount cap 150 then virtualize. Vault lists names and
metadata only — values are structurally absent. The detail pane is
selection-driven; focus never leaves the list. Neutral empty is
`no preview`. Stale-cleared after deletion. Independent scroll.
Sanitized rich text degrades to plain text.

### Forms & grid (S5 · S6)

Declared field order. Per-field inline errors; submit focuses the first
invalid. Submit-failed keeps the form open with values intact. Passwords
masked. An empty dropdown shows its declared hint. Grid = sections;
tiles are image | token | emoji + title + optional badge. Media-failed
keeps the title over a placeholder glyph. ←→↑↓ 2D navigation joins the
ladder without breaking it. Empties share the list grammar.

### Action panel (S7)

⌘K on the selected row toggles it, anchored to the row. Typing filters
actions. Primary marked ↩. Meta-actions on every command row: `Pin` /
`Unpin` · `Set alias…` · `Set shortcut…`. Entity destructive actions
wear danger text + glyph. A disabled command → the panel lists
meta-actions + the verbatim reason only. Row vanishes → panel closes,
nearest-neighbor selection, no dead fire.

### Args & confirmation (S8 · S9)

The input bar morphs into inline typed fields (fixture: `Capture note`
→ field `title`, text, placeholder `Note title`, required · field
`tag`, dropdown, options `inbox` / `idea`, optional; sample value
`Standup follow-ups`). ⇥ traverses. ⏎ blocks on missing required and
focuses the first empty required field with its placeholder emphasized.
Dropdowns type-to-filter. Invalid-type message is inline. A hotkey can
open the palette directly in args mode.

Confirmation renders declared copy only — fixture: title
`Purge archived notes?`, body `Permanently deletes every archived note
in this workspace.`, confirm `Purge`, `Cancel` focused by default.
Repeat-guarded. Target invalidated between trigger and confirm → honest
message, never executes.

### Settings & hotkeys (S12)

Whole-registry table, source filter `Core areas` / per-extension.
Columns: command · effective binding · alias (inline edit) · source.
Alias rule `1–32 characters, no whitespace`. Conflicts name the culprit
(`already used by 'session.new'`) and offer explicit overwrite. The
overwritten loser becomes unbound and is flagged. An extension dormant
default reads `default unavailable — conflicts with X`. Global hotkeys
are shell-gated — in browser mode rows are disabled with
`requires desktop shell` (Settings is the ONLY surface where they
render disabled instead of absent). A chord captured by another app
reads `unavailable — in use by another application` with the previous
binding still effective. macOS Accessibility callout deep-links System
Settings. Non-QWERTY limitation surfaces in recorder copy. Reset-one /
reset-all. Global summon default ⌘⇧Space.

### Extensions & palette settings (S15 · S16)

Per-extension Palette panel: contributed commands with effective
bindings + views (fixture `ext.notes`: `Capture note ⌥⇧N` ·
`Recent notes` · `Purge archived notes` · views `Recent notes`,
`Browse notes`). An unhealthy extension → contributions grayed with
`extension notes is unhealthy (crash loop)`. Settings › Palette:
agent-fallback toggle (v1 ships exactly one fallback target — no
ordered list renders), personalization master-switch mirror,
`Reset palette personalization` scoped to the workspace with
confirmation + post-reset feedback.

### Signal map (finalized by the production-parity pass)

- **destructive** → danger `#E0635A` (`--danger`) text + glyph, danger
  confirm button (`Button` destructive: `--danger-tint` plate,
  `--danger` label, no border)
- **attention / needs-you** → the existing badge→tone dictionary, no
  second map
- **extension-source chip** → info `#8E8EB5` (`--info`) with the
  extension name (`Pill xs info mono`)
- **success feedback** → `#5FBF85` (`--success`) glyph + label
- **pending** → motion token (`StatusDot` accent + pulse), never a
  progress percentage
- **running** → `SessionBadgeGlyph`: the `--accent-tint` roundel with
  `--accent` glyph, the WHOLE glyph pulsing, state word
  `--accent-strong`. This reverts the round-2 `--badge-fill` roundel,
  which moved the boards away from production. The accent budget is
  held by keeping the pulse the only accent motion in the row.
  `.sig[data-k="static"]` keeps its neutral `--badge-fill` plate —
  the vault glyph has no production status behind it.
- **selection** → `--elevated` raise + `--fg-strong` label and glyph
  (the production `CommandItem` `data-selected` recipe). No light rim:
  `--highlight` is a button/pill top rim and never marks a row, tile
  or chip. The `--row-selected` + rim pair belongs to `ListingRow`
  lists, not the palette. The inset-accent marker stays retired.

Color is never the only channel — tone + glyph + literal state word,
always.

### Chord tiers (three, each mapped)

1. **Row chord** → `CommandShortcut`: one `.pal-chord` text span
   carrying the whole chord, mono `--text-badge`, `--tracking-mono`,
   `--faint`. No plate, no border, no per-key caps.
2. **Footer hint** → `Kbd`: one `.key` per hint carrying the whole
   chord (`↑↓`, `⌘⇧G`, `esc`) — h20 · min-w20 · radius-sm ·
   `--canvas-soft` fill · mono `--text-mono-id` 510 · `--muted`.
   No border, no cast shadow. Footer = `OsPaletteFooter`.
3. **Settings binding** → `ShortcutBindingKeys`: bordered caps WITH a
   `--line` border on `--canvas` — **one cap per binding carrying the
   whole chord** (`⌘⇧G`, `⌥⇧N`; production maps over bindings, not
   keys — never split a chord into `⌘ ⇧ G` triplets). Alternate
   binding (index > 0) = dashed `--line-strong` on transparent;
   overridden = accent-dim border with `--accent-strong` label.
   Settings is the only surface where a chord is edited.

Prototype-local note: board footers push the esc cluster right with the
herdr `.pal-foot-flex` spacer; production `OsPaletteFooter` uses
`ml-auto` on the esc hint — same read, implementation uses `ml-auto`.

## Spatial & tonal grammar (round 4 — binding)

Anti-cockpit direction (PRODUCT.md): calm and legible for people who
are not terminal operators. Values are the FORWARD contract —
implementation adopts them; production's current 30/32px density is
anatomy precedent only.

- **Zones.** Head (nav row + field) · results · footer, separated by
  `--line-soft` hairlines. Head pads `12px 8px 10px`; results
  `4px 8px 12px`; footer `12px 20px`.
- **One 20px rail.** Every leading glyph, group label, banner glyph,
  and footer key lands its left edge 20px from the panel edge
  (zone pad-x 8 + element pad-x 12; crumb row 8 + back control 8).
- **Ladder.** Input box 40px (query voice 13px) · command rows 40px ·
  entity/two-line rows 48px · panel rows 36px · args pills 32px ·
  leading icons 16px · roundels 18px.
- **Blocks separated by full-bleed rules (round 5, reference-
  approved).** Each group after the first opens with an edge-to-edge
  `--line-soft` divider (the results well cancels its inline pad);
  16px headroom above the label, 8px below, 2px between sibling
  rows. Group labels recede to `--faint` — a step darker than row
  text, never the same tone as the menus. The action panel's
  sections carry the same dividers. Bands render as inset chips
  (`margin 10px 8px 0`, radius-md), never full-bleed dams.
- **Args fields = the query-box grammar (round 5).** 40px
  `--canvas-tint` boxes on the same border/radius as the input box;
  inline labels are quiet normal-case `--text-form-label` `--subtle`
  (the uppercase inline chip read as cockpit texture), values 13px.
  The args nav row (command glyph + name) rides the crumb-row
  recipe on the 20px rail.
- **Confirmation = Dialog anatomy (round 5).** Title =
  `DialogTitle` (`--text-item-title` 15 · 510 · tracking-tight);
  body = `DialogDescription` (small-body, `--muted`); actions live
  in a `DialogFooter` band — full-bleed `--canvas-tint` behind a
  `--line` rule, right-aligned `Button` neutral + destructive —
  never floating in the body. Palette-hosted confirms only; the
  settings-inline reset confirm keeps the light inline action row.
- **Panel ground.** `--cp-panel` = `color-mix(in oklab, --canvas 55%,
  --canvas-soft)` — one step BELOW the window-chrome ramp so the
  palette separates tonally from the OS behind it; nested popups
  (action panel, dropdowns) share it. Field fills stay
  `--canvas-tint`; selection stays `--elevated`.

## Depth grammar

**The palette is flat** (`DESIGN.md` §5 + the live palette). Glass and
backdrop-blur belong to OS-shell chrome — the menubar and dock — and
never to the palette, action panel, tooltips or dropdowns. The one
sanctioned blur in this set is the 3px scrim behind the palette.

| Surface | Recipe |
| --- | --- |
| Palette panel (`.palette`) | opaque `--canvas-soft` · no border · `--radius-lg` (10) · `--shadow-overlay` · 4px pad (`Command` root `p-1`) · no backdrop-filter |
| Backdrop (`.palette-overlay`) | `--overlay-scrim` + `blur(--overlay-blur)` (3px) · top 9vh, 16vh at ≥960px |
| Action panel (`.pal-act`) | opaque `--canvas-soft` · no border · `--radius-lg` · `--shadow-hairline` · 4px pad (PopoverContent / DropdownMenu) |
| Dropdown (`.pal-dd`) | `--canvas-soft` · no border · `--radius-md` · `--shadow-hairline` · 4px pad (Select / Combobox popup) |
| Tooltip (`.pal-tip`) | `--canvas-soft` · `--radius-md` · px12/py6 · `--text-form-label` `--fg-strong` · `--shadow-hairline` |
| Rows / tiles selected | `--elevated` + `--fg-strong` — never `--row-selected` + `--highlight` |
| Focus | `--focus-ring` on `:focus-visible` / `.focus-ring` only; fields add `--line-strong` on focus-within |

Every radius comes from the production ladder (3 / 4 / 5 / 6 / 8 / 10 /
pill); the 12–14px glass radii are retired from this set.

## Production component map

Authoritative board-class → production mapping. Every class is either a
shipped `@compozy/ui` primitive, a `web/src/systems/os` domain composite,
or a declared gap. Implementation tasks read this table, not the CSS.

| Board class | Production mapping | Kind |
| --- | --- | --- |
| `.palette` / `.palette-overlay` | `CommandDialog` (`DialogContent unframed` + scrim) + `Command` root | primitive |
| `.pal-input-box` (new) | `CommandInput` group (`data-slot=command-input-group`) | primitive |
| `.pal-scope` in input box | trailing chip slot — SearchInput `kbd`-slot precedent + `Pill md` | gap → domain (`PaletteScopeChip`) |
| `.pal-ghost*` | `PaletteGhostText` (domain, no primitive) | domain composite |
| `.pal-group` | `CommandGroup` heading (eyebrow) | primitive |
| `.pal-item` / `.pal-act__row` / `.pal-dd__row` | `CommandItem` / `DropdownMenuItem` / `ComboboxItem` | primitive |
| `.pal-item--sub`, `.pal-srow` | `CommandItem` + domain row body (`os-palette-session-row` grammar) | domain composite |
| `.pal-chord` (rows) | `CommandShortcut` | primitive |
| `.pal-foot .key` | `Kbd` | primitive |
| `.cp-reg .keys .key`, `.sc-row .key` | `ShortcutBindingKeys` (web/os) | domain composite |
| `.pal-foot` | `OsPaletteFooter` | domain composite |
| `.pal-crumbs` / `.pal-crumb` | `os-palette-breadcrumb` grammar | domain composite |
| `.pal-tip` | `Tooltip` / `TooltipContent` | primitive |
| `.pal-act` | `Popover` / `DropdownMenu` hosting a nested `Command` | primitive |
| `.pal-dd` | `Combobox` / `Select` popup (`CommandSelectShell`) | primitive |
| `.pal-args` / `.pal-arg` | `PaletteArgsBar` (domain; fields = Input/SearchInput grammar) | domain composite |
| `.pal-confirm*` | `PaletteConfirmation` (domain; buttons = `Button` neutral/destructive) | domain composite |
| `.pal-check` | `Checkbox` | primitive |
| `.pal-pick` | `InputGroup` / `Input` | primitive |
| `.pal-field*` | `Field` / `FieldLabel` / `FieldDescription` / `FieldError` | primitive |
| banner lane (`.pal-sec-err`, `.pal-form__fail`, `.cp-conflict`, `.cp-ext__health`) | `Alert` (compact palette variant) | primitive + delta |
| `.pal-skel*` | `Skeleton` / `SkeletonRows` | primitive |
| `.pal-pend` | `StatusDot` accent + pulse / `Spinner` | primitive |
| `.pal-frame*`, `.pal-preview-none`, `.pal-none--*` | `Empty` (compact) / `CommandEmpty` | primitive |
| `.pal-split*`, `.pal-kv*` | `SplitPane` + `PropertyRow` / `MetadataList` | primitive |
| `.pal-tile*`, `.pal-grid`, `.pal-gsec*` | `PaletteGridView` (domain; states per CommandItem) | domain composite |
| `.pal-band*`, `.pal-reload` | `PaletteViewBand` (domain) + `Pill sm success` | domain composite |
| `.pal-src` | `Pill xs info mono` | primitive |
| `.pal-chip` | `Pill md neutral` (active = elevated) | primitive |
| `.cp-toolbar` | `ListingToolbar`-style row (domain) | domain composite |
| `.cp-reg` | `Table` (+ `window-manager-shortcut-table` extension) | primitive |
| `.cp-alias` | `Input` (compact) — `AliasCell` domain | domain composite |
| `.cp-cmd__id` | `MonoId` | primitive |
| `.cp-ext*` | `ListGroup` + `SettingRow` grammar — `ExtensionPalettePanel` | domain composite |
| `.cp-fb` | success `StatusLine` voice (glyph + word) | domain composite |
| `.sig` | `SessionBadgeGlyph` (18px tinted roundel) | domain composite |

### Retired class vocabulary

- **Per-key `.pal-k` chips inside command rows** — a bound row renders
  ONE `.pal-chord` text span. `.pal-k` survives only as a neutralizing
  reset for boards mid-migration.
- **Per-key `.pal-foot .key` chips** — one `.key` per footer hint
  carries the whole chord. Per-key caps live only in settings.
- **The dashed-underline resting affordance on `.cp-alias`** — the
  production `Input` reads editable through its border box.
- **Glass on `.palette` / `.pal-act` / `.pal-tip`**, and every
  `--highlight` rim on a row, tile or chip.
- **The round-2 `.sig[data-k="running"]` `--badge-fill` override.**
- **The recorder's JS identity swap** (`keys` → `pill`) — the trigger
  now takes an `.is-recording` class and keeps its identity.
- `--cp-args-h` renamed `--cp-arg-h`; the mobile `.pal-head--args`
  height override (the palette head is auto-height at every width).

### Authorized deltas from production

- **Disabled rows keep color-based dimming**, not `opacity-50`.
  BR-8 mandates an AA-legible verbatim reason in `.pal-hint`, and 50%
  opacity drags it below AA. Implementation needs
  `data-[disabled=true]:opacity-100` plus color overrides on
  `CommandItem`.
- **`.pal-kv` label gutter is 72px**, not `--width-kv-label` (140) —
  140px inside a 208px detail rail leaves no room for the value.
- **The breadcrumb back control is a 24px button with a 16px glyph.**
  Production folds the `corner-up-left` mark into the breadcrumb nav
  at 12px; the boards keep an interactive control that meets the
  target-size floor. The 12px size applies to a glyph rendered inside
  `.pal-crumbs`.
- **`.pal-arg__lab` is a compact eyebrow at `--text-badge`.** A 12px
  `Field` label does not fit a 28px `--height-search` pill; the label
  keeps the eyebrow voice one step down.
- **Loop animations stay off the `--dur-*` ramp** (shimmer 1.6s, band
  sweep 1.2s, pulse 1.8s). The ramp tops out at 200ms and describes
  state feedback, not continuous cadence.

## Lab layout

Lab pages run full-viewport (authorized delta vs the herdr 960px
scaffold — operator direction 2026-08-16). Staged fragments render at
production content width. The palette renders pixel-true at
`min(560px, 92vw)`. VC captures at 1440×900, normative, dark-only.

## Shared data story

Workspace `acme` (global scope adds `payments`, `infra`, `notes`);
operator `pedro`; catalog scale `142 commands · 96 available`;
extension `ext.notes` labeled `Notes (ext.notes)`; session rows reuse
the herdr story (`Refactor session store` waiting-for-input · claude ·
2m, `Fix payment retries` running · claude, `Release notes draft` done
· hermes · 26m); clients `cl_7f21` (shell) / `cl_a09c` (browser);
approval `apr_55e0c9`; note `nt_9a01d2`; chords ⌘K palette (alt ⌘⇧P) ·
⌘E Sessions view · ⌘N New session · ⌘⇧G Global scope ·
⌥⇧N Capture note · ⌘⇧Space global summon; tasks counts All 12 ·
Queued 3 · Running 4 · Needs-approval 2 · Done 3.

## Files

Each board = final surface (§01) + states lab. `index.html` is the set hub.

| Board | Surfaces | Status |
| --- | --- | --- |
| `command-palette-root.html` | S1 rest, first-run, query + ghost, async sections, global scope | delivered |
| `command-palette-root-states.html` | S1 edge/failure + S10 destination + S14 pending | delivered |
| `command-palette-view-shell.html` | S2 stack chrome + frames | delivered |
| `command-palette-view-bands.html` | US-039 latency bands | delivered |
| `command-palette-domain-list.html` | S3 list exemplar + S4 detail pane | delivered |
| `command-palette-form-grid.html` | S5 form + S6 grid | delivered |
| `command-palette-action-panel.html` | S7 | delivered |
| `command-palette-args-confirmation.html` | S8 args + S9 confirmation | delivered |
| `command-palette-settings.html` | S12 shortcuts + global hotkeys | delivered |
| `command-palette-settings-palette.html` | S15 palette settings + S16 extensions detail | delivered |

`command-palette.css` opens with chapter 0 (the production-parity token
lane, the lab-scaffold deltas, and the cross-board chrome recipes:
palette panel, scrim, input box, results well, eyebrow lane, command-row
lane, chord tiers, footer, banner lane, empty lane, chip lane), then
holds chapters 1–12 (1 root results, 2 destination, 3 view-stack chrome,
4 bands, 5 domain list + detail, 6 form, 7 grid, 8 action panel, 9 args,
10 confirmation, 11 settings, 12 extensions + palette settings) and the
chapter 13 review appendix. New feature runs append after the marked
append point; parity and bug fixes edit chapters in place.

Iterate on these files; don't regenerate. Implementation tasks cite the boards as visual contracts — artboard CSS is a contract, never a stylesheet to import.
