# Shared UI Primitives

`@compozy/ui` owns generic primitives for the web app and site. `src/index.ts` is the public inventory; colocated stories show supported use. Root `CLAUDE.md` owns delivery; `DESIGN.md` owns visual grammar.

- Keep primitives free of domain imports, queries, SSE, product stores, and Compozy-specific prop defaults. Domain composites live in `web/src/systems/<domain>/components/` and compose these primitives.
- Consumers import from `@compozy/ui`, never package internals. Internal renames update all consumers together without compatibility re-exports.
- Consumers do not keep a prefixed or restyled copy of a primitive that exists here; a near-miss extends the primitive with a variant, slot, or tone. Promoting a composite into this package migrates or domain-renames every existing callsite in the same change; dormant promoted code beside live local copies fails `compozy-ui-reuse/no-shadow-ui-primitive`.
- A new exported behavior ships a colocated story and coverage in the nearest suitable `__tests__/` suite. Reuse existing coverage for re-exports or editorial changes; do not add file-existence tests.
- Prototype-only marks do not become brand variants. `Logo` changes require an explicit design-system decision; actual brand assets remain authoritative.
- Use `useReducedMotionConfig()` under `UIProvider`. Give each animation one owner: CSS for simple states; `motion` for unmount, synchronized, or layout transitions. Width animations explicitly use zero duration under reduced motion.
- Dialog follows its Base UI portal lifecycle; Popover and Sheet keep their existing lifecycle patterns. Do not combine `data-open:animate-*` with a second motion exit owner.

| Need                               | Primitive                 |
| ---------------------------------- | ------------------------- |
| Main-pane inventory/navigation     | `ListingRow`              |
| Compact rails, pickers, inspectors | `Item`                    |
| Section heading/count around rows  | `ListGroup`               |
| Catalog card view                  | `CatalogCard`             |
| Kanban, inbox, chat/tool grammar   | Existing domain composite |

Stories cover distinct supported variants using real primitives; mock data boundaries only. Use tokens, tag interaction stories `play-fn`, and keep the standard dark background.
