# Command palette — root, views, input flows, settings

Design contract for the surfaces in `.compozy/tasks/command-palette/_uiux.md`,
delivered as ten boards in this folder. Companions: `_spec.md` (behavior
authority), `_user_stories.md` (ACs/ECs). Binding visual lineage: the
herdr-parity palette grammar (this set extends it). This file is the locked
semantic contract — every ghost tail, hint slot, and confirm verb on the
boards traces back here.

## Locked decisions

### Root & results (S1 · S10 · S14)

Group precedence is Pinned → Recents → context group → curated groups.
Entities never interleave into the command group.

- **Ghost tail** renders only for a high-confidence top result, after the
  caret, preserving typed casing. `→` at the end of the input accepts.
- **Every bound row** carries a chord badge (`.pal-chord` / `.pal-k`, not
  the footer `.key` scale).
- **Alias** renders `Title (alias)` (e.g. `Capture note (cap)`).
- **Settings rows** read `Settings → {page}`. **App rows** read `Open {app}`.
- **Workspace label** is a sub-line suffix (herdr `.pal-srow` precedent)
  plus ONE widened globe scope chip — current-scope rows carry no label.
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

### Signal map (finalized by this pass)

- **destructive** → danger `#E0635A` (`--danger`) text + glyph, danger
  confirm button
- **attention / needs-you** → the existing badge→tone dictionary, no
  second map
- **extension-source chip** → info `#8E8EB5` (`--info`) with the
  extension name
- **success feedback** → `#5FBF85` (`--success`) glyph + label
- **pending** → motion token
- **selection** → neutral plate `--row-selected` + top light edge
  (inset-accent marker stays retired)

Color is never the only channel — tone + glyph + literal state word,
always.

## Glass grammar

Inherited from herdr DESIGN-NOTES: shell-glass floating chrome
(`--shell-glass-pop`, blur 30–34px, 12–14px radii, `--shadow-overlay` +
top light edge) is reserved for floating chrome (palette, action panel).
Window content stays on the 3–10px production radius scale.

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

`command-palette.css` holds chapters 1–12 (1 root results, 2 destination,
3 view-stack chrome, 4 bands, 5 domain list + detail, 6 form, 7 grid,
8 action panel, 9 args, 10 confirmation, 11 settings, 12 extensions +
palette settings). Later runs append after the marked append point —
they never restyle earlier chapters.

Iterate on these files; don't regenerate. Implementation tasks cite the boards as visual contracts — artboard CSS is a contract, never a stylesheet to import.
