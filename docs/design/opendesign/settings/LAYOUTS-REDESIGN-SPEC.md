# Settings › Layouts — visual redesign

Prototype: `settings-layouts.html` + `settings-layouts.js`
Replaces: `web/src/routes/_app/settings/-layouts-settings-page.tsx` and its three editor components.

---

## 1. Why

The current page flattens a spatial domain into scalars. In the *minimal* Storybook fixture (1 desktop, 1 group, 1 leaf) it renders **9 number inputs, 4 free-text inputs, 1 JSON textarea and 12 selects** — and it scales linearly with the layout: `+4 numbers per group`, `+1 number per split child`, `+1 select per node`. A modest 3-desktop workspace is ~59 typed boxes on one scroll.

Every one of those boxes is a rectangle, a ratio or an inset that got turned into a number on its way to the user:

| Today | Actually is |
|---|---|
| `x` `y` `w` `h`, four 0–1 decimals, `step=0.05` | a rectangle on a screen |
| `Weight`, a decimal in a detached 7rem column | the position of a divider |
| `Repeat ratios`, a comma-separated string | a set of stops along an axis |
| `Inner/Top/Right/Bottom/Left gap`, five px fields | a box model |
| `Edge band` / `Corner reach` / `Exit slack` | hit zones around the screen edges |
| `Top center` / `Bottom center`, two selects | two positions on the screen edge |
| `Shortcut map`, a raw JSON textarea | 24 named actions with chords |

The redesign makes each of those the thing it is. **The page contains zero `input[type=number]` for geometry.** The only remaining slider is `history_limit`, which genuinely is a scalar.

## 2. Information shape

Per the settings-section rule, Layouts leads with what no other section has: **a canvas**. The page is:

1. **Workspace layout** — a desktop canvas you manipulate directly, plus a selection inspector and the daemon's validate → preview → apply gate docked inside the stage card.
2. **Saved layouts** — profile cards whose thumbnail is their real geometry, computed from the stored document.
3. **Window behavior** — diagram choice-cards and modifier keycaps instead of 7 selects.
4. **Spacing and snapping** — a 1:1 gap box model, a snap-zone map with the controls sitting where the zones are, and a track of repeat-width stops.
5. **Shortcuts** — a grouped recorder table with tiling geometry icons and live conflict detection.
6. **Advanced** — history limit + provenance.

## 3. Field map — every control to its contract

### Layout document (`PUT …/window-manager/layout`, gated by validate + preview)

| Control | Writes | Rule enforced by the control |
|---|---|---|
| Drag a group edge | `desktops[].groups[].frame.{x,y,w,h}` | clamped to `x,y ≥ 0`, `w,h ≥ 0.08`, `x+w ≤ 1`; magnetic detents at 0, ¼, ⅓, ½, ⅔, ¾, 1; live overlap flag (`topology.group_overlap`) |
| Drag a seam | `…root…weights[]` | the pair is rebalanced, so the vector **always sums to 1** — `topology.split_weight_sum` becomes unreachable |
| Rows / Columns / Stack | replaces the node via the same flatten-and-rebuild as `toSplit`/`toStack` | disabled below 2 windows; emits normalized weights |
| Distribute evenly | `weights[] = 1/n` | — |
| Window picker on a leaf | `leaf.windowId` | windows already claimed by another node are disabled (`topology.window_membership`) |
| Member list on a stack | `stack.activeId` | warns below 2 members (`topology.stack_size`) |
| Desktop tab, click-to-rename | `desktops[].name` | — |
| Drag a floating window | `windows[].floatingRect` | clamped inside 0–1 (`topology.floating_rect`) |
| Import / Export JSON | whole document | import lands in the draft and still passes the review gate |
| Review changes | `POST …/layout/validate` then `POST …/preview` | preview counts come from `changes.{desktopIds,groupIds,nodeIds,windowIds}` — the contract returns ids and counts, never geometry, so the panel shows counts only |
| Apply layout | `PUT …/layout` with `expected_revision` | stays disabled until the preview fingerprint matches the current draft, exactly as `reviewCurrent` does today |

### Layout profiles (`window_layout` resource)

| Control | Writes |
|---|---|
| Card → Load | replaces the draft document (now behind a confirm when the draft is dirty) |
| Name / Resource ID | `spec.display_name` / `spec.id` — the hint states that changing the id forks a new record, because `expectedVersion` resets to 0 |
| Scope segmented | `scope.kind` ∈ `global \| workspace` |
| Screen shape segmented | `spec.aspect_variant` ∈ `any \| landscape \| portrait` — labelled as stored-only, because nothing selects a profile by it |
| Overflow cards | `spec.overflow_policy` ∈ `stack \| reject` |
| Footer sentence | `spec.participant_slots`, derived from the document — shown, never edited |

### Global config (`PATCH /api/settings/window-manager`, one save bar)

| Control | Writes | Range |
|---|---|---|
| 5 diagram pick rows | `new_window_policy`, `small_viewport_policy`, `focus_policy`, `drag_away_policy`, `desktop_transition` | the literal unions |
| 2 keycap rows | `group_move_modifier`, `swap_modifier` | `alt \| control \| meta \| shift \| none` |
| 3 switches | `focus_wrap`, `focus_follows_pointer`, `raise_on_focus` | — |
| Gap box guides | `gaps.{inner,top,right,bottom,left}` | 0–64, integer |
| Snap map grips | `snap.{edge_band,corner_reach,exit_slack}` | 4–128 / 16–512 / 0–64 |
| Centre zone buttons | `bindings.{top_center,bottom_center}` | `none \| reserved \| zoom` |
| Repeat-width stops | `snap.repeat_ratios` | 1–8 stops, each 0.1–0.9, unique |
| History slider | `history_limit` | 1–500 |
| Shortcut chips | `shortcuts[actionId]` | chord = modifiers + one `KeyboardEvent.code`; only overrides are stored, so the table shows the shipped default with a reset affordance |

## 4. Deliberately not built

These do not exist in the contract and were not invented:

- **Add / remove / reorder desktops, groups or windows.** The stage says so in the tab strip. Those are `desktop.create/delete/reorder` semantic commands on `POST …/commands`, not layout-document edits.
- **Per-display / multi-monitor config.** No display entity exists anywhere in the model.
- **Auto-tiling modes (bsp, master-stack, fibonacci).** `layout.arrange` is a command with presets `horizontal | vertical | grid | stack`; it is not a stored layout property.
- **Floating rules ("app X always floats").** `new_window_policy` is a single global switch; there is no per-app rule table.
- **Per-workspace overrides.** `document.overrides` and `.agh/config.toml` exist, but the settings section is `scope: "global"`, `available_scopes: ["global"]`. Surfacing it needs a new schema, not a new component.
- **Profile auto-activation by screen shape.** `aspect_variant` is stored and nothing consumes it — the UI says so instead of implying a match rule.
- **Undo / redo in the editor.** Only `Reset`. `layout.undo/redo` are daemon commands the settings editor does not call.
- **A visual preview from the daemon.** `preview` returns ids and counts. Every rectangle on this page is computed client-side from `frame` + `weights` + `axis`.

## 5. Two production defects found while mapping the contract

**D-1 · The split buttons are inverted.**
`window-manager-layout-node-editor.tsx:79` maps `Split rows → axis "horizontal"` and `:87` maps `Split columns → axis "vertical"`. The projector divides the **width** on `horizontal` (`web/src/systems/os/lib/layout-projection.ts:190`), so `horizontal` produces columns. Pressing "Split rows" today produces columns. The redesign drops the axis vocabulary entirely and labels the two diagram buttons **Rows** (`vertical`) and **Columns** (`horizontal`).

**D-2 · Every split created in Settings fails validation.**
`toSplit` emits `weights: windowIds.map(() => 1)` (`window-manager-layout-node-editor.tsx:41`). `validate.go:249` requires `|Σweights − 1| ≤ 1e-6`, and `validateLayoutDocument` (`service_layout.go:40-74`) runs `ValidateSnapshot` on the document **as sent**, with no normalization. So a two-way split sends `[1,1]` and "Validate and preview" returns `topology.split_weight_sum`. The redesign emits `1/n` and rebalances pairwise on drag, which makes the invalid state unreachable rather than merely reported.

Both are code fixes, independent of this redesign.

## 6. Notes for implementation in `web/`

- The document review gate, the profile CRUD and the config draft keep their existing hooks (`useWindowManagerLayoutEditor`, `useWindowManagerLayoutProfiles`, `useWindowManagerConfigEditor`). Only the presentation changes.
- The page currently floats **two** competing `sticky bottom-3` bars. Here the document's review bar is docked inside the stage card and only the global config uses the shared `SettingsSaveBar` — one floating bar on the page, matching every other settings section.
- `SettingsPageFrame` needs a width beyond `wide` (960px) for the canvas + inspector; the prototype uses 1040px and collapses the inspector under the canvas below 1100px.
- New primitives worth promoting to `packages/ui` if reused: none. The canvas, gap box, snap map and chord recorder are domain composites and belong in `web/src/systems/settings/`.
- `Delete` on a profile still has no confirmation in production; the prototype confirms only the destructive *load* (which silently discards unapplied edits today). Both deserve confirmation.
- Accessibility: every drag affordance is also a `role="slider"` with arrow-key support and an `aria-valuenow`, so the page is operable without a pointer.
