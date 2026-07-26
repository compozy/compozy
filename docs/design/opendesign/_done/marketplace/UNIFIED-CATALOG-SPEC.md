# Marketplace — single-surface IA spec (v2)

> **Purpose.** Issue-level spec for collapsing the duplicated catalog IA into **one marketplace surface**. Input for `cy-create-techspec` / task decomposition. All `web/` paths verified against the tree on 2026-07-17. v2 replaces the v1 "one home per kind route" model after review: there are no per-kind sidebar routes at all.
>
> **Design artifacts:** `marketplace.html` (canonical — the whole surface), `marketplace-detail.html` (per-item detail), `bundle-activation-detail.html` (activation deep view), `marketplace-index.html` (IA map).

## 1. Problem

The marketplace shipped as a parallel IA: `/marketplace` (+ `?kind=` views) only installs, while `/skills`, `/mcp`, `/extensions` (+ Bundles tab) only list what is installed, and extensions have two detail pages for the same item. Six sidebar Catalog entries overlap in scope. Users have two half-answers for "how do I add a skill?".

## 2. Decision — one Marketplace surface

- **Sidebar Catalog group has a single entry: `Marketplace`** (plus the non-catalog `Bridges` and `Knowledge`). The `Skills`, `MCP`, and `Extensions` sidebar entries are deleted.
- **Topbar `RouteNav` carries four kind links: `Skills · MCPs · Extensions · Bundles`** — real subroutes, Skills is the default. Kind switching is **not** a page-body tab strip.
- **Every kind subroute uses the same chrome + template** (parity with `systems/design-system.html` and production catalog pages). See §2.1.
- What changes between kinds is only card content and actions (skill version/scope vs MCP transport/status vs extension trust/enable vs bundle profile/contents) — never the chrome or template.
- Installed management (enable, update, authorize, remove, deactivate) lives in the **Installed scope** (listing-toolbar PillGroup). Nothing in the product shows "only installed" as a separate page anymore.

### 2.1 Chrome contract (all four kind subroutes)

Canonical stack — match `marketplace.html` and `systems/design-system.html` (`Topbar → PageHead → listing toolbar → body`):

```
Topbar (48px)
  leading:  Breadcrumb  Home › Marketplace › {Kind}
  center:   RouteNav    Skills · MCPs · Extensions · Bundles
  trailing: ghost Refresh

PageHead (PH1)
  icon well (kind) · H1 (kind name) · mono count · meta
  "N in the marketplace · N installed · N updates available"

Listing toolbar
  leading:  SearchInput (compact, `/` focuses)
  trailing: PillGroup  Installed | Marketplace   ← icons on both segments;
            (Installed carries mono count chip)     occupies the Rows|Cards slot

Body
  card grid only — no rows, no Rows|Cards toggle, no underline Marketplace|Installed tabs
```

| Zone | Owns | Must not |
| --- | --- | --- |
| Topbar breadcrumb | Where you are (`Home › Marketplace › Kind`); Marketplace links to `/marketplace` | Kind icon / H1 / count |
| Topbar RouteNav | Which **kind** subroute | Installed vs Marketplace scope |
| PageHead | Kind identity (icon · H1 · count · meta) | Search, scope toggle, install CTA |
| Listing toolbar | Search + **scope** (Installed\|Marketplace PillGroup) | Underline tabs; Rows\|Cards view toggle; Filters chip bar |
| Card grid | Entries for the active scope | Designer chrome / viewport toggles |

URL state: `?tab=installed` selects Installed scope (`tab` omitted = Marketplace). `?q=` is the search query. Do **not** reintroduce page-body underline tabs for scope.

### Route table

| Route | Content |
| --- | --- |
| `/marketplace` | Index — redirects to `/marketplace/skills` |
| `/marketplace/skills` (default) · `/marketplace/mcps` · `/marketplace/extensions` · `/marketplace/bundles` | Shared kind template: §2.1 chrome + card grid. URL state: `?tab=installed`, `?q=` |
| `/marketplace/$kind/$entryId` | Per-item detail for all four kinds (README, config template, trust rail, contents). Installed state adds management: skill content/shadows, extension enable/env/diagnostics/provenance, MCP config/status/auth. "Manage" lands on the kind's Installed scope (`?tab=installed`) |
| `/marketplace/bundles/activations/$id` | Bundle activation deep view (profile contents, inventory, bind switch, deactivate) — moved from `/extensions/bundles/$id` |

### Deleted routes (zero-legacy, no redirects)

`/skills` · `/skills/$name` · `/mcp` · `/extensions` · `/extensions/$name` · `/extensions/bundles/$id` · the `?kind=` search-param views on `/marketplace` (kind becomes a **path segment**) · the marketplace Overview landing (no cross-kind shelf page; Skills is the entry).

### Behaviors (prototype contract — `marketplace.html`)

1. Topbar: breadcrumb `Home › Marketplace › Kind`, with Marketplace linked to `/marketplace`; center `RouteNav` with the four kind links; trailing ghost `Refresh` (`POST /api/marketplace/refresh?kind=`). Route identity (icon/H1) does **not** live in the topbar.
2. PageHead (PH1): kind icon well + kind H1 + mono count + meta `N in the marketplace · N installed · N updates available`.
3. Listing toolbar: SearchInput leading; trailing PillGroup **`Installed | Marketplace`** with Lucide-style icons (package / store bag). Installed segment carries a mono count chip. Default scope is Marketplace (`tab` omitted). **No** page-body underline tab strip for scope.
4. **Marketplace scope**: full catalog for the kind; installed entries are marked (`installed`/`active` pill, ghost `Manage` → Installed scope); updatable entries show `vX available` + `Update`; others show `Install`/`Activate`. MCP cards open the guided install dialog (stdio env fields / remote OAuth info + scope); bundle cards open the activation dialog (profile, scope, bind switch, "What changes" preview); unverified extensions confirm before install.
5. **Installed scope**: same card anatomy, management trail per kind — skills: update + overflow (`via <bundle>` items swap Remove for "Open bundle activation"); MCPs: status pill (`running`/`authorize`) + `Authorize` when pending; extensions: enable switch + trust pill; bundles: profile/scope meta, deactivate via overflow. Remove/deactivate use the destructive modal (typed confirmation for removes).
6. Search filters the active scope; `/` focuses it. Empty states: teaching empty per kind on Installed ("…shows up here" + CLI hint + "Browse the marketplace" button that switches scope), query-empty with Clear search, route error with Retry, loading skeleton grid.
7. Install/activate flashes the card once and offers "View installed →" in the toast; remove returns the item to the Marketplace scope.

## 3. `web/` refactor inventory

### 3.1 DELETE

| Path | Why |
| --- | --- |
| `web/src/routes/_app/skills.tsx`, `skills.$name.tsx` | Kind lives at `/marketplace/skills`; skill detail merges into marketplace detail |
| `web/src/routes/_app/mcp.tsx` + `web/src/routes/_app/-mcp-context.tsx` + `web/src/hooks/routes/use-mcp-page.ts` | MCP management moves to the Installed scope (dialogs reused) |
| `web/src/routes/_app/extensions.tsx`, `extensions.$name.tsx`, `extensions.bundles.$id.tsx` | Inventory pages and split detail dissolve into the marketplace surface |
| `web/src/systems/marketplace/components/marketplace-landing.tsx` + landing half of `marketplace-route-bodies.tsx` | No Overview page; Skills is the default subroute |
| `web/src/systems/marketplace/components/marketplace-kind-view.tsx` | Replaced by the shared kind template (§2.1 chrome + cards) |
| `web/src/systems/marketplace/components/marketplace-row.tsx` | Cards only — no rows in this family |
| `web/src/systems/skill/components/skill-list-panel.tsx`, `skill-list-filters.tsx`, `web/src/systems/skill/lib/skill-list-filters.ts` | Installed-skills listing replaced by Installed-scope cards |
| `web/src/systems/extensions/components/extensions-inventory.tsx`, `extension-inventory-row.tsx`, `extension-inventory-card.tsx`, `bundle-inventory-row.tsx`, `bundle-inventory-card.tsx`, `inventory-empty.tsx`, `inventory-skeleton.tsx` | Same — Installed scope owns this |
| Route tests: `web/src/routes/_app/__tests__/-skills.test.tsx`, `-extensions.test.tsx`, `-mcp.test.tsx` | Assertions move to the new subroute tests |

Every deletion lands in the same PR as its replacement — no dual routes, no aliases.

### 3.2 ADD

| Path | Purpose |
| --- | --- |
| `web/src/routes/_app/marketplace.skills.tsx`, `marketplace.mcps.tsx`, `marketplace.extensions.tsx`, `marketplace.bundles.tsx` | Thin child routes rendering the shared kind page (kind as path segment; `tab`/`q` as search params) |
| `web/src/routes/_app/marketplace.bundles.activations.$id.tsx` | Moved activation detail (content from `bundle-activation-detail.tsx`) |
| `web/src/systems/marketplace/components/marketplace-kind-page.tsx` | Shared template per §2.1: `PageHead` + `ListingToolbar` (search · Installed\|Marketplace PillGroup) + card grid; per-kind config object (labels, verbs, card variants). No underline scope tabs. |
| `web/src/systems/marketplace/components/marketplace-installed-card.tsx` | Installed-card variants (skill/mcp/extension/bundle action trails) composed on `CatalogCard` |
| `web/src/systems/marketplace/hooks/use-marketplace-kind-page.ts` | Joins catalog (`GET /api/marketplace/{kind}`) + installed inventory per kind; derives `installed`, `updateAvailable`, counts |

### 3.3 MODIFY

| Path | Change |
| --- | --- |
| `web/src/routes/_app/marketplace.tsx` | Becomes the layout route: topbar breadcrumb + kind `RouteNav` + Refresh + `<Outlet/>`; index redirect → `/marketplace/skills`; drop the `kind` search param |
| `web/src/routes/_app/marketplace.$kind.$entryId.tsx` + `systems/marketplace/components/marketplace-detail*.tsx` | Stays the single detail; installed state gains management sections (skill content/shadows from `skill-detail-panel.tsx`, extension sections from `extension-detail*.tsx`, MCP config/status/auth from settings dialogs); Manage → `/marketplace/{kind}?tab=installed` |
| `web/src/systems/marketplace/components/use-marketplace-action-controller.tsx` | Extends beyond install/update/activate with remove, deactivate, enable/disable, authorize — delegating to the existing inventory mutations (`use-extension-actions.ts`, `use-skill-actions.ts`, settings MCP adapters) |
| `web/src/systems/marketplace/components/marketplace-card.tsx`, `marketplace-entry-actions.tsx`, `marketplace-grid.tsx` | Marketplace-scope cards; grid stays cards-only (no rows switch) |
| `web/src/systems/runtime/components/app-sidebar.tsx` | `CATALOG_NAV_ITEMS` (~lines 270–277): remove Extensions, Skills, MCP entries; keep Marketplace, Bridges, Knowledge |
| `web/src/systems/skill/`, `web/src/systems/extensions/`, settings MCP adapters/dialogs | **Data layers and dialogs are kept** — they feed the Installed scope and the detail; only their page-level components die |

### 3.4 Data layer

- **No new endpoints.** Installed scope: `GET /api/skills?workspace=`, `GET /api/extensions`, `GET /api/bundles/activations`, `GET /api/settings/mcp-servers`. Marketplace scope: `GET /api/marketplace/{kind}` (limit 100). Refresh: `POST /api/marketplace/refresh?kind=`. Mutations unchanged (`/api/skills/marketplace/*`, `/api/extensions*`, `/api/bundles/*`, `/api/settings/mcp-servers*`).
- `update_available` stays backend-provided. Extensions already embed the listing in the installed record (`use-extensions.ts`); skills/MCP join client-side by name in `use-marketplace-kind-page.ts` for v1. **Backend follow-up:** embed the listing in skills/MCP inventory responses like extensions (Go + OpenAPI + TS co-ship per `agh-contract-codegen-coship`).
- `GET /api/marketplace/search` (cross-kind fan-out) loses its web surface; it remains an agent/CLI surface — note in the endpoint docs, do not delete.
- Query keys unchanged; the kind-page hook is a derived selector over existing caches.

### 3.5 Listing standard exception

`LISTING-STANDARD.md` defaults inventories to rows with a Rows|Cards toggle and a title-in-topbar shell (Vault / Bridges). The marketplace family is an **explicit dual exception**:

1. **Cards only** — no Rows|Cards view toggle.
2. **Chrome** — breadcrumb topbar + PageHead + listing toolbar (§2.1), not the inventory title-in-topbar shell. Trailing PillGroup = **Installed | Marketplace** scope (with icons).

Recorded in `LISTING-STANDARD.md` §When-to-use / §Marketplace chrome; do not add a Rows|Cards toggle or underline scope tabs back.

### 3.6 Tests / stories

- **Rewrite:** `systems/marketplace/components/__tests__/marketplace-components.test.tsx` (kind page: breadcrumb/PageHead, scope PillGroup, search, per-kind cards), route test for the layout + one per subroute, `marketplace-action-controller.test.tsx` (extended verbs).
- **Move:** skill/extension/bundle detail assertions into the marketplace detail tests; activation detail tests to the new route.
- **Keep:** all adapter tests, `use-extensions.test.tsx`, `use-skill-actions.test.tsx`, skill formatter/query-key tests.
- **E2E:** merge `skills.spec.ts`, `mcp.spec.ts`, `extensibility.spec.ts` flows into `marketplace.spec.ts` (per-kind: browse → install → Installed scope → update → remove; MCP authorize; bundle activate/deactivate). Fixtures gain joined-inventory cases.
- **Storybook:** one story set for the kind page (per kind × per scope × states); delete landing/kind-view/inventory stories.

### 3.7 Phasing (one PR each)

1. **Surface** — layout route + four subroutes + kind-page template (§2.1) + Installed scope for skills/extensions/bundles; sidebar slims to one entry; delete `/skills`, `/extensions` routes.
2. **MCP** — Installed scope for MCPs (settings dialogs wired), delete `/mcp`; guided install from cards.
3. **Details** — adaptive detail merges (skill content, extension management, MCP config), activation route move, delete remaining detail routes + landing.

## 4. Copy

- Scope PillGroup labels: `Installed` / `Marketplace` (icons required; bundles Installed shows `active` pills). Kind RouteNav labels: `Skills` · `MCPs` · `Extensions` · `Bundles`.
- Head meta: `N in the marketplace · N installed · N updates available`.
- Installed empty: "No skills installed yet. Everything you install from the marketplace shows up here. You can also use `agh skills install <name>`." (per-kind verb/CLI variants; bundles: activate).
- Toasts: `<name> installed · View installed →`, `<name> updated to vX.Y.Z`, `<name> authorized · server running`, `<name> deactivated`.
- Never render catalog-status banners inside modal chrome.

## 5. QA impact (flag, don't retest)

Reset to `untested`: `docs/qa/scenarios/ET-web-marketplace-landing-browse.md`, `ET-web-marketplace-search-fanout.md` (web fan-out surface removed — rescope or retire), `ET-web-extensions-manage.md`, `ET-web-bundle-activation-detail.md`. New content-addressed `untested` scenarios: marketplace kind RouteNav + default entry, Installed-scope management per kind, MCP authorize from Installed, remove-returns-to-marketplace.

## 6. Web/Docs impact

`packages/site` docs that reference `/skills`, `/mcp`, `/extensions`, or `?kind=` marketplace URLs must be updated to the subroute model. Official AGH skill (`skills/agh/`): update route guidance for install/manage flows.

## 7. AGH Impact Audit

- **Native tools:** no impact — no `agh__*` IDs, descriptors, schemas, or capability gates change; web-only IA.
- **Extensibility and hooks:** all install/manage endpoints and dialogs unchanged; `GET /api/marketplace/search` remains an agent/CLI surface without a web page. Backend listing-embed follow-up is contract-additive with OpenAPI co-ship.
- **Workspace data isolation:** unchanged — workspace scoping of skills inventory and MCP scope handling carries into the Installed scope; query keys stay workspace-aware.
- **Official AGH skill:** update `skills/agh/` route references (§6).

## 8. Acceptance gates

- Prototype parity with `marketplace.html` across the four kinds × two scopes, including §2.1 chrome (`agh-ui-screenshot` captures).
- `make verify` per PR; merged e2e lane green.
- No page-body underline `Marketplace | Installed` tabs; no Rows|Cards toggle on kind pages.

## 9. Open questions

1. Sidebar Catalog group naming — with one entry left, keep the `Catalog` section label or fold Marketplace/Bridges/Knowledge under an existing group?
2. Bundle activation deep view — keep as its own route (spec default) or render as a sheet over the Bundles Installed scope?
3. Backend listing-embed for skills/MCP (§3.4) — schedule now or after phase 3?
