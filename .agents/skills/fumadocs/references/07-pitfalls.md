# Pitfalls Catalogue

Consolidated from `fumadocs.dev/docs/*` (verified May 2026). Cross-references in parentheses.

## Stack-wide

- **Tailwind v3 silently breaks the theme.** Only Tailwind v4 + `fumadocs-ui/css/preset.css` is supported. Make sure your `@source` glob covers `app/`, `components/`, and any vendored files — utilities are tree-shaken otherwise. (`references/04-ui-layouts-and-components.md`)
- **Edge runtime is unsupported.** Required for Next.js + Cloudflare via OpenNext (`https://opennext.js.org/cloudflare`); never raw Cloudflare Pages "Edge". (`references/01-install-and-setup.md`)
- **Provider import path mismatch is silent.** `fumadocs-ui/provider/<framework>` must match the adapter or `RootProvider` silently no-ops — search dialog, theme switcher, i18n, hot keys all stop working. (`references/04-ui-layouts-and-components.md`)
- **`<body>` needs `flex flex-col min-h-screen`.** Missing it breaks layout height on every adapter.
- **`suppressHydrationWarning` belongs on `<html>`** — theme switch hydrates client-side.
- **No icon library is shipped.** `icon` strings in frontmatter / `meta.json` are inert until you pass an `icon` handler to `loader()` that maps names to JSX. (`references/02-source-and-page-tree.md`)

## Source / page tree

- **Slug collisions are fatal.** Duplicate URLs anywhere in the tree corrupt active-route detection. Wrap re-organised folders with `(group)` or override slugs in `loader({ slugs })`. (`references/02-source-and-page-tree.md`)
- **Trees are serialised payloads.** Don't stash functions, large blobs, or secrets on tree nodes; UI receives them on the client. For non-RSC clients, call `source.serializePageTree(tree)`.
- **`StaticSource` paths must be virtual.** `./file.mdx`, `D://content/...`, or other absolute paths are rejected.
- **Dynamic sources are cached.** Call `source.revalidate()` (or `source.revalidate('docs')` for multi-source) when upstream content changes; otherwise stale pages persist across requests.
- **`pages` filters siblings.** Once you list any item, only listed items render — don't forget `...` for "everything else".
- **Folder groups vs root folders are different concepts.** `(group)` removes the folder from the slug; `root: true` toggles "isolated tab" mode. Don't conflate.
- **Default plugins already on in Fumadocs MDX.** `remarkHeading` and `remarkImage` are pre-wired; adding them again duplicates ids/imports.

## MDX pipeline (`fumadocs-mdx`)

- **`fumadocs-mdx` is ESM only.** Use `next.config.mjs` (or enable Native Node TS resolver for `.ts`). A CommonJS `next.config.js` will fail to load. (`references/03-mdx-pipeline.md`)
- **`.source/` is generated.** Created on `dev` / `build`; otherwise prime via `npx fumadocs-mdx` (or `postInstall(...)` for Bun). Missing folder = no collection types, no entries.
- **Don't import from `.source/` directly.** Use the `collections/*` alias.
- **Collection `mdxOptions` discards globals.** No merge — wrap with `applyMdxPreset(...)` if you need the docs preset back.
- **`schema` runs at build time.** Outputs must be serialisable. Use the function form to read `ctx.path` etc.
- **Dynamic mode constraints.** No `import` / `export` in MDX bodies; images must be URL strings (no `./relative.png`). Pass components via the `components` prop.
- **Last-modified plugin needs full Git history.** Shallow clones (CI default) break it — `fetch-depth: 0` in GitHub Actions.
- **Bundler ceiling around ~500 MDX files.** Beyond that switch to `async`/`dynamic` mode or a custom remote source.
- **Turbopack ≠ Webpack** for `async: true` — only the server-side win is realized; bundling is still eager.
- **Browser entry only sees `doc`/`docs`.** Other collection types stay server-only.
- **Workspaces don't inherit.** Each workspace config is independent; `cwd` is workspace-relative; outputs land at `.source/{workspace}/`.
- **Markdown `<include>` requires `remark-directive`** (and frequently `rehype-raw`); MDX uses JSX form directly.
- **Plugin order is load-bearing.** `rehype-katex` before syntax highlighter; twoslash transformer after Shiki defaults. Use the function form `(v) => [...]` whenever order matters.

## UI / Layouts

- **Conflicting typography plugin.** Stacking `@tailwindcss/typography` over Fumadocs' built-in plugin doubles `prose` styles — set `className: wysiwyg` on the third-party plugin.
- **Missing `RootProvider`.** Without it (or with mismatched framework subpath like `fumadocs-ui/provider/next` on a TanStack app) search dialog, theme switcher, i18n, hot keys all silently no-op.
- **Layout/page import mismatch.** Using Notebook or Flux requires importing `DocsPage` from `fumadocs-ui/layouts/notebook/page` (or `flux/page`) — defaulting to `docs/page` produces broken TOC/footer.
- **Page-tree mismatch.** When `tree={source.getPageTree()}` doesn't match your routing (wrong `loader()`, missing `meta.json`, locale mismatch), sidebar items disappear or render in the wrong tab.
- **Custom navbar height.** Replacing `nav.component` without overriding `--fd-nav-height` desyncs the grid; sticky elements (`--fd-docs-row-*`) compute the wrong top offset.
- **Banner's `changeLayout`.** Default `true` injects a `<style>` block — combined with custom heights it can clip the sidebar. Either set `changeLayout={false}` and adjust `--fd-banner-height` manually, or accept the injected styles.
- **Flux is a client component.** You cannot pass functions or non-serialisable props from a Server Component into the Flux layout. Pair with **static/local search** — server-side fetch search is degraded there.
- **AutoTypeTable in client components.** Server-only. Use the build-time MDX integration (`fumadocs-typescript`) for client surfaces.
- **`CodeBlock`'s `ref` collision.** React 19 forwards `ref` as a normal prop on `pre`; spreading it into `CodeBlock` breaks `forwardRef` — strip with `({ ref: _ref, ...props })`.
- **Locale toggle layout shift.** Translations swap labels — use `useI18n()` instead of hard-coded strings, otherwise the navbar reflows on locale change.
- **Tabs without `value`.** Identifying tabs by index breaks on re-render; always provide `value` (and `id` for hash deep-links). `groupId` + `persist` is required for cross-page sticky tab state.
- **`prefetch` defaults.** Layout `<Link>` prefetches aggressively; on Vercel this inflates Data Cache and serverless invocations — set `sidebar.prefetch: false` when needed.
- **RTL.** Set `dir="rtl"` on **both** `<body>` and `<RootProvider>` — half-applied RTL leaves the sidebar in LTR.
- **Base UI vs Radix.** Default is Radix; switching to `@fumadocs/base-ui` requires `package.json` alias plus `cli.json: { "uiLibrary": "base-ui" }` for the CLI to scaffold matching primitives.

## Search / i18n

- **Search adapter mismatch.** `useDocsSearch({ type: 'fetch' })` only matches the Orama API route; switching to Algolia/Trieve/Mixedbread requires changing **both** the server route and the client `type`.
- **Provider boundary.** `useDocsSearch` lives in a client component. If `RootProvider` is rendered from a server tree, wrap it in a `'use client'` provider so dialog functions can cross.
- **Locale slug collisions.** When `[lang]/` and a literal route share a top-level segment (e.g. `app/api`), the i18n middleware matcher must exclude `api`, `_next/static`, `_next/image`, and any custom static folders.
- **`hideLocale: 'always'` cookie.** Bad for fully static hosting and search indexing; prefer `'default-locale'` unless you control the cache layer.
- **Static search drift.** Static modes index at build; CMS / OpenAPI / Obsidian / Python pipelines that change *between* builds will silently miss content.
- **Static search needs both ends configured.** Wiring only the server or only the UI yields silent runtime errors.
- **Partial trees on i18n.** When a translation file is missing, the page silently falls back to `fallbackLanguage`; if you set `fallbackLanguage: null`, expect 404s for untranslated pages.

## Integrations

- **OpenAPI build cost.** Very large specs explode `generateFiles()` outputs and the build graph. Prefer `openapiSource()` and use `per: 'tag' \| 'file'` to coarsen output. Pre-bundle the spec server-side; the `getSchema().bundled` payload is needed by `getLLMText` and search.
- **OpenAPI page-type drift.** Switching to `openapiSource()` mutates `page.type` to `'openapi'`. Every consumer (`getLLMText`, page renderer, OG image, search index, RSS) must branch — run `npm run typecheck` after wiring.
- **OG runtime mismatch on edge.** `next/og` runs Satori in serverless/edge — heavy fonts and Tailwind classes don't apply. Takumi runs Rust/native and **must** be added to `serverExternalPackages` / `external` of your bundler, otherwise builds fail. On Vercel, `@takumi-rs/image-response` cannot be edge-bundled. `revalidate = false` on the route is mandatory or you'll re-render every request.
- **`mdx-remote` and public assets.** On Vercel the runtime `public/` folder is stripped post-build; relative image paths in remotely-fetched MDX won't resolve. Use absolute URLs.
- **Local MD migration.** When migrating from Fumadocs MDX, drop `source.config.ts` and `createMDX()` config in the same change — leaving them creates two competing loaders. Page data shape changes (`page.data.body` becomes a render function, `getText('processed')` returns a promise) — update every consumer.
- **`@fumadocs/local-md` virtual JS engine** is slower than native; for large docs sets that don't need Worker-edge compatibility, prefer Fumadocs MDX.
- **Validate-links runtime.** `source` is normally only available inside the framework runtime. Configure `fumadocs-mdx` Bun (or Node) loader to run lint scripts; `tsx` works as fallback.
- **Feedback block IDs** are content-hash + position based — re-ordering a paragraph silently changes the ID, severing continuity in 3rd-party stores (PostHog/GitHub Discussions). Treat as expected on heavy edits.
- **`*.mdx` rewrites** require **both** the route handler and the Next `rewrites()` (or framework equivalent) — installing only one half makes `Accept`-based negotiation 404 silently.
- **Storybook vs Story.** Fumadocs Story is for docs galleries only; do not rely on it for visual regression / interaction tests.

## Deploy

- **Docker builds break** unless `source.config.ts` + `next.config.*` are present at the install step (deps stage). Copy them alongside `package.json` / lockfiles.
- **SPA mode for React Router/TanStack Start requires explicit pre-render lists** — pages not reachable from the visible UI must be added manually.
- **External search hosts (Algolia / Mixedbread / Orama-Cloud) require explicit index sync** — usually a separate prebuild step (`/static.json` route or build script). They will not auto-index from your local content.
