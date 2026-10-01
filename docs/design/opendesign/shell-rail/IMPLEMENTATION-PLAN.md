# Shell rail v2: implementation plan

Scope: what must change in the repo to ship the shell and design system prototyped in
`shell-rail-v2.html`, as specified in `brand-spec.md` and `DESIGN-NOTES.md`.
Source: a 5-slice read-only audit run on 2026-09-29 covering tokens/theming, shell chrome, windows/tiling,
`@compozy/ui` primitives, and compatibility/QA/docs. All paths are repo-relative.

## TL;DR

- **The daemon barely changes.** Window rects persist as normalized 0–1 fractions
  (`internal/windowmanager/types.go` `NormalizedRect`), so moving the dock, adding the rail and removing gutters
  needs **no layout migration**. The only Go work:
  - Two public **config default** changes: `window_manager.gaps.*` → 0 and `bindings.bottom_center`.
  - An optional new `main_stack` arrangement.
- **The largest item is theming.** The runtime is dark-only today (`color-scheme: dark`, and nothing sets
  `.dark`). Light/dark needs:
  - Theme-scoped tokens.
  - A DESIGN.md generator that understands selectors.
  - Per-theme focus ring and contrast tests.
  - Cleanup of about 70 hardcoded literals.
  - A persisted toggle with no flash on boot.
- **The shell chrome is web-only work.** The dock becomes the rail, the menubar becomes a flat topbar, the pager
  moves into the topbar, and the dock band is removed. Electron changes one constant plus its background color.
- **Most primitives change through tokens and variant classes.** Only StateGlyph, the IconWell tone, the
  sunken Surface variant and ThemeToggle are new.
- **Compatibility:**
  - User state: nothing to migrate.
  - Public surface: two config defaults change, with no key renames.
  - Everything else is internal: delete it without aliases (SD-013 / L-040).

## Decisions needed before a spec

| # | Decision | Options | Recommendation |
|---|---|---|---|
| D1 | Does floating mode survive gutterless tiling? A borderless, shadowless floating window has no visible edge. | (a) floating keeps a 1px hairline + `shadow-elevated`; (b) default `new_window_policy` → `beside_focus` | **a + b** |
| D2 | Compact mode (<960px) | keep the bottom tab bar (needs a conditional bottom reservation) vs a collapsed rail | Keep the tab bar in v1 and drop its New Session and pager leader |
| D3 | Where the theme persists | localStorage (`light\|dark\|system`, internal) vs a new `config.toml` key (public surface, co-ship) | **localStorage** + boot script; add a config key only if agents or the CLI need it |
| D4 | Theme scope | light + dark + `system` | Include `system`; the toggle cycles or has a menu |
| D5 | Terminal palette in light mode | light xterm set vs the terminal stays dark | Terminal stays dark in v1 (documented rule) |
| D6 | Profile switcher slot | stays in the topbar tray vs moves to the rail foot | Rail foot, above the theme toggle |
| D7 | `window_manager.gaps.inner` | keep the key with default 0, or deprecate it | Keep the key and default to 0; the gap editor previews the result |
| D8 | Tab min width | 112 (notes) vs 136 (prototype) | 136 |
| D9 | Site (`packages/site`) font swap | follow Inter/Geist Mono or keep its own | Decide separately (site has its own theme) |

## Workstream A: tokens, theming and fonts (L)

**`packages/ui/src/tokens.css`** (770 lines; plain `@theme` at 15–630, `:root` at 632–770). Because it is not
`@theme inline`, utilities resolve to `var(--color-*)`, and redefining variables per theme switches everything
at runtime.
- **Theme selectors.** Add `[data-theme="light"]` and `[data-theme="dark"]` sets and flip `color-scheme` per
  theme. Set `data-theme` **and** `.dark` together, and redefine `@custom-variant dark` to match, so the
  existing `use-code-block.ts:149` detection and the 7 `dark:` utilities keep working.
- **New roles.** Map the new roles onto the ramp, as renames or aliases decided in the spec:
  - `chrome`, `surface`, `sunken`, `surface-2`, `selected`, `code-bg`
  - `border`/`border-strong`
  - `fg`/`muted`/`subtle` plus derived `fg-2`/`fg-3`
  - `primary`/`on-primary` (inverted)
  - `accent` (the accent orange: `#e8572a` dark, `#d14e25` light) for highlights and needs-you
  - success mint, `well`, `shadow-card`/`shadow-elevated`/`shadow-pop`
  - Values come verbatim from `brand-spec.md` and `shell-rail-v2.html` lines 14–72.
- **Replace dark-only alpha families** with theme-scoped values:
  - hairlines `--color-line*`, the glaze ladder, chat fills
  - data-viz ink, overlays, `--shadow-overlay`/`--highlight`
  - `--shadow-focus-ring` (50% white, which is invisible on light)
  - avatar palette, wallpapers, `--shell-plate-gradient`
- **Retire the shell carve-out:**
  - `--shell-glass*` (`:root` 645–646 and `@theme` 522–523)
  - `--radius-window/dock/dock-item`, `--shadow-window*`, `--shadow-dock`, `--shadow-shell-*` (511–537)
  - `--size-dock-band/zone-offset/clearance` (575–588)
  - `--size-traffic-light*` and `--spacing-traffic-light-gap` (563, 593)
  - 14 tsx files use glass or window/dock shadows, and 11 use window/dock radii.
- **Add shell tokens:**
  - rail width 60 and item 40; `--height-menubar` 44 → 52
  - deck tab max 208 / min 136; seam hit area 9
- **Radii:** `md` 10 → 8, `lg` 14 → 12, add pill usage. `sm` 7 already matches the icon well; the icon-well
  token is 12 today and must be fixed.
- **Type scale:** 12 / 13 / 13.5 / **14.5 body** / 15 title / 17 heading (today body is 15 and small-body 13.5).
- **Weights:** 425 / 500 / 600, replacing 400 / **510** / 600 / 700. `--font-weight-display: 620` depends on the
  Geist axis and must be re-checked. 355 `font-medium` uses shift.
- **Terminal:** `packages/ui/src/terminal-tokens.css` is a dark-only xterm palette. Apply the D5 rule.

**Fonts (S)**
- `web/src/styles.css:3-4` and `packages/ui/.storybook/preview.css:3-4`: `@fontsource-variable/geist` and
  `jetbrains-mono` → `@fontsource-variable/inter` + `@fontsource-variable/geist-mono`, added with
  `bun add`. Update `web/package.json:41-42` and `packages/ui/package.json:30,55`.
- `tokens.css:150-154` `--font-sans/mono/keys`, plus `tokens-runtime.css` (the Nerd Font fallback comment names
  JetBrains Mono; verify with QA scenario `ET-terminal-nerd-glyphs`).
- Weight 425 requires the **variable** Inter.

**DESIGN.md codegen (M)**
- `scripts/sync-design-md.mjs`, wired via `magefiles/codegen.go:177,187` and `magefiles/defaults.go:36`.
- **Blocker:** `runtime = new Map(parseDecls(file))` lets the last declaration win, so a light block would
  silently overwrite the dark tables.
  - Parse per selector and emit dark/light columns.
  - Update the hardcoded color-group stems (lines 14–22).
  - Add marker pairs for the new roles.
- Re-run `scripts/sync-font-size-classes.mjs` (→ `packages/ui/src/lib/font-size-classes.generated.ts`).
- Rewrite the prose in the token sources and generator templates: DESIGN.md §1 "runtime is dark-only" (around
  line 545), the atmosphere and warm-ramp text (around 554), §5 the glass carve-out (around 1001), and about 43
  dock/pager/menubar lines (200–377).

**Hardcoded literals (M)**
- About 70 hits: web 12 `rgba(255,255,255)`, 52 hex; ui 1 rgba, 6 hex.
- Worst: `web/src/components/design-system-showcase-tokens.ts` (41), `web/src/systems/profiles/lib/profile-identity.ts`
  (8), `packages/ui/src/lib/identity-palette.ts`, `symbol-palette.ts`.
- Hardcoded dark settings:
  - `sonner.tsx:20` `theme="dark"`
  - `loop-editor-canvas.tsx:150` `colorMode="dark"`
  - `cron-builder.tsx:84` `[color-scheme:dark]`
  - Logo `mode:"dark"` props
  - `<meta name="theme-color">` in `web/index.html`

**Lint and tests (S–M)**
- `lint-plugins/compozy-design-system-core-rules.mjs`: `no-design-glaze-rgba` hardcodes white-alpha classes, and
  `no-low-contrast-focus-ring` needs the new ring tokens. Tests are in
  `lint-plugins/__tests__/compozy-design-system.test.mjs`.
- `packages/ui/src/__tests__/focus-ring-contrast.test.ts` (reads through `token-source.ts`, first regex match)
  must assert per theme. `prose-ladder-contract.test.ts` pins the type tokens, and `terminal-view.test.tsx:33`
  pins `--terminal-selection`.

## Workstream B: theme toggle and persistence (S–M, after A)

- There is no theme state today. Appearance (`web/src/systems/os/apps/settings/appearance-settings-pane.tsx`,
  `hooks/use-appearance-settings-pane.ts`) holds Wallpaper, Dock magnification and Reduce motion as
  **in-memory runtime fields** (`runtime/window-manager-runtime-core.ts:58-60`, setters
  `window-manager-runtime.ts:285-297`), and they are never persisted.
- Persist the theme in localStorage (D3), following
  `web/src/systems/session/stores/session-streaming-preference-store.ts`, with an inline boot script in
  `web/index.html` that sets `data-theme` before first paint.
- Add a Theme control to the Appearance pane (light / dark / system) and a `ThemeToggle` in the rail foot. The
  toggle needs `aria-pressed` or a dynamic label.
- **Electron:**
  - `desktop/src/window/product-window.ts:79` and `boot-window.ts:25` hard-code `backgroundColor:"#131211"`;
    make them `nativeTheme`-aware or renderer-driven.
  - `product-window-chrome.ts` sets the Linux title-bar overlay color.
- Palette keywords: `internal/cmdpalette/corecmds/settings.go:19`, replace `dock` with `theme` (internal).

## Workstream C: shell chrome, rail and topbar (M; web + 1 Electron constant)

All paths below are in `web/src/systems/os/`.
- **Layout (`components/desktop-shell.tsx`, `DesktopShellScopedBody`).** It is a flex column (menubar, then
  `os-desk`, which contains wallpaper, `OsWinLayer`, surfaces and dock). Change it to a grid: topbar across the
  full width on row 1, and **rail plus `os-desk`** on row 2. The rail mounts beside the desk, not inside it.
- **Work area:**
  - `components/os-win-layer.tsx:167` is the **only** dock-band reservation
    (`bottom-[calc(var(--size-dock-band)+…)]`) → `inset-0`.
  - `hooks/use-os-win-layer.ts:51-75` measures bounds with `ResizeObserver` and sends `origin`, so the rail
    offset is picked up automatically.
  - Audit snap and deck geometry for any `origin.x === 0` assumption.
- **Dock → rail:**
  - Rewrite `components/os-dock.tsx` as a vertical rail (40px items, running dot at left, active = plate +
    `shadow-card`, minimized ring, orange badges, tooltips `side="right"`).
  - Delete `OsDockNewSession` and `OsDockZone`.
  - Add a rail foot with ThemeToggle, Settings (running the existing `settings.general` command) and the
    profile switcher (D6).
  - Add Up/Down roving focus.
- **`hooks/use-desktop-dock.ts`:**
  - Stop emitting separators (~79-80).
  - **Keep `onNewSession`**: the Sessions icon still calls it when there are no sessions.
  - Keep `renderItemMenu`; `os-dock-app-menu.tsx` works as is.
- **Delete magnification:** `use-dock-magnify.ts`, `lib/dock-magnify.ts` + its test, `setDockMagnify`
  (`window-manager-runtime.ts:290`), `window-manager-view.ts:188`, and the Appearance toggle
  (`appearance-settings-pane.tsx:127`). All internal, 18 files.
- **Compact (D2):** `components/os-dock-tab-bar.tsx`. If kept, `os-win-layer` needs a bottom reservation
  conditional on `presentation`.
- **Menubar → topbar (`components/os-menubar.tsx`, `desktop-menubar.tsx:176`):**
  - Drop `bg-shell-glass backdrop-blur-shell`; the bar becomes flat on `chrome`.
  - Remove the Settings `Control` and `onSettingsClick`. Settings stays reachable in the logo menu
    (`menubar/compozy-menu.tsx:37-42`).
  - Add a `pager` slot plus an **All desktops** button. Tray order: dots, grid, bell, ⌘K. `status` and
    `updateIndicator` stay conditional.
  - **Traffic lights:** keep the `env(titlebar-area-x/width)` handling (line 251), with a fixed 84px only as a
    fallback in macOS Electron. Linux puts window controls on the right.
  - The whole bar stays `[app-region:drag]`, and every control (including the dots and grid button) is
    `no-drag`. The bell, ⌘K and grid button stay outside every `role="menubar"` subtree.
- **Pager (`components/desktop-pager.tsx`, `desktop-pager-surface.tsx`):**
  - Mount it in the topbar.
  - Remove the click-active-dot-opens-overview behavior; the grid button owns the overview.
  - Keep the ±2 overflow `…` controls, which open the overview.
  - Rethink the `compact` `w-[50vw]` width.
  - The overview (`desktops-overview*.tsx`, opened via `use-desktop-shell-body.ts:129`) stays in
    `desktop-manager-surfaces.tsx`.
- **Electron:** `desktop/src/window/product-window-chrome.ts` `PRODUCT_TITLE_BAR_HEIGHT = 44` → 52, plus its test
  (`__tests__/product-window-chrome.test.ts`). `trafficLightPosition` may be needed to centre the lights.

## Workstream D: window frame, browser tabs, tiling (M; web + small Go)

All paths below are in `web/src/systems/os/`.
- **`components/os-window-frame.tsx` (`OsWindowChrome`):**
  - Drop `rounded-window border border-line-*` and `shadow-window`; the frame is a flat surface box.
  - Floating frames follow D1.
  - Blur dims identity and trail to .55.
  - Root heads get the IconWell (via `Topbar` `glyph`).
- **`components/os-traffic-lights.tsx`:** becomes quiet trailing icon buttons in the order minimize, zoom, close.
  **Keep** the API (`OsTrafficLightAction`, `onSelect`, `wrapZoom`, `zoomed`, `compact`), the
  `data-slot="os-traffic-lights"` pointer-capture guard, and the aria labels "Close window" / "Minimize window" /
  "Zoom window", which about 15 e2e steps click.
- **`components/os-window-deck.tsx`:**
  - Controls move to the right end; the row becomes a recessed `sunken` strip.
  - Hairline separators replace `gap-0.5` and hide next to the hover and active tabs.
  - Caret, `+` button, context menu and drag logic are reused as is.
- **`components/os-window-tab.tsx`:** the active tab is a surface plate with concave feet (10px radius box with a
  surface box-shadow, overlapping 1px, per DESIGN-NOTES round 3). Tokens `--width-deck-tab-max` 180 → 208,
  `--min-width-deck-tab` 96 → 136, and `--height-deck` / `--radius-deck-tab` re-tuned.
- **Tiling is already gap-parameterized; 0 works end to end:** `lib/layout-projection.ts`,
  `lib/window-manager-layout-area.ts` (early return at `gap === 0`), `lib/frame-seams.ts`, `lib/snap-targets.ts`,
  `lib/tiled-resize.ts`. `lib/window-deck-geometry.ts` needs no change.
- **`components/os-snap-seam.tsx`:**
  - Hit area 12 → 9px (±4.5); drop `rounded-pill`. Hover, drag, focus and keyboard resize already exist.
  - Hover color: the accent orange, or `muted` as in the prototype. Decide in the spec.
- **Check corners:** `react-rnd` free-edge handles against the 9px seam where tiled frames meet.
- **Arrange presets:**
  - Web today has `two-up | grid` (`lib/os-types.ts:77`, `lib/window-placement-presets.ts`); the daemon has
    `horizontal | vertical | grid | stack` + `layout.balance` (`internal/windowmanager/commands.go:251`).
  - Columns = horizontal, Balance = `layout.balance`.
  - **Main and stack is new:** add a daemon arrangement, a reducer case (`internal/windowmanager/reducer_layout_arrange.go`),
    the command id, and the tool schema (`internal/tools/builtin/window_manager_schemas.go`). This is an additive
    public enum; update the official skill references if they list arrangements.
- **Go config defaults (public surface: default change only, no key rename, no shim):**
  - `internal/config/window_manager.go:116-117` and `internal/windowmanager/config.go:122`: gaps `Inner 8 → 0`,
    outer `8/10 → 0`. Mirror them in `internal/daemon/window_manager_boot.go:286-317`,
    `internal/settings/window_manager_section.go:271` and `internal/api/contract/settings_window_manager.go:81`
    (plus OpenAPI examples if present).
  - `bindings.bottom_center = "reserved"` (the comment at `window_manager.go:96` exists only to protect the dock)
    → `zoom` or `none`.
  - `new_window_policy` default `floating` (`internal/config/window_manager.go:103`,
    `internal/windowmanager/config.go:111`), per D1.
  - Go tests: `internal/config/window_manager_test.go`, and `internal/settings/service_test.go:900` plus
    `config_apply_service_test.go:2384-2420`, which set `Inner = 0` as a "change". They must use a non-default
    value now.
  - The repo's own `config.toml:28` pins gaps to 8, so the new look won't show locally until it is edited.
  - Docs: `packages/site/content/docs/configuration/config-toml.mdx:202,1037,1053-1054`. Release note: users with
    explicit `[window_manager.gaps]` keep their gaps.

## Workstream E: `@compozy/ui` primitives (M)

Most changes cascade from tokens (A). Rough consumer counts are from `web/src`.

| Primitive | Change | Kind | Consumers | Effort |
|---|---|---|---|---|
| Button (`button-variants.ts`) | pill radius on every size; `primary`/`default` → inverted fg; `secondary`/`neutral` → surface-2, no border; `outline` → surface-2/ghost; optional ↵ `kbd` slot | mod | ~315 files / 619 uses (the orange → inverted switch hits every primary) | M |
| ButtonGroup / SplitButton / Toggle | pill radius | mod | few | S |
| Pill / PillCount | count badge → accent orange, 600 | mod | 117 / 3 files | S |
| PillGroup | pill segments; active = surface-2 + `shadow-card` | mod | 29 files | S |
| Filters (reui) / ListingToolbar | pill chips, 30px | mod | — | S–M |
| StatusDot | keep for needs-you orange and idle | mod | 17 files | S |
| **StateGlyph** | running spinner ring · queued dashed ring · done mint check · attention orange dot | **new** | migrate ~30–50 ad-hoc Spinner/StatusDot/check combos | M |
| Spinner (`Loader2`) | optionally render the ring | mod | 104 files | S |
| **IconWell** | 26px, r7, mint tint, as a `well` tone/size on `KindIcon` (extend, don't fork) | new tone | KindIcon 18, ItemMedia 5 | S–M |
| Surface | `variant="sunken"` (no border, r12) | mod | composites inherit | S |
| Card | r12 + `shadow-card` on surface | mod | CatalogCard/RadioCard/StatusCard | S |
| Tooltip | pill, inverted `bg-fg text-canvas`, 12/500; invert nested kbd | mod | 28 files | S |
| Popover / DropdownMenu / ContextMenu / Menubar | r12 + `shadow-pop`, 32px r8 items, uppercase micro labels via `eyebrow` (mind `no-inline-eyebrow`), pill Menubar trigger | mod | 9 / 28 / 12 files | M |
| Tabs / LaneTabs | underline stays; segmented uses → PillGroup | mod | 1 / 7 | S |
| Kbd | r6, surface-2, inset ring, fg-2 | mod | 16 files | S |
| Table | muted 13.5/500 header (not eyebrow), 40px; rows 48px, `hover:bg-sunken` | mod | 9 files + LinkedRecordTable | S |
| Dialog | r12 + overlay shadow; pill footers come automatically | mod | 50 files | S |
| Textarea / InputGroup | composer variant: surface + `shadow-card`, `shadow-elevated` on focus, pill tools, round inverted send | mod | 33 files | M |
| **ThemeToggle** | rail foot + Appearance | **new** | — | S (UI) |

- **Landing rules:**
  - New exports go in `primitives.ts` or `exports/foundation.ts`, with a colocated story and test
    (`packages/ui/CLAUDE.md`).
  - `compozy-ui-reuse/no-shadow-ui-primitive` bans local PascalCase copies once a primitive is exported. There
    are no collisions today, but watch `radio-card.stories.tsx` `IconWellLg`.
  - New colors must be tokens (`compozy-design-system/*`).
- **Tests at risk:**
  - About 38 files assert classes: `button.test.tsx`, `pill.test.tsx`, `tabs.test.tsx`, `status-dot.test.tsx`,
    `surface.test.tsx`, `kind-icon.test.tsx`, `radio-card.test.tsx`, `confirm-dialog.test.tsx`, `alert.test.tsx`,
    `empty.test.tsx`.
  - Most assert `data-slot` / `data-tone`, which survive.
  - Per the repo rule, fix production and update assertions only where the contract truly changed.

## Workstream F: tests, QA, docs, DS reference (M–L)

- **E2E (11 files).** `web/e2e/__tests__/os-shell.spec.ts` (23 selector hits, 173 KB; `rightHalfTileX` at
  4087-4090 still works at gap 0; compact control boxes at 1941-1943). Also `terminal-agent.spec.ts`,
  `profiles.spec.ts`, `attention.spec.ts`, `loop-run.spec.ts`, `terminal.spec.ts`,
  `workspace-setup.spec.ts`, `worktrees.spec.ts`, and `desktop/e2e/_electron/__tests__/shell.spec.ts:1206`.
  - The choke point is `web/e2e/fixtures/os-navigation.ts` (its Settings opener uses `os-menubar-settings` /
    `os-dock` / `os-dock-tabbar`), plus `selectors.ts` and `worktree-repo.ts`.
  - Keep `data-slot="os-dock-item"` and `data-app` on rail items (or rename everywhere at once).
  - Keep `aria-label="Dock"` or update the tests.
- **Unit tests (web).** In `components/__tests__/`: `desktop-pager`, `os-dock`, `os-window-frame` (242-268),
  `os-window` (221-227), `os-window-deck` (162). Also `lib/__tests__/dock-magnify`, `attention-model`,
  `onboarding-setup-frame.test.tsx`, `appearance-settings-pane.test.tsx`, and 4 hooks tests using `dockMagnify`.
- **Stories.**
  - OS stories: `os-dock`, `desktop-pager`, `os-traffic-lights`, `attention-surfaces`, `_desktop.tsx`,
    `_shell-fixture.tsx`, and the frame/deck/dialog-in-window stories.
  - Also `settings-appearance.stories.tsx` and `tasks-list-surface.stories.tsx`.
  - Totals: 138 stories in `packages/ui` and 223 in `web`.
  - There are no pixel baselines. "Re-baseline" means recapturing eng-ui-screenshot bundles for the 49 VC-tagged
    web stories and `packages-ui-storybook-overlays.spec.ts`.
- **QA scenarios (`docs/qa/scenarios/`).**
  - Rewrite: `ET-web-dock-default-window-size`, `ET-web-dock-contextual-session-launch`,
    `ET-web-menubar-menu-set`, `RT-desktop-pager-overview`, `ET-web-desktop-shell-lifecycle`,
    `ET-web-route-chrome-topbar`, `ET-web-tiled-window-shadow-flush`, `ET-window-tab-deck-lifecycle`,
    `ET-window-zoom-in-place`, `ET-window-manager-multi-client`, `ET-window-tab-close-reopen`,
    `ET-profile-desktop-restoration`, `APP-native-window-controls`.
  - Retire: `ET-web-dock-magnification`.
  - About 30 more carry incidental "Dock" wording.
  - **New:** theme toggle persists across reload and `system` follows the OS; the light theme passes contrast.
- **Copy and glossary.**
  - `docs/_memory/glossary.md:295,315,319,323,414` (shell model, Desktop pager, Dock, Menubar).
  - `COPY.md:259,309-311` (the Menubar entry lists Settings).
  - Add **Rail** and **Theme**.
- **Site docs.** `packages/site/content/docs/workspaces/window-management.mdx:100` ("lower left, aligned with the
  Dock") and `configuration/config-toml.mdx`.
- **DS reference.**
  - Rewrite `docs/design/opendesign/design-system/os-shell.html` (43 hits) and `ds-shell.css` (44).
  - Fonts in `ds-core.css:79-83`; traffic lights in `PARITY.md`.
  - The v2 board supersedes chapter 02.
- **Official skill (`skills/compozy/`).** No dock/menubar/pager mentions. Update only if `main_stack` or a theme
  config key ships.

## Compatibility (SD-013)

| Regime | Items | Handling |
|---|---|---|
| User state | persisted layouts (normalized rects), window snapshot | nothing to migrate; windows reflow once when the work area grows (the rail takes 60px of width, the dock band returns 82px of height) |
| Public surface | `window_manager.gaps.*` default → 0; `bindings.bottom_center` default; optional `new_window_policy` default; additive `main_stack` arrangement; optional theme config key (only if D3 = config) | same keys, explicit user values win, release note + docs defaults; additive enum is safe |
| Internal | dock, magnification, pager placement, traffic lights, menubar Settings, glass/dock/window tokens, fonts, primitives, tests, DS docs, palette keywords | delete without aliases; rename all consumers together |

## Suggested delivery order

1. **Tokens + theme infrastructure + fonts + generator** (A, B without UI).
   - Ship dark-only first with the new ramp, so the diff is the new look with no new behavior.
   - Then add the light set and the boot script.
2. **Primitives** (E): Button, Tooltip, menus, PillGroup, Table and Kbd, plus StateGlyph, the IconWell tone,
   the sunken Surface variant and ThemeToggle, each with story and test.
3. **Shell chrome** (C): rail, topbar, pager move, dock-band removal, Electron title-bar height and background.
4. **Window frame and tabs** (D): flat frame, trailing controls, browser tabs, seams, Go defaults, `main_stack`.
5. **Sweep** (F): E2E fixtures and specs, QA scenarios, glossary/COPY/site docs, DS reference, literal cleanup.

Each step goes through `make gate`. Final validation is a real app run of the rewritten QA scenarios in both
themes (eng-qa-bootstrap lab), not new test files alone.

## Effort summary

| Workstream | Effort |
|---|---|
| A: tokens, theming, fonts, DESIGN.md codegen, literals | **L** |
| B: theme persistence + toggle + Electron | S–M |
| C: rail, topbar, pager, work area | M |
| D: frame, browser tabs, seams, Go defaults, main+stack | M |
| E: primitives | M |
| F: tests, QA, docs, DS reference | M–L |

## Risks

- **Two theme mechanisms drifting apart** (`.dark` vs `data-theme`): set both from one place.
- **Light-mode contrast.** The reference's light muted text (`#a1a1a1`) is about 2.6:1 on white. Keep it for
  decoration only and use `fg-2` for readable copy, or darken it. The focus ring must be themed.
- **Weight and scale shift** (510 → 500, body 15 → 14.5): a broad layout reflow; check truncation-sensitive
  surfaces (tables, tabs, menus).
- **Floating windows without chrome** (D1): they need an explicit finish.
- **`origin.x` assumptions** in snap and deck geometry once the rail offsets the desk.
- **Site regression:** `packages/site` imports the tokens but runs its own `.dark` + `@theme inline`.
- **No flash on boot** in both web (`index.html` script) and Electron (window background).
