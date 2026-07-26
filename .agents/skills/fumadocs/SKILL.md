---
name: fumadocs
description: Authoritative guide for the Fumadocs documentation framework — Core (headless), MDX (content loader), and UI (Tailwind v4 layouts and components). Use when scaffolding a Fumadocs project, editing source.config.ts, wiring loader() and meta.json conventions, picking a search adapter, composing DocsLayout/Notebook/Flux/Home layouts, customising the MDX component map, integrating OpenAPI specs, generating OG images, exporting PDF/EPUB/RSS, or configuring i18n across Next.js, React Router, TanStack Start, and Waku adapters. Surfaces canonical install snippets per adapter, decision tables for entry modes and content sources, and the most common pitfalls (Tailwind v3 silent failure, ESM-only config, .source/ generation, layout/page import mismatch, slug collisions, edge runtime ban). Don't use for generic MDX/Markdown questions outside Fumadocs, for unrelated React framework setup, or for projects that don't use fumadocs-core/fumadocs-mdx/fumadocs-ui.
---

# Fumadocs

Fumadocs is a docs framework for React: it sits **inside** Next.js / React Router / TanStack Start / Waku and combines three independently usable packages.

| Package | Role | Skip when |
| --- | --- | --- |
| `fumadocs-core` | Headless engine: `loader()`, page tree, MDX plugins, search adapters, i18n primitives | never (always required) |
| `fumadocs-mdx` | Content source: compiles `content/**/*.mdx` + `meta.json` into typed `.source/` collections | replaced by `@fumadocs/local-md`, `@fumadocs/mdx-remote`, or a custom source |
| `fumadocs-ui` | Opinionated theme: `RootProvider`, layouts, MDX defaults, search dialog (Tailwind v4) | building a fully bespoke UI on top of `fumadocs-core` |

Server-first via React Server Components. Static export is opt-in. **Edge runtime is unsupported.**

## Procedures

**Step 1: Identify the task class**

Map the user's request to one of these classes, then load the matching reference:

| Class | Trigger phrases | Reference |
| --- | --- | --- |
| Bootstrap a project | "scaffold", "set up Fumadocs", "install in <framework>", "create-fumadocs-app" | `references/01-install-and-setup.md` |
| Routing / file conventions | `meta.json`, `pages` directives, slug rules, root folders, page tree | `references/02-source-and-page-tree.md` |
| MDX configuration | `source.config.ts`, `defineDocs`, `defineCollections`, async / dynamic mode, plugin order, typegen | `references/03-mdx-pipeline.md` |
| UI layouts / components / MDX wiring | DocsLayout/Notebook/Flux/Home, `RootProvider`, `mdx-components.tsx`, theming, MDX overrides | `references/04-ui-layouts-and-components.md` |
| Search / i18n | adapter selection, search dialog, locale routing, middleware | `references/05-search-and-i18n.md` |
| OpenAPI / OG / AI / content sources / guides | `<APIPage>`, `generateFiles`, OG image route, `llms.txt`, Ask AI, `local-md`, `mdx-remote`, PDF/EPUB/RSS | `references/06-integrations.md` |
| Debugging an existing setup | "broken", "blank sidebar", "404", "build fails", "weird hydration" | `references/07-pitfalls.md` |

If the task spans multiple classes, load the references in the order shown above (setup → tree → MDX → UI → search/i18n → integrations).

**Step 2: Confirm the package surface in use**

Before recommending code, check which Fumadocs surface the project actually uses. Read these files (paths are conventional, not absolute):

1. `package.json` — confirm `fumadocs-core`, `fumadocs-mdx`, `fumadocs-ui` versions and which adapter is installed (`next`, `@react-router/...`, `@tanstack/react-start`, `waku`).
2. `source.config.ts` (Fumadocs MDX) **or** `lib/source.ts` (low-level `loader()`) — determines whether the project uses the bundler-backed MDX path or a runtime source (`local-md`, `mdx-remote`, custom).
3. The catch-all docs route — Next: `app/<segment>/[[...slug]]/page.tsx`; React Router: `routes/docs/$.tsx` or `route('docs/*', ...)`; TanStack Start: `routes/docs/$.tsx`; Waku: `pages/docs/[...slugs].tsx`.
4. The root layout / provider import path — `fumadocs-ui/provider/{next,react-router,tanstack,waku}` MUST match the framework.
5. Tailwind entry — confirm Tailwind 4, the colour preset (`fumadocs-ui/css/<theme>.css`) and `fumadocs-ui/css/preset.css` imports.

If any of these contradicts the user's request, surface the conflict before editing.

**Step 3: Choose adapters and modes deterministically**

Use these decision tables instead of guessing:

### MDX entry mode (Fumadocs MDX → `collections/{server,browser,dynamic}`)

| Choose | When |
| --- | --- |
| `server` | RSC / SSR / SSG (default, fastest first paint) |
| `browser` | Client-routed apps (TanStack Start, React Router SPA mode) — only `doc` / `docs` collections |
| `dynamic` | Very large libraries (>500 MDX) where build time / memory dominates |
| direct `import` | One-off MDX as a page or React component (no `loader()` indirection) |

### Content source

| Choose | When |
| --- | --- |
| Fumadocs MDX (default) | Bundler-backed, typegen, image optimization, full MDX `import`/`export` |
| `@fumadocs/local-md` | Runtime-only, Cloudflare Workers compat, no `eval`, no bundler step |
| `@fumadocs/mdx-remote` | Runtime compile from CMS / remote content (trusted input only) |
| custom (`StaticSource` / `DynamicSource`) | Hardcoded trees, generated content, multi-tenant per-permission sources |

### Search adapter

| Choose | When |
| --- | --- |
| `orama` (default) | Self-hosted, free, typed schema, vector capable |
| `flexsearch` | Tiny / medium docs, zero infra, smaller bundle than Orama |
| `algolia` | Enterprise scale, polished relevance (free tier requires logo) |
| `orama-cloud` | Hosted Orama, scale without ops |
| `mixedbread` | Semantic / vector / natural-language queries |
| `typesense` | OSS scale + faceting (community adapter) |
| `trieve` | RAG / hybrid retrieval (community-maintained) |
| custom | Hand-roll a `/static.json` route from `source.getPages().structuredData` |

### Layout

| Choose | When |
| --- | --- |
| `fumadocs-ui/layouts/docs` | Classic docs site with persistent sidebar |
| `fumadocs-ui/layouts/notebook` | Denser, app-shell feel; supports top-tab navigation |
| `fumadocs-ui/layouts/flux` | Aggressively minimal (client-only — no unserialisable RSC props) |
| `fumadocs-ui/layouts/home` | Marketing / landing pages that share docs chrome |

When the layout changes, **the per-doc page import must follow** (`fumadocs-ui/layouts/<layout>/page` → `DocsPage`, `DocsTitle`, `DocsBody`).

**Step 4: Wire `source.config.ts`, `loader()`, and `mdx-components.tsx`**

These three files form the spine of any Fumadocs setup. Use the canonical templates (read first, then adapt):

1. Read `assets/source-config-template.ts` for the `defineDocs` + `defineCollections` + `defineConfig` shape, schema extension via `pageSchema` / `metaSchema`, and the recommended `tsconfig` alias `collections/* → .source/*`.
2. Read `assets/source-template.ts` for the `loader({ source, baseUrl, url, slugs, icon, i18n, plugins })` shape and the typical `lib/source.ts` exports.
3. Read `assets/mdx-components-template.tsx` for the `getMDXComponents` pattern, the `pre` ref-strip workaround, and `createRelativeLink(source, page)` wiring.

If the project already has these files, prefer surgical edits over rewrites — collection-level `mdxOptions` **wipes** globals, so use `applyMdxPreset(...)` to keep the docs preset.

**Step 5: Validate before claiming done**

For any change that touches MDX content, source config, page conventions, or layouts:

1. Trigger source generation via the project's existing dev script (Fumadocs MDX writes `.source/` on `dev`/`build`; otherwise run `npx fumadocs-mdx`).
2. Run the framework's typecheck — `.source/index.d.ts` must compile cleanly. A `.source/` not regenerated is the most common cause of "module not found" or "property does not exist" errors.
3. Render the affected route(s) — sidebar items match `meta.json.pages`, breadcrumbs/TOC populate, search dialog opens with `⌘K`. Layout-level breakage is silent until rendered.
4. If i18n is configured, check the locale variants: `defaultLanguage` and at least one non-default. `hideLocale: 'always'` cookies break static caches — surface this if applicable.

**Step 6: Use the pitfall catalog before debugging**

When the user reports a bug, **read `references/07-pitfalls.md` first** — most failures are catalogued. The five most common:

1. **Tailwind v3 silently breaks the theme** — only Tailwind v4 + `fumadocs-ui/css/preset.css` is supported.
2. **`next.config.js` (CJS) won't load `fumadocs-mdx`** — the loader is ESM only; rename to `next.config.mjs` or enable Native Node TS resolver.
3. **Layout/page import mismatch** — Notebook/Flux require `DocsPage` from `layouts/<layout>/page`; defaulting to `layouts/docs/page` produces broken TOC/footer.
4. **Duplicated URLs in the page tree** — Fumadocs locates the active node by `pathname` only; any duplicate corrupts active-link detection.
5. **Provider subpath mismatch** — `fumadocs-ui/provider/<framework>` must match the adapter or `RootProvider` silently no-ops (search, theme switch, i18n all stop working).

## Error Handling

* **The user mentions a feature that isn't visible in `package.json`.** Ask which adapter they're on before generating code; never assume Next.js because the docs default to it.
* **The framework adapter is older than Fumadocs 16.** Earlier majors used different provider paths and Tailwind 3 — verify the version and read `references/07-pitfalls.md` for migration notes before editing.
* **`source.config.ts` and a runtime source (`local-md` / `mdx-remote`) coexist.** Two competing loaders is not a supported state — read `references/06-integrations.md#content-sources` and pick one before changing code.
* **Static export is required.** Confirm whether search needs to be `static` (Orama static, FlexSearch static, or hosted/cloud) — server-fetch search will not work statically. See `references/05-search-and-i18n.md#static-export`.
* **`.source/` does not exist or is stale.** Trigger source generation before any typecheck; missing entries cause cryptic "module not found" / "is not exported" errors.
* **OpenAPI is requested.** `openapiSource()` mutates `page.type` to `'openapi'` — every consumer (`getLLMText`, page renderer, search index, OG image, RSS) must branch on it. Read `references/06-integrations.md#openapi` before wiring.
