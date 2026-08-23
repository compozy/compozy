# PARITY.md — prototype DS ↔ `@compozy/ui` production map

Living two-way map between this folder's prototype vocabulary and the production
primitives in `packages/ui/src`. Replaces `UI-ALIGNMENT-SPEC.md` (shipped in full;
archived in git history). Rule: **reuse before create** — before authoring any
prototype component, find its row here; before proposing a new production
primitive, check `packages/ui/src/index.ts`.

## Token naming

Prototypes use **bare names**; production prefixes colors with `--color-*` and
uses rem-based `--text-*`/`--height-*` tokens. Values are identical (px mirrors
rem at 16px base).

| Prototype (`ds-core.css`) | Production (`tokens.css`) |
| --- | --- |
| `--canvas`, `--canvas-soft`, `--canvas-tint`, `--elevated`, `--rail` | `--color-canvas`, `--color-canvas-soft`, `--color-canvas-tint`, `--color-elevated`, `--color-rail` |
| `--fg`, `--fg-strong`, `--muted`, `--subtle`, `--faint`, `--disabled` | `--color-fg` … `--color-disabled` |
| `--line`, `--line-soft`, `--line-strong`, `--line-focus` | `--color-line*`, `--color-line-focus` |
| `--accent*`, signal tones + `-tint`s, `--neutral-ink` | `--color-accent*`, `--color-success*`, … |
| `--row-hover`, `--row-selected`, `--input-fill`, `--btn-fill`, `--btn-hover`, `--badge-fill`, `--surface-glaze`, `--bar-fill`, `--scrim` | `--color-row-hover`, `--color-row-selected`, `--color-input-fill`, `--color-btn-default-fill`, `--color-btn-default-hover`, `--color-badge-fill`, `--color-surface-glaze`, `--color-bar-fill`, `--color-overlay-scrim` |
| `--dur-fast/--dur/--dur-slow`, `--ease`, `--ease-in-out` | `--duration-fast/base/slow`, `--ease-out`, `--ease-in-out` |
| `--focus-ring(-inset)`, `--highlight`, `--shadow-overlay`, `--shadow-hairline(-inset)`, `--inset-strong` | `--shadow-focus-ring/-inset`, `--shadow-highlight`, `--shadow-overlay`, `--shadow-hairline(-inset)`, `--shadow-inset-strong` |
| `--viz-*` | `--color-viz-*` |
| shell: `--shell-glass(-pop)`, `--teal`, `--r-win/-dock/-dock-item`, `--dur-shell-*`, `--ease-spring`, `--shadow-win*`, `--dock-band` | `--shell-glass(-pop)`, `--wallpaper-teal`, `--radius-window/-dock/-dock-item`, `--duration-shell-*`, `--ease-spring`, `--shadow-window*`, `--size-dock-band` |

Known production gaps (flagged in the divergence ledger): window-head 44px and
strip 38px are untokenized in `web/` (`h-11`, `h-[38px]`); the `tokens.css`
z-ladder comment (menubar 500 · dock 600) is stale prose — production layers
menubar/dock in flow.

## Component map

| Prototype class | `@compozy/ui` export | Notes |
| --- | --- | --- |
| `.btn` (+variants/sizes) | `Button`, `buttonVariants` | 24/26/30/34/36/44 ladder; press = translateY(1px) |
| `.btn[aria-pressed]` | `Toggle` | on = elevated + fg-strong + highlight |
| `.pill-group` | `PillGroup` | track pad 2 / gap 1; segment 24 (sm 20) |
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
| shell `.menubar`/`.dock*`/`.win*`/`.deck*` | `web/src/systems/os/**` (`Topbar`, `OsDock`, `OsWindowFrame`, `OsWindowDeck`, `OsTrafficLights`) | chapter 02 documents values + file paths |
| `.popover`/`.pop-item` | shell popovers (`DropdownMenu` on glass) | |
| `.snap-preview`/`.snap-seam` | `OsSnapOverlay`, `OsSnapSeam` | ephemeral affordances |

## Production inventory not yet mirrored in prototypes

`Stepper`, `Tree`, `SplitButton`, `Combobox`, `InputGroup`, `Slider`,
`Accordion`/`Collapsible`, `Avatar*`, `Tooltip`, `Toast (Sonner)`,
`DataSurface`, `Filters` builder menus, `Timeline`, `OwnerAvatar` palette,
`Dock` (decision dock — NOT the OS dock; naming collision, disambiguate in
prose), chart family (`DayAreaChart`, `DayStackedBars`, `QueueHealthSparkline`,
`StatusBreakdown`, `PriorityBars`), `QrCode`, `Logo`, ~35 brand logos.
When a prototype needs one, mirror the production contract here first.
