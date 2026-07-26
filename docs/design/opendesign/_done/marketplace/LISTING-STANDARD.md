# Listing standard — rows & cards

Reusable catalog listing pattern for AGH redesigns (Loops, Skills, Bridges, Vault, and similar inventories). Applied in `loops-catalog.html` and `vault-redesign.html`.

**Visual companion:** [`catalog-design-system.html`](./catalog-design-system.html) — live specs, anatomy, playground, and class contract for redesigning other inventory pages.

## When to use which view

| View | Default for | Use when |
| --- | --- | --- |
| **Rows** | Loops catalog, installed skills, bridges, vault-style inventories | Operator scanning, dense meta, status + rate + primary action |
| **Cards** | Marketplace / browse & install | Choosing among fewer items; logo + short pitch + install/run |

Default view is always **rows**. Persist `view=rows|cards` in URL/search params when both ship.

**Marketplace exception:** the unified marketplace kind pages are cards-only. The listing-toolbar trailing PillGroup is reused for **`Installed | Marketplace`** scope (with icons), not Rows|Cards. See `UNIFIED-CATALOG-SPEC.md` §2.1.

## Toolbar anatomy (left → right)

```
[ SearchInput (compact) ] [ Filters (reui chip bar) ] … spacer …
[ PillGroup: Rows | Cards ]
```

Marketplace kind pages (exception — no Filters, no view mode):

```
[ SearchInput (compact) ] … spacer …
[ PillGroup: Installed | Marketplace ]   ← icons on both segments; Installed may show count chip
```

- **Search** = `@agh/ui` `SearchInput` (26px row, min 220px — `--width-search-input-min`, `/` shortcut). Lives **before** Filters in the listing toolbar — not in the topbar. AND-combined with filter chips.
- **Filters** = `@agh/ui` reui `<Filters>` (HTML: vault-redesign chip pattern). Not kind/category tab strips. Marketplace kind pages omit Filters.
- **View mode** = `PillGroup` (default `md`, 24px segments) — borderless track, `radius-xs` segments, elevated active segment, never accent fill. Labels `Rows` | `Cards` (optional Lucide `List` / `LayoutGrid` inside the segment). Marketplace reuses this slot for **scope**, not view mode.
- Do **not** show a “Sorted by …” label unless the page has a real sort control.
- Do **not** double-encode the same facet as both a PillGroup and a Filter field.
- Do **not** put Marketplace|Installed as underline page-body tabs — that control is the trailing PillGroup only.

## Topbar (inventory pages)

**Default (Vault / Bridges / Loops):** title-in-topbar shell — **not** a breadcrumb:

```
[ icon well ] [ Title ] [ count ] ………… [ secondary ghost ] [ primary CTA ]
```

- Icon: 24×24 elevated well, accent stroke icon (12px).
- Title: 14px medium. Count: mono chip.
- Trailing actions use the system button pair (`PageActionsTopbarSlot` — both buttons `size="sm"`, same 22px row):
  - **Secondary** = `btn btn--ghost btn--sm` (Runs, Refresh, …)
  - **Primary CTA** = `btn btn--primary btn--sm` (New from template, New secret, …)
- Do **not** use `btn--outline` for the default secondary in inventory topbars.
- Do **not** size the primary CTA larger than its sibling ghost — topbar actions share one row height.

### Marketplace chrome (exception)

Marketplace kind pages follow the product shell in `systems/design-system.html`, not the inventory title-in-topbar shell:

```
Topbar:     Breadcrumb (Home › Marketplace › Kind) · RouteNav (Skills · MCPs · Extensions · Bundles) · Refresh
PageHead:   kind icon well · H1 · count · meta
Toolbar:    Search leading · Installed | Marketplace PillGroup trailing
Body:       card grid
```

Full contract: `UNIFIED-CATALOG-SPEC.md` §2.1.

## Filter fields (Loops catalog reference)

| Key | Type | Notes |
| --- | --- | --- |
| `kind` | select | `builtin` / `custom` |
| `category` | select | Distinct categories from data |
| `status` | select | Last-run status (10-state enum) |
| `mode` | select | e.g. `delivery` / `watch` |
| `name` | text | contains / starts_with / is |

Chip shape: **label · operator · value · remove**. One chip per field (`allowMultiple: false`). AND across chips.

## Row layout (canonical)

Grid: `[ leading 34px ] [ main minmax(0,1fr) ] [ trail auto ]`

| Slot | Content |
| --- | --- |
| Leading | Neutral icon well `34×34`, `rounded-md`, `bg-elevated` |
| Main · name | Truncated name + kind/source tags + mono slug |
| Main · description | One truncated line |
| Main · meta | Dot-separated mono facts + optional neutral binding badge |
| Trail | `Pill mono tone="neutral"` (type/namespace) → always-visible delete / primary action |

- Hover: `--row-hover` only. No lift, no side-stripe accent.
- Selected (optional, for main-pane lists that keep an active row while detail is open): `data-selected` → `--row-selected` / `bg-row-selected`. No selection rail, dot, or indicator chrome — those remain `Item`'s job.
- Flat list by default (no namespace/type section headers). Filters own type/namespace scoping.
- Do not duplicate type in meta when the trail already shows a type Pill.
- Type/kind labels use `@agh/ui` **`Pill`** (`mono`, `size="sm"`, `tone="neutral"`). Vault namespaces use `vaultNamespaceTone` (`sessions` → `info`, else `neutral`). Never invent bordered elevated chips.

## Card layout (same data)

Compose like `@agh/ui` `CatalogCard`:

```
[logo 24]  title
           updated · relative date   ← under title (CatalogCard.Meta eyebrow — Inter UC, text-subtle; ids stay mono)
description (optional — only when it adds meaning)
── border-t ──
Pill type (+ optional Pill kind) ····· Delete / Run
```

Grid: `1 / 2 / 3` columns at `sm` / `xl`. Cards are **borderless** flat surfaces — `canvas-soft` resting, `hover:bg-elevated`, never outlined, no shadows.
- Relative date stays under the title (`.listing-card__eyebrow`), not in the footer.
- Type lives in the footer (opposite the action) as `Pill mono`.
- Body description is optional. Use it for real secondary copy (e.g. loop goal). Never repeat the title / ref / name in the body — Vault cards omit `.listing-card__desc` entirely.

## Shared class contract (HTML prototypes)

| Role | Classes |
| --- | --- |
| Toolbar | `.listing-toolbar` / `.listing-toolbar__leading` / `.listing-toolbar__trailing` |
| Search | `.search` (`SearchInput` — before Filters) |
| Filters | `.filters` / `.filter-*` (vault) |
| View toggle | `.pill-group` / `.pill-group__item` |
| Type / kind | `.pill[data-slot=pill][data-mono=true]` (trail on rows; footer-meta on cards) |
| Rows | `.list` > `.listing-row` (alias `.row`) |
| Cards | `.listing-card-grid` > `.listing-card` |
| Empty | `.empty` |

## Anti-patterns

- Kind/category as a second segmented control when Filters already cover them
- Icon-only view toggle without `PillGroup`
- Accent side-stripes, gradients, hover-lift on rows/cards
- Invented metrics or fake catalog entries
- Designer chrome (viewport/theme toggles) inside the product surface
