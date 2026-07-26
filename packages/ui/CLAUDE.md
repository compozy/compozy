# CLAUDE.md (packages/ui)

`@agh/ui` is the single source of generic UI primitives for every AGH surface (`web/`, `packages/site/`). `src/index.ts` is the surface contract **and** the canonical primitive inventory — check it before authoring any component on any surface; if an identifier is not exported there, consumers cannot import it. Colocated stories under `src/components/**/stories/` are the canonical usage reference. Token/type/depth grammar: root `DESIGN.md`. (Root `CLAUDE.md` rules apply — this file adds package-specific ones.)

## Tripwires

- **No domain imports.** Nothing from `web/src/**`, `@/systems/**`, TanStack, `agh-openapi` types, or zustand. A primitive that owns a query, store, or SSE subscription belongs in `web/src/systems/<domain>/` instead.
- **No AGH-specific defaults in primitive props** — defaults stay generic or become required props.
- **Reference artwork never extends brand primitives** — a mark/asset that exists only to match a mock or prototype is placeholder; render the real brand asset or a domain component in `web/src/systems/<domain>/`. `Logo` variants change only by explicit design-system decision (L-032).
- **No new export without a colocated story and a test in the same PR.** Tests live in the nearest `__tests__/` beside the source.
- **Renames are hard cuts** — update every consumer in the same change; no compat re-exports or aliases.
- **`useReducedMotionConfig()` (context-aware), never `useReducedMotion()`** inside primitives under `UIProvider`.
- **Never pair `data-open:animate-*` keyframes with `motion` exit animations** — one animation owner per primitive.

## Where a component lives

Domain-free shape a second surface could use unchanged (slot/variant API, token-driven) → here. Reads session events, hits a query, or only makes sense inside one domain → `web/src/systems/<domain>/components/`, composing these shells — never redefining them (`compozy-ui-reuse/no-shadow-ui-primitive` blocks shadows; domain variants take a domain-prefixed name like `SessionToolCallRow`). Consumers import from `@agh/ui`, never `packages/ui/src/**`.

## Which list component?

| Need                                                       | Use                                 |
| ---------------------------------------------------------- | ----------------------------------- |
| Main-pane inventory / navigational rows                    | `ListingRow`                        |
| Compact selection chrome (rails, pickers, inspector lists) | `Item` (`selected` + `indicator`)   |
| Section shell around either (eyebrow + count + items)      | `ListGroup`                         |
| Card view of the same listing data                         | `CatalogCard`                       |
| Dedicated grammars (kanban, inbox, chat/tool rows)         | domain components in `web/systems/` |

## Motion vs. CSS

CSS owns simple state (hover, focus, pulse, shimmer). `motion` owns unmounts, sibling-synced timing, and layout transitions — Dialog/Popover/Sheet exits use the `AnimatePresence` + `actionsRef` pattern. Width animations ignore `reducedMotion`: any primitive animating width (Sidebar collapse, SplitPane resize) sets `duration: 0` explicitly when the provider is reduced.

## Stories

Every variant has a story; stories render real primitives (mocks replace data only); tokens only — no hex/`rgb()`/`hsl()` literals; interaction-dependent stories are tagged `play-fn`; dark background is implicit (never pass a background override).
