# Settings › Layouts — implementation spec

Work order to land `settings/settings-layouts.html` in `web/`.
Design rationale and field map: `LAYOUTS-REDESIGN-SPEC.md` (read it first — this document does not repeat the "why").

Status: **draft, not started.**
Owning surface: `web/src/systems/settings/` + `web/src/routes/_app/settings/layouts.tsx`.
Breaking change: yes — 11 components are deleted outright (§4). No compat shims, no aliases; this is a hard cut per the greenfield rule.

---

## 1. The load-bearing discovery — do not write geometry code

**The projector this redesign needs already exists and is already shipping in the OS shell.**

| Existing | What it gives the settings canvas |
|---|---|
| `web/src/systems/os/lib/layout-projection.ts:371` — `projectLayout(input): LayoutProjection` | pixel rects for every window, stack and **seam** on a desktop, from the same normalized document the daemon stores |
| `ProjectedSeam` (`os/lib/window-manager-types.ts`) | per-boundary `{ id, splitId, boundaryIndex, orientation, rect, value, minValue, maxValue, axisSpan, leadingWeight, trailingWeight }` — a complete drag contract with the identity `layout.resize` uses |
| `web/src/systems/os/lib/seam-preview.ts:23` — `seamWeightDelta(seam, deltaPx)` | pixel delta → weight delta, clamped by the projection minimums **and** the daemon's `MIN_SPLIT_WEIGHT` floor, mirroring `reducer_layout_resize.go` |
| `seam-preview.ts:36` — `applySeamPreviewToDesktop(desktop, seam, deltaPx): LayoutDesktop` | pure, immutable, returns a new desktop with the boundary moved and the weight vector still summing to 1 |
| `os/lib/layout-projection.ts:407` — `clampFloatingRect(input)` | keeps a dragged floating window inside the work area, preserving the title-bar grab relationship |

Consequences, and these are not optional:

1. **The settings canvas renders `projectLayout()` output.** It does not compute its own rects. This is what makes the preview *truthful* — it is pixel-identical to what the runtime will render, including the `adaptive-stack` and `minimum-unmet` diagnostics, which the settings editor has never shown.
2. **Seam drags call `applySeamPreviewToDesktop`.** Bug D-2 (§3) then cannot recur, because that function is already a mirror of the reducer and preserves the sum-to-1 invariant by construction. Do not hand-roll pairwise rebalancing in `systems/settings/`.
3. **`useLayoutSeam` (`os/hooks/use-layout-seam.ts`) is NOT reusable as-is** — it dispatches live `layout.resize` commands against the runtime store. Settings edits a *draft document*. Reuse the two pure functions from `seam-preview.ts`; write a separate `use-layout-draft-seam.ts` that applies them to the draft instead of dispatching.
4. `projectLayout` currently lives under `systems/os/`. Settings importing from `systems/os/lib/` is a cross-system import — check `make lint` boundaries before writing the import. If the boundary rule forbids it, the fix is to **move** `layout-projection.ts`, `seam-preview.ts`, `window-manager-types.ts` and `window-manager-schemas.ts` into a shared `systems/window-manager/` module consumed by both `os` and `settings` — not to duplicate them. Duplicating geometry is a blocking failure.

---

## 2. Current inventory

| File | LOC | Fate |
|---|---:|---|
| `routes/_app/settings/layouts.tsx` | 11 | keep |
| `routes/_app/settings/-layouts-settings-page.tsx` | 167 | rewrite (composition only) |
| `settings/components/window-manager-layout-document-editor.tsx` | 220 | **delete** → canvas + review bar |
| `settings/components/window-manager-layout-node-editor.tsx` | 208 | **delete** → inspector + `layout-mutations.ts` |
| `settings/components/window-manager-layout-profiles.tsx` | 152 | **delete** → profile grid + card + editor |
| `settings/components/window-manager-behavior-fields.tsx` | 109 | **delete** → behavior picks |
| `settings/components/window-manager-geometry-fields.tsx` | 105 | **delete** → gap editor + snap map + ratio track |
| `settings/components/window-manager-binding-fields.tsx` | 94 | **delete** → snap map (zones) + shortcut table + advanced |
| `settings/components/window-manager-config-editor.tsx` | 53 | rewrite (drop the bespoke sticky bar) |
| `settings/components/window-manager-number-field.tsx` | 40 | **delete** |
| `settings/components/window-manager-select-field.tsx` | 40 | **delete** |
| `settings/components/window-manager-toggle-field.tsx` | 25 | **delete** |
| `settings/components/window-manager-config-field-types.ts` | 13 | **delete** if unreferenced after the above |
| `settings/components/window-manager-config-fields.ts` | 3 | **delete** if unreferenced after the above |
| `settings/hooks/use-window-manager-config-editor.ts` | 118 | keep, extend (§6) |
| `settings/hooks/use-window-manager-layout-editor.ts` | 121 | keep, extend (§6) |
| `settings/hooks/use-window-manager-layout-profiles.ts` | 136 | keep, fix (§3 D-4) |
| `settings/lib/window-manager-layout-{types,schema,tree,projection,query}.ts` | 131/404/19/88/72 | keep unchanged |
| `settings/adapters/window-manager-layouts-api.ts` | 262 | keep unchanged |
| `settings/mocks/window-manager-fixtures.ts` | 91 | **rewrite** — currently contains invalid enum values (§3 D-3) |

Everything else in `systems/settings/` is untouched.

---

## 3. Fix these before building the UI

Four defects, each independently reproducible today. **D-1 and D-2 must land first** — the redesign's structure assumes they are fixed.

### D-1 · The split buttons are inverted
`window-manager-layout-node-editor.tsx:79` maps `Split rows → axis "horizontal"`, `:87` maps `Split columns → axis "vertical"`. `layout-projection.ts:190` divides the **width** on `horizontal`, so `horizontal` produces columns. "Split rows" currently produces columns.
**Fix:** the axis vocabulary leaves the UI entirely. The inspector exposes two diagram buttons — **Rows** → `axis: "vertical"`, **Columns** → `axis: "horizontal"`. Add a regression test asserting the mapping against `projectLayout` output, not against the label.

### D-2 · Every split created in Settings fails validation
`window-manager-layout-node-editor.tsx:41` emits `weights: windowIds.map(() => 1)`. `validate.go:249` requires `|Σweights − 1| ≤ 1e-6`, and `validateLayoutDocument` (`internal/windowmanager/service_layout.go:40-74`) runs `ValidateSnapshot` on the document **as sent**, with no normalization pass. A two-way split therefore sends `[1,1]` and "Validate and preview" returns `topology.split_weight_sum`.
**Fix:** `toSplit` emits `1/n`. Structural conversions move into `settings/lib/window-manager-layout-mutations.ts` (§5) so the invariant has one owner and one test.

### D-3 · The Storybook fixture asserts enum values that do not exist
`settings/mocks/window-manager-fixtures.ts` declares `aspect_variant: "square" | "wide"` and `overflow_policy: "floating" | "ignore"`, and sets document `version: 1`. The real contract is `any|landscape|portrait`, `stack|reject`, and `SnapshotVersion = 2` (`internal/windowmanager/types.go:9`, enforced at `resource_codec.go:174-180`).
**Fix:** rebuild the fixture from the zod schemas. It should be dense enough to exercise the canvas: ≥ 2 desktops, ≥ 2 groups, a nested split, a stack, a floating window and a minimized window. The current fixture (1 desktop / 1 group / 1 leaf) is why the density problem was never visible in review captures.

### D-4 · Loading a profile silently discards unapplied edits
`use-window-manager-layout-profiles.ts:92-103` — `selectProfile` calls `onLoad(structuredClone(record.spec.document))`, replacing the whole draft with no confirmation. `remove` (`:75-79`) also deletes with no confirmation.
**Fix:** `Load` is confirmed when `editor.dirty` is true; `Delete` uses `ConfirmDialog` from `@agh/ui`. Also fix the reset asymmetry at `:80-89` — after a delete it clears `id`/`displayName` but leaves `aspect`/`overflow`/`scope` on the deleted record's values.

---

## 4. Delete targets

Removed in the same change, with no deprecation window:

```
web/src/systems/settings/components/window-manager-layout-document-editor.tsx
web/src/systems/settings/components/window-manager-layout-node-editor.tsx
web/src/systems/settings/components/window-manager-layout-profiles.tsx
web/src/systems/settings/components/window-manager-behavior-fields.tsx
web/src/systems/settings/components/window-manager-geometry-fields.tsx
web/src/systems/settings/components/window-manager-binding-fields.tsx
web/src/systems/settings/components/window-manager-number-field.tsx
web/src/systems/settings/components/window-manager-select-field.tsx
web/src/systems/settings/components/window-manager-toggle-field.tsx
web/src/systems/settings/components/window-manager-config-field-types.ts   ← if unreferenced
web/src/systems/settings/components/window-manager-config-fields.ts        ← if unreferenced
web/src/systems/settings/components/__tests__/window-manager-layout-document-editor.test.tsx
```

Plus their entries in `settings/components/index.ts`. The three thin wrappers (`window-manager-{number,select,toggle}-field`) duplicated `SettingsNumberInput` / `SettingsChoiceGroup` / a bare `Switch` and must not be reintroduced.

`window-manager-layout-document-editor.test.tsx` is deleted rather than migrated because it asserts the DOM of a component that ceases to exist. Its behavioural coverage moves per §9.

---

## 5. New files

Directory: `web/src/systems/settings/components/layouts/`. Hard cap 500 lines per file; the budgets below are targets, and any file that grows past its budget splits rather than absorbs.

### Canvas
| File | Responsibility | ~LOC |
|---|---|---:|
| `layout-canvas.tsx` | consumes `projectLayout()`, positions groups/tiles/seams/floats, owns selection | 190 |
| `layout-canvas-tile.tsx` | one projected window or stack, presentational only | 110 |
| `layout-canvas-seam.tsx` | seam handle + detent tooltip; delegates math to `seam-preview.ts` | 120 |
| `layout-canvas-group-edge.tsx` | four group edge handles, overlap flag | 130 |
| `layout-canvas-floating.tsx` | floating/minimized window, drag via `clampFloatingRect` | 110 |
| `layout-desktop-tabs.tsx` | desktop tab strip, click-to-rename | 90 |

### Inspector + review
| File | Responsibility | ~LOC |
|---|---|---:|
| `layout-inspector.tsx` | routes on selection kind, renders the empty/desktop summary | 130 |
| `layout-inspector-node.tsx` | Rows/Columns/Stack actions, weight readout, Distribute evenly | 150 |
| `layout-inspector-window.tsx` | leaf window picker + stack active picker, "in use" gating | 120 |
| `layout-review-bar.tsx` | dirty → Review → Apply gate, preview counts, diagnostics list | 140 |

### Profiles
| File | Responsibility | ~LOC |
|---|---|---:|
| `layout-profile-grid.tsx` | card grid + load confirm + delete confirm | 130 |
| `layout-profile-card.tsx` | card, chips, computed thumbnail | 110 |
| `layout-profile-editor.tsx` | 5 fields (name, id, scope, aspect, overflow) + derived slot count | 150 |

### Global config
| File | Responsibility | ~LOC |
|---|---|---:|
| `window-manager-behavior-picks.tsx` | the 5 diagram pick rows + 2 modifier keycap rows + 3 switch rows | 150 |
| `window-manager-behavior-diagrams.tsx` | the SVG set as data; no logic | 150 |
| `window-manager-gap-editor.tsx` | 1:1 box model, 6 handles, keyboard steppers | 170 |
| `window-manager-snap-map.tsx` | edge bands, corner reach, exit slack, 2 centre-zone bindings | 200 |
| `window-manager-ratio-track.tsx` | repeat-width stops: add / drag / remove / uniqueness | 160 |
| `window-manager-shortcut-table.tsx` | grouped rows, disclosure, conflict banner | 140 |
| `window-manager-shortcut-row.tsx` | label + tiling diagram + chord chip + reset | 100 |

### Hooks — `web/src/systems/settings/hooks/`
| File | Responsibility | ~LOC |
|---|---|---:|
| `use-layout-canvas-selection.ts` | selection state + `findNode`/`replaceNode` over the draft | 110 |
| `use-layout-draft-seam.ts` | pointer drag → `applySeamPreviewToDesktop` on the draft (**not** a `layout.resize` dispatch) | 120 |
| `use-layout-group-frame-drag.ts` | edge drag → normalized frame, detents, overlap detection | 140 |
| `use-window-manager-shortcuts.ts` | effective chord resolution, recording, conflict map | 150 |

### Lib — `web/src/systems/settings/lib/`
| File | Responsibility | ~LOC |
|---|---|---:|
| `window-manager-layout-mutations.ts` | `toSplit` / `toStack` / `evenWeights` — **the single owner of the weight-sum invariant** | 90 |
| `window-manager-shortcut-chords.ts` | parse / format / canonicalize / symbolize / detect conflicts | 130 |
| `window-manager-layout-thumbnail.ts` | profile-card rect math (no gaps, no minimums — a diagram, not a projection) | 80 |
| `window-manager-snap-geometry.ts` | px ↔ map-scale conversion for the snap map and gap editor | 70 |

### `packages/ui`
`Slider` does not exist. `packages/ui/src/components/` has `popover.tsx`, `switch.tsx`, `tabs.tsx`, `tooltip.tsx`, `scroll-area.tsx` — no slider. The `history_limit` control needs one, and a slider is a generic primitive, so per the reuse rule it lands in `packages/ui` with a story and a test, not in `web/`:

```
packages/ui/src/components/slider.tsx
packages/ui/src/components/__tests__/slider.test.tsx
web/.storybook/... or packages/ui story colocated per the existing convention
```

Everything else composes existing primitives: `Button`, `Input`, `Switch`, `Popover`, `Tooltip`, `ConfirmDialog`, `Field*`, `Eyebrow`, `Item*`, `Spinner`.

---

## 6. Hook changes

`use-window-manager-layout-editor.ts` — add:
- `setDesktopName(desktopIndex, name)`
- `setNode(nodeId, next)` — replaces a node anywhere in the active desktop
- `setFloatingRect(windowId, rect)` — the document contract already validates `floating_rect`; the current editor simply never wrote it
- `applySeam(seam, deltaPx)` — thin wrapper over `applySeamPreviewToDesktop`

Keep `reviewCurrent` exactly as it is. The fingerprint gate is correct and the redesign depends on it.

`use-window-manager-config-editor.ts` — replace the current all-or-nothing `canSave` with per-field validity so the UI can surface *which* value is out of range. Today the only feedback is `aria-invalid` plus a disabled button, and the explanatory string (`"Fix repeat ratios and shortcut JSON before saving."`) is thrown from a code path that can only run after a click that cannot happen.

`use-window-manager-layout-profiles.ts` — add the confirm gates from D-4; fix the post-delete reset.

---

## 7. Page composition

`-layouts-settings-page.tsx` becomes composition only (~120 LOC), rendering:

```
SettingsPageFrame (needs a width beyond `wide`; see below)
├── LayoutStage            ← desktop tabs + LayoutCanvas + LayoutInspector + LayoutReviewBar
├── LayoutProfileGrid + LayoutProfileEditor
├── SettingsGroup "Window behavior"   → WindowManagerBehaviorPicks
├── SettingsGroup "Spacing and snapping" → WindowManagerGapEditor | WindowManagerSnapMap | WindowManagerRatioTrack
├── SettingsGroup "Shortcuts"        → WindowManagerShortcutTable
├── SettingsAdvancedFold             → history Slider + SettingsTiles (provenance)
└── SettingsSaveBar                  ← the shared one, for the global config draft only
```

Two structural fixes here:

- **One floating bar.** Today `window-manager-config-editor.tsx:34-50` and `window-manager-layout-document-editor.tsx:183-217` are both `sticky bottom-3` in the same scroll region, so two bespoke bars can float on top of each other. In the redesign the document's review bar is **docked inside the stage card** (not sticky) and only the global config uses the shared `SettingsSaveBar` — matching every other settings section's dirty/saved dot and `Discard` / `Save changes` wording.
- **Column width.** `settings-page-frame.tsx:17,46` exposes a boolean `wide` that picks between the `max-w-settings-page-form` and `max-w-settings-page-wide` Tailwind tokens. The canvas + 236px inspector needs ~1040px, so the boolean becomes a `width?: "form" | "wide" | "canvas"` union with a matching `max-w-settings-page-canvas` token — not a bespoke `className` on the call site. Migrate the existing `wide` call sites (Providers, and Layouts itself) in the same change. Below 1100px the inspector stacks under the canvas.

---

## 8. Backend and contract

**No daemon changes are required for the UI to work.** Every control maps to an existing field; the endpoint list is unchanged from `LAYOUTS-REDESIGN-SPEC.md` §3.

Two gaps worth deciding on separately — neither blocks this work:

1. **Layout profiles have no CLI surface.** `internal/cli/window_manager_layout.go` ships `export`, `validate`, `apply`, `watch`; `grep -rn "layout-profile" internal/cli/` returns nothing. The redesign promotes profiles to a first-class surface, so the core premise ("every capability must be manageable by agents") says the CLI should catch up: `agh window-manager layout-profile {list,get,put,delete}` over the existing `…/layout-profiles` routes. **File as its own task** — the web work does not depend on it.
2. **Per-workspace overrides have no UI.** `document.overrides` is a full `WorkspaceConfig` on the wire (`web/src/generated/agh-openapi.d.ts:6220-6252`) but `Record<string, unknown>` in the settings types, and the section declares `scope: "global"`, `available_scopes: ["global"]`. Surfacing it needs a new schema and a scope switcher, not a new component. Out of scope here; the redesign states "Global only" in Advanced so the UI does not imply otherwise.

Config lifecycle: `window_manager.*` is `DiffClassLive` (`internal/config/lifecycle/lifecycle.go:108`) — it hot-applies. The page shows `Applies live` in the head and must **not** render a restart notice.

---

## 9. Test plan

Placement first, per `consolidate-test-suites`. Every row names the invariant, the layer that owns it, and the suite it belongs in.

| Invariant | Owning layer | Canonical suite |
|---|---|---|
| `toSplit`/`toStack` always emit weights summing to 1 (D-2) | pure lib | **new** `settings/lib/__tests__/window-manager-layout-mutations.test.ts` |
| Rows→`vertical` / Columns→`horizontal` produce the geometry the projector renders (D-1) | pure lib, asserted through `projectLayout` | same suite as above |
| Chord parse/format/canonicalize round-trips; conflicts detected | pure lib | **new** `settings/lib/__tests__/window-manager-shortcut-chords.test.ts` |
| Seam drag preserves the weight sum and respects `MIN_SPLIT_WEIGHT` | already owned by `seam-preview.ts` | **existing** `os/lib/__tests__/` — extend only if a settings-specific path is added; do **not** duplicate |
| Draft fingerprint invalidates the preview on any edit | hook | **existing** `settings/lib/__tests__/window-manager-layout-query.test.ts` |
| Apply stays disabled until validate + preview match the draft | component | **new** `settings/components/__tests__/layout-review-bar.test.tsx` (replaces the deleted document-editor test) |
| Loading a profile over a dirty draft confirms first (D-4) | component | **new** `settings/components/__tests__/layout-profile-grid.test.tsx` |
| Canvas renders one tile per projected window and marks overlapping groups | component | **new** `settings/components/__tests__/layout-canvas.test.tsx` |
| Config ranges (gaps 0–64, edge band 4–128, corner reach 16–512, exit slack 0–64, ratios 1–8 × 0.1–0.9 unique, history 1–500) | hook | **existing** `settings/components/__tests__/window-manager-config-editor.test.tsx`, retargeted at the hook |
| Slider primitive keyboard + aria contract | primitive | **new** `packages/ui/src/components/__tests__/slider.test.tsx` |

Forbidden by default and **not** to be added: snapshot tests of the canvas SVG/DOM, CSS assertions, and tests that only re-assert the zod schemas. The schemas already own those.

Storybook: rewrite `settings/routes/settings-layouts.stories.tsx` against the new fixture (§3 D-3) with at least `default`, `dirty-unreviewed`, `validation-failed` and `no-workspace` stories, so the visual-contract capture actually exercises density.

---

## 10. Accessibility contract

Non-negotiable — the page is otherwise pointer-only, which would be a regression from a page made of native inputs.

- Every drag affordance (seam, group edge, gap guide, snap grip, ratio stop) is also `role="slider"` with `aria-valuemin` / `aria-valuemax` / `aria-valuenow` / `aria-label`, and responds to arrow keys (shift = ×10 on px values).
- The canvas is a `listbox`-style selection surface: tiles are focusable, arrow keys move selection, `Enter` opens the inspector's primary action.
- Centre-zone bindings open a real `Popover` menu with `role="menu"`, not a custom div.
- Shortcut recording announces state via `aria-live="polite"` and is cancellable with `Escape`.
- Conflicts and diagnostics are `role="status"` regions, not colour-only signals.

---

## 11. AGH Impact Audit

```markdown
AGH Impact Audit:

- Native tools: no impact — checked skills/agh/ tool descriptors and the agh__* registry;
  no window-manager tool IDs exist and none are added. The window-manager surface is reached
  through the CLI (internal/cli/window_manager_*.go) and HTTP/UDS routes
  (internal/api/httpapi/window_manager_routes.go), neither of which changes shape here.

- Extensibility and hooks: no impact on extensions, hooks, skills/capabilities, bundles,
  registries or bridge SDKs — checked internal/hooks event list and the extension surface;
  window-manager emits no hook events and exposes no extension point. Config lifecycle is
  unchanged: window_manager.* stays DiffClassLive (internal/config/lifecycle/lifecycle.go:108)
  and no config.toml key is added, removed or re-ranged.

- Workspace data isolation: no new data. The layout document is workspace-scoped and already
  keyed by workspace_id through the client-state engine (internal/daemon/window_manager_repository.go:17-19);
  layout profiles are scope-tagged global|workspace in resource_records with the CHECK at
  internal/store/globaldb/schema/definitions/34_resources.sql:5 and the visibility filter at
  internal/api/core/window_manager_layout_profiles.go:212-219. The redesign reads the same
  queries with the same keys (settings/lib/query-keys.ts:24-36); no new cache, SSE channel or
  event path is introduced, so no new cross-workspace leak surface exists.

- Official AGH skill: no impact — checked skills/agh/ for window-manager references; the skill
  documents no layout tool IDs, CLI paths or capabilities. If §8 item 1 (layout-profile CLI verbs)
  is taken up as its own task, that task must update skills/agh/.
```

**Web/Docs Impact.** Web: the whole of §4–§7. Docs (`packages/site`): `content/runtime/core/configuration/config-toml.mdx:133-161` documents `[window_manager]` and stays accurate — no key changes. No docs change is required for this task; if the layout-profile CLI lands, its reference page is generated and will co-ship there.

---

## 12. QA tracker impact

User-visible behaviour changes, so this is a **flag, not a retest**:

- Add `untested` content-addressed scenarios under `docs/qa/scenarios/` for: drag-to-rebalance a split; drag a group edge into an overlap and see it refused; record and reset a shortcut; edit gaps and see the canvas follow; load a saved layout over a dirty draft.
- Reset `qa_status` to `untested` on any existing scenario covering Settings › Layouts.
- D-1 and D-2 are behaviour fixes and each need their own scenario.

E2E: the page is UI-bearing, so a Playwright lane belongs in `make test-e2e-web` covering the review → apply gate. Pointer-drag assertions are brittle; assert the *outcome* (weights, diagnostics, enabled state) through the store, not the pixel path.

---

## 13. Phasing

**P1 — correctness.** D-1, D-2, D-3, D-4 with their tests. Ships against the current UI. Independently mergeable and independently valuable.

**P2 — the stage.** Shared window-manager module if the boundary check demands it (§1.4), canvas + inspector + review bar + desktop tabs, page recomposed, delete targets removed, one save bar. This is the bulk of the work.

**P3 — the config surface.** Behavior picks, gap editor, snap map, ratio track, shortcut table, `Slider` primitive, advanced fold.

**P4 — profiles.** Card grid with thumbnails, editor, confirm gates.

Acceptance for the whole change: `make verify` green; zero `input[type=number]` bound to a geometry field under `systems/settings/`; no file over 500 lines; `agh-ui-screenshot` capture of the new Storybook stories cited in the completion notes.

---

## 14. Open questions

1. **Boundary rule** — may `systems/settings/` import from `systems/os/lib/`? If not, the shared-module move in §1.4 is mandatory and should be its own commit ahead of P2. Resolve before starting P2.
2. **Group repositioning** — the prototype only resizes groups via their edges, which cannot move a group without also resizing a neighbour. Is a drag-to-move affordance wanted, and if so what happens to the vacated space? The daemon has no "swap groups" command, so any answer here is client-side rect math.
3. **Adaptive-stack diagnostics** — `projectLayout` reports `adaptive-stack` and `minimum-unmet` at the current viewport. Should the settings canvas surface them (honest: "this layout will fold at this width") or suppress them, given the canvas is a fixed-aspect reference and not the user's real viewport?
