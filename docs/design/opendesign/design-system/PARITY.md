# PARITY.md — prototype DS ↔ `@compozy/ui` production map

Living two-way map between this folder's prototype vocabulary and the production
primitives in `packages/ui/src`. Replaces `UI-ALIGNMENT-SPEC.md` (shipped in full;
archived in git history). Rule: **reuse before create** — before authoring any
prototype component, find its row here; before proposing a new production
primitive, check `packages/ui/src/index.ts`.

## Token naming

Prototypes use **bare names**; production prefixes colors with `--color-*` and
uses rem-based `--text-*`/`--height-*` tokens. Values are identical (px mirrors
rem at 16px base) in **both themes**: `ds-core.css` `:root` mirrors
`tokens.css` (dark, the default) and its `[data-theme="light"]` block mirrors
`tokens-light.css`. Set `data-theme="light"` on `<html>` to preview light.

| Prototype (`ds-core.css`) | Production (`tokens.css`) |
| --- | --- |
| `--rail`, `--desk`, `--canvas`, `--canvas-soft`, `--canvas-tint`, `--sunken`, `--surface-2`, `--selected`, `--code-bg`, `--elevated`, `--well` | `--color-rail`, `--color-desk`, `--color-canvas`, `--color-canvas-soft`, `--color-canvas-tint`, `--color-sunken`, `--color-surface-2`, `--color-selected`, `--color-code-bg`, `--color-elevated`, `--color-well` |
| `--fg`, `--fg-strong`, `--fg-2`, `--fg-3`, `--muted`, `--subtle`, `--faint`, `--disabled` | `--color-fg` … `--color-disabled` |
| `--primary`, `--primary-hover`, `--primary-ink` | `--color-primary`, `--color-primary-hover`, `--color-primary-foreground` |
| `--attn` | `--color-attn` (alias of `--color-accent`: highlight / needs-you) |
| `--line`, `--line-soft`, `--line-strong`, `--line-focus` | `--color-line*`, `--color-line-focus` |
| `--accent*`, signal tones + `-tint`s, `--neutral-ink` | `--color-accent*`, `--color-success*`, … |
| `--row-hover`, `--row-selected`, `--input-fill`, `--btn-fill`, `--btn-hover`, `--badge-fill`, `--surface-glaze`, `--bar-fill`, `--scrim` | `--color-row-hover`, `--color-row-selected`, `--color-input-fill`, `--color-btn-default-fill`, `--color-btn-default-hover`, `--color-badge-fill`, `--color-surface-glaze`, `--color-bar-fill`, `--color-overlay-scrim` |
| `--dur-fast/--dur/--dur-slow`, `--ease`, `--ease-in-out` | `--duration-fast/base/slow`, `--ease-out`, `--ease-in-out` |
| `--focus-ring(-inset)`, `--highlight`, `--shadow-card`, `--shadow-elevated`, `--shadow-pop`, `--shadow-overlay`, `--shadow-hairline(-inset)`, `--inset-strong` | `--shadow-focus-ring/-inset`, `--shadow-highlight`, `--shadow-card`, `--shadow-elevated`, `--shadow-pop`, `--shadow-overlay`, `--shadow-hairline(-inset)`, `--shadow-inset-strong` (theme-scoped shadows are `@theme` adapters over `--theme-shadow-*`) |
| `--font-sans` (Inter), `--font-mono` (Geist Mono), `--font-keys`, `--w-regular/-medium/-semibold` (425/500/600) | `--font-sans`, `--font-mono`, `--font-keys`, `--font-weight-normal/-medium/-semibold` |
| `--text-meta` 13, `--text-body` 14.5, `--text-heading` 17, `--text-count` 9.5 | `--text-meta`, `--text-body`, `--text-heading`, `--text-count` |
| `--viz-*` | `--color-viz-*` |
| shell (ds-shell.css): `--h-bar` 52, `--w-lights` 84, `--w-rail-dock` 60, `--rail-item` 40, `--h-deck` 40, `--h-deck-tab` 32, `--w-deck-tab` 208/136, `--h-win-head` 48, `--h-win-toolbar` 44, `--seam-hit` 9, `--dur-shell-*`, `--ease-spring`, `--teal` | `--height-menubar`, `--width-traffic-lights`, `--width-rail`, `--size-rail-item`, `--height-deck`, `--height-deck-tab`, `--width-deck-tab-max`/`--min-width-deck-tab`, `--height-window-head`, `--height-window-toolbar`, `--size-seam-hit`, `--duration-shell-*`, `--ease-spring`, `--wallpaper-teal` |

Retired with the dock-era shell (still in `tokens.css` until their last consumer
goes, then deleted): `--shell-glass*`, `--radius-window/-dock/-dock-item`,
`--shadow-window*`, `--shadow-dock`, `--shadow-shell-*`, `--size-dock-*`,
`--size-traffic-light*`, `--spacing-traffic-light-gap`. Prototypes must not use
their bare mirrors.

## Component map

| Prototype class | `@compozy/ui` export | Notes |
| --- | --- | --- |
| `.btn` (+variants/sizes) | `Button`, `buttonVariants` | pills · 24/26/32/34/36/44 ladder; `--primary` inverted, secondary `--surface-2`; press = translateY(1px) |
| `.btn[aria-pressed]` | `Toggle` | on = elevated + fg-strong + highlight |
| `.pill-group` | `PillGroup` | filter pills: segment 30 at 13.5 (sm 20) |
| `.field` (28px search) | `SearchInput` | canvas-soft fill, 12px glyph |
| `.input` / `.textarea` / `select.input` | `Input` / `Textarea` / `NativeSelect`, `Select` | 36px; disabled = token swap |
| `.ctl` (32px) | `Select size="sm"`, compact controls | `--height-control-compact` |
| `.cbx` | `Checkbox` | 16px, radius 6, accent plate |
| `.switch` | `Switch` | 32×18 / sm 24×14 |
| `.pill` (+tones/forms) | `Pill`, `PillDot`, `pillVariants` | round, 18/20/24, tint+ink |
| `.tag` / `.count-chip` / `.provchip` | `Pill` variants / Tabs count chip / `MonoId` context | |
| `.livebadge` | Tabs `liveLabel`, `LiveBadge` | aria-live polite |
| `.mono-id` | `MonoId` | mono 11, copy affordance in prod |
| `kbd`/`.key`/`.keys`/`.chord` | `Kbd`, `KbdGroup`, `CommandShortcut` | `--font-keys`, never mono |
| `.eyebrow` / `.eyebrow-caps` | `Eyebrow` (`default`/`caps`) | lint: `no-inline-eyebrow` |
| `.tab` / `.lane-tab` | `Tabs`/`TabsTrigger` / `LaneTabs` | 1.5px fg-strong underline |
| `.listing-row` family | `ListingRow.*` | 34px well, title 15/510/-.01em |
| `.list-shell` / `.listing-toolbar` | `ListGroup` / `ListingToolbar` | strip order locked |
| `.listing-card` | `CatalogCard` | |
| `.surface` | `Surface` | flat tile — no border/shadow |
| `.table` family | `Table.*`, `LinkedRecordTable` | th 36 eyebrow, td 10/12 |
| `.menu` family | `DropdownMenu`/`ContextMenu`/`Menubar` popups, `Select` content | radius 14, hairline, highlight = elevated |
| `.palette` / `.pal-*` | `Command*`, `CommandDialog` | opaque panel; chapter 04 |
| `.dialog` system | `Dialog*`, `dialogShellClass`, `EntityDialog*` | 560/720/880/1180; ruled gutter 20px |
| `.choice` | `RadioCard`, `Choice*` | neutral selection |
| `.srow` family / `.tiles` / `.savebar` / `.notice` / `.adv-toggle` | settings surfaces in `web/` + `Field*`, `Alert`, `ActionResultBanner` | |
| `.ffield`/`.flabel`/`.fhint`/`.ferr` | `Field`, `FieldLabel`, `FieldDescription`, `FieldError` | label never outranks value |
| `.kpi` / `.kpi--lg` | `Metric`, `MetricGrid` | 17 window / 24 hero, weight 620 |
| `.meter` / `.progress` / `.bars` / `.im` | `Progress`, `Sparkline`, `StackedProgress`, `IntensityMeter` | viz ink monochrome |
| `.empty` (+framed/cause) | `Empty` | 48 well / 20 glyph / 18 title |
| `.sk` | `Skeleton`, `SkeletonRows` | shimmer 2s |
| `.att` rows | bell/approvals surfaces | |
| `.session-row` / `.agent-group` | sessions sidebar in `web/` | selected = neutral plate |
| `.transcript`/`.composer`/`.toolcall` | `ChatMessageBubble`, `StreamMarkdown`, `ToolCallRow`, `CodeBlock` | |
| `.d` / `.status-dot` | `StatusDot`, `ConnectionIndicator` | color + shape |
| `.select-rail` | `ItemSelectionIndicator` (`rail`/`dot`) | 2px, inset 8 |
| shell `.menubar` (topbar) · `.pager`/`.pager-dot` · `.rail`/`.rail-item`/`.rail-foot` · `.win*` · `.wc*` · `.deck*` | `web/src/systems/os/**`: `OsMenuBar` ("System bar"), `DesktopPager`, `OsDock` ("Dock") + rail foot (`ProfileSwitcher`, `ThemeToggle`, Settings), `OsWindowFrame`, `Topbar` (the 48px **window head**, not the menubar), `OsTrafficLights` (quiet Minimize/Zoom/Close icons), `OsWindowDeck`/`OsWindowTab`, `OsWindowToolbar` | chapter 02 documents values; the old traffic-light squares and bottom glass dock are gone |
| `.popover`/`.pop-item` | shell popovers (`DropdownMenu`, `Menubar` content) | `--canvas` + `--shadow-pop`, no glass |
| `.snap-preview`/`.snap-seam` | `OsSnapOverlay`, `OsSnapSeam` | seam: 9px hit, 1px `--line`, 2px accent on hover/drag |
| `.desk-empty` | `OsEmptyDesktop` | "{name} is empty" card + New session |
| `.ov`/`.space-card` | `DesktopsOverview` | opened from the tray's All desktops |

## Production inventory not yet mirrored in prototypes

`Stepper`, `Tree`, `SplitButton`, `Combobox`, `InputGroup`, `Slider`,
`Accordion`/`Collapsible`, `Avatar*`, `Tooltip`, `Toast (Sonner)`,
`DataSurface`, `Filters` builder menus, `Timeline`, `OwnerAvatar` palette,
`Dock` (decision dock — NOT the OS dock; naming collision, disambiguate in
prose), chart family (`DayAreaChart`, `DayStackedBars`, `QueueHealthSparkline`,
`StatusBreakdown`, `PriorityBars`), `QrCode`, `Logo`, ~35 brand logos.
When a prototype needs one, mirror the production contract here first.
