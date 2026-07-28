# Integrations & Guides

Source URLs (verified May 2026): `fumadocs.dev/docs/integrations/{openapi,feedback,llms,obsidian,og,python,story,typescript,validate-links,content}`, `/docs/integrations/openapi/{api-page,generate-files,server,without-rsc}`, `/docs/integrations/og/{next,takumi}`, `/docs/integrations/story/{next,vite}`, `/docs/integrations/content/{custom,local-md,mdx-remote}`, `/docs/guides/{access-control,customize-ui,export-epub,export-pdf,rss}`.

## OpenAPI

Replace hand-written API docs with an OpenAPI/Swagger spec when you want an interactive playground, code samples, generated TS types, and consistent rendering.

### Pieces

| Piece | Module | Purpose |
| --- | --- | --- |
| `createOpenAPI({ input })` | `fumadocs-openapi` | Server instance. **Server-only — never import in browser.** Accepts file paths, external URLs, or a function. Supports `proxyUrl` to forward CORS-blocked playground requests via `openapi.createProxy({ allowedOrigins })`. |
| `<APIPage />` | `fumadocs-openapi` | RSC component built via `createAPIPage(openapi, { client, codeUsages, mediaAdapters })`. Client/server configs split when running under RSC; non-RSC merges them. Supports `x-codeSamples` and i18n via `defineI18nOpenAPI`. |
| `generateFiles({ input, output, per, groupBy, ... })` | `fumadocs-openapi` | Emits MDX. `per`: `tag \| file \| operation \| custom`. `groupBy`: `tag \| route \| none`. `meta: true` produces `meta.json`. |
| `openapiSource()` + `<ClientAPIPage />` | `fumadocs-openapi/ui` | **Without RSC** path. Render via `useFumadocsLoader` and `page.data.getClientAPIPageProps()`. **Required for Vite-based frameworks** (TanStack Start, React Router, Waku). `shiki` must be installed so Vite externalizes WASM regex. |

### MDX vs virtual files

- `generateFiles()` writes real `*.mdx` to `content/docs`.
- `openapiSource()` integrates with `loader()` for dynamic generation (page tree updates as schema changes) but **changes `page.type` to `'openapi'`** — every consumer (`getLLMText`, page renderer, search index, OG image, RSS) must branch on it.

### Spec input

YAML or JSON. Pass an array, URL, or `() => Promise`. Use `x-displayName` on tags for nicer titles. Bundle/dereference happens server-side; expose via `page.data.getSchema().bundled` for LLM payloads.

### Styling

```css
@import 'tailwindcss';
@import 'fumadocs-ui/css/neutral.css';
@import 'fumadocs-ui/css/preset.css';
@import 'fumadocs-openapi/css/preset.css'; /* must come AFTER the UI preset */
```

### Code generators

```ts
import { createCodeUsageGeneratorRegistry } from 'fumadocs-openapi/requests/generators';
import { registerDefault } from 'fumadocs-openapi/requests/generators/all';

const codeUsages = createCodeUsageGeneratorRegistry();
registerDefault(codeUsages);
codeUsages.add('myLang', { label: 'MyLang', lang: 'mylang', generate: ({ method, path }) => '...' });
```

### Media adapters

Implement `encode()` + `generateExample()` to support custom request bodies (binary, form, custom JSON shapes).

### Pitfalls

- **Build cost** — large specs explode `generateFiles()` outputs and the build graph. Prefer `openapiSource()` and use `per: 'tag' \| 'file'` to coarsen output.
- **Page-type drift** — switching to `openapiSource()` mutates `page.type`. Run `npm run typecheck` after wiring; every consumer must branch on `page.type === 'openapi'`.

## Content sources

| Source | Module | Use when |
| --- | --- | --- |
| **Fumadocs MDX** | `fumadocs-mdx` | Default. Bundler-backed, type-gen, build-time image optimization, full MDX `import`/`export` |
| **`@fumadocs/local-md`** | `@fumadocs/local-md` | Runtime-only loader for local `.md`/`.mdx`. **No bundler/typegen, no image optimization, no MDX imports/exports.** Best for thin runtime, Cloudflare Workers compat (virtual JS engine, no `eval`), or to drop `source.config.ts`. Hot reload via `local-md dev -- <cmd>` + `docs.devServer()` |
| **`@fumadocs/mdx-remote`** | `@fumadocs/mdx-remote` | On-demand MDX compiler (`createCompiler().compile({ source })`). **Trusted input only** (executes code). No imports/exports. On Vercel, public images may not resolve post-build — use absolute URLs |
| **custom (`StaticSource` / `DynamicSource`)** | `fumadocs-core/source` | Hardcoded trees, generated content, multi-tenant per-permission sources. Pre-render hot pages with `generateStaticParams` and **clone remote sources locally before build to avoid rate limits** |

### Typed frontmatter (for runtime sources)

```ts
import { localMd } from '@fumadocs/local-md';
import { pageSchema, metaSchema } from 'fumadocs-core/source/schema';

const docs = localMd({
  dir: 'content/docs',
  frontmatterSchema: pageSchema.extend({ /* ... */ }),
  metaSchema: metaSchema.extend({ /* ... */ }),
});
```

Schemas surface as type-safe `page.data.<field>` and are visible to `update().files(...)` filters used for access control.

### Migration pitfalls

- When migrating from Fumadocs MDX to `local-md`, **drop `source.config.ts` and `createMDX()` config in the same change** — leaving them creates two competing loaders.
- Page data shape changes: `page.data.body` becomes a render function, `getText('processed')` returns a promise — update every consumer.

## OG image generation

### `next/og`

```tsx
// app/og/docs/[...slug]/route.tsx
import { ImageResponse } from 'next/og';
import { generate as DefaultImage } from 'fumadocs-ui/og';
import { source } from '@/lib/source';

export async function GET(req: Request, { params }: { params: { slug: string[] } }) {
  const page = source.getPage(params.slug);
  if (!page) return new Response('Not found', { status: 404 });
  return new ImageResponse(<DefaultImage {...page.data} />, { width: 1200, height: 630 });
}

export function generateStaticParams() {
  return source.getPages().map((page) => ({ slug: [...page.slugs, 'image.png'] }));
}
```

Wire URL into `generateMetadata().openGraph.images` via a `getPageImage(page)` helper that appends `image.png` to `page.slugs`. Other presets via `npx @fumadocs/cli add og/mono`.

### Takumi (faster, Rust)

Install `@takumi-rs/image-response` and add to `serverExternalPackages` (Next), `external` (Vite/Nitro/Waku). Use `generate as DefaultImage` from `fumadocs-ui/og/takumi` and prefer `format: 'webp'` (`image.webp` segment). Bundles Geist + Geist Mono full-axis fonts.

### Pitfalls

- **`next/og`** runs Satori in serverless/edge — heavy fonts and Tailwind classes don't apply.
- **Takumi must be added to `serverExternalPackages` / `external`** of your bundler, otherwise builds fail. Cannot be edge-bundled.
- `revalidate = false` on the route is mandatory or you'll re-render every request.

## AI / LLM integrations

### `llms.txt` / `llms-full.txt`

```ts
import { llms } from 'fumadocs-core/source';
import { source } from '@/lib/source';

// app/llms.txt/route.ts
export function GET() {
  return new Response(llms(source).index(), { headers: { 'Content-Type': 'text/plain' } });
}
```

`llms-full.txt` concatenates per-page `getLLMText(page)` — uses `page.data.getText('processed')`, requires `includeProcessedMarkdown: true` in `defineDocs.postprocess`.

### Per-page `*.mdx`

Route handler at `/llms.mdx/docs/[[...slug]]` returning `text/markdown`, plus a Next `rewrites()` (`/docs/:path*.mdx -> /llms.mdx/docs/:path*`).

### `Accept` header negotiation

`fumadocs-core/negotiation` `isMarkdownPreferred` + `rewritePath` in `proxy.ts`:

```ts
if (isMarkdownPreferred(request)) {
  return rewritePath(request.nextUrl.pathname, '/llms.mdx');
}
```

### Page Actions

`@fumadocs/cli add ai/page-actions` gives `<LLMCopyButton />` + `<ViewOptions />` (markdown URL + GitHub source link).

### Ask AI (chat)

- `@fumadocs/cli add ai/openrouter` — Vercel AI SDK + OpenRouter, `/search` tool.
- `@fumadocs/cli add ai/inkeep` — `INKEEP_API_KEY`.

Mount `<AISearch><AISearchPanel/><AISearchTrigger/></AISearch>` inside the docs layout.

## Story / Storybook

`@fumadocs/story` is a docs-focused alternative to Storybook for component galleries embedded inside docs. Storybook is still preferred for full visual regression / interaction tests.

```tsx
// docs/components/button.story.tsx
import { defineStoryFactory, defineStory } from '@fumadocs/story';

export const story = defineStory(import.meta.url, {
  Component: Button,
  args: { children: 'Click me' },
});
```

Next.js path: install `@fumadocs/story`, add `@import '@fumadocs/story/css/preset.css';`, and use `createFileSystemCache('.next/fumadocs-story')` in production (mandatory on Vercel).

## TypeScript intellisense

- `<AutoTypeTable path="..." name="...">` (`fumadocs-typescript/ui`) — interactive prop tables. **Server Component only.**
- `auto-type-table` MDX directive via `remarkAutoTypeTable` (build-time).
- Cache: `createGenerator({ cache: createFileSystemGeneratorCache('.next/fumadocs-typescript') })`.
- TSDoc tags:
  - `@internal` — hide.
  - `@remarks \`Foo\`` — simplified type.
  - `@fumadocsType \`Foo\`` — rename.
  - `@fumadocsHref #anchor` — link to other table.
- Always pair with `<TypeTable>` MDX component for hand-written tables.

## Feedback widget

```bash
npx @fumadocs/cli@latest add feedback
```

`<Feedback onSendAction>` for page-level (server actions OK). Add `remarkFeedbackBlock` for paragraph-level `<FeedbackBlock>` UI; block IDs are content-derived. Reference impl forwards both shapes to GitHub Discussions via Octokit GraphQL.

**Block IDs are content-hash + position based** — re-ordering a paragraph silently changes the ID, severing continuity in 3rd-party stores (PostHog/GitHub Discussions). Treat as expected on heavy edits.

## Validate links

```bash
bun ./scripts/lint.ts
```

```ts
import { scanURLs, validateFiles } from 'next-validate-link';
import { source } from '@/lib/source';
import { docs } from 'collections/server';

const scanned = await scanURLs({ preset: 'next', populate: { 'docs/[[...slug]]': source.getPages().map(...) } });
await validateFiles({ scanned, markdown: { components }, checkRelativePaths: 'as-url' });
```

`source` is normally only available inside the framework runtime. Configure `fumadocs-mdx` Bun loader (or Node loader / `tsx`) so the script can `await import('@/lib/source')`.

## Access control

### Loader-level filter

```ts
import { loader } from 'fumadocs-core/source';
import { update } from 'fumadocs-core/source/update';

export function createSource(permission: 'public' | 'admin') {
  const filtered = update(docs.toFumadocsSource())
    .files((files) => files.filter((f) => f.type === 'meta' || canSee(f.data, permission)))
    .build();
  return loader({ source: filtered, baseUrl: '/docs' });
}
```

Filter `meta` files **in** (`type === 'meta'`). Cache per-permission sources in-memory.

### Custom (per-route)

Load page in route, compare `page.data.permission` to `getUser().permission`, otherwise `notFound()`. You then own page tree, search index, and SEO yourself.

## Customize UI (escape hatches in priority order)

1. **Props** — `<DocsLayout sidebar={{ enabled: false }} containerProps={{ className }} />`.
2. **CSS via documented selectors** — `id` / `data-*` (`#nd-docs-layout`, `[data-toc-popover]`). **Avoid `> div` chains** — DOM structure is not part of the public contract.
3. **CLI installs** — `@fumadocs/cli add layouts/docs` and `@fumadocs/cli customize` for granular slot installation. Once installed, **import the layout type from your local copy**, not `fumadocs-ui`. Trade-off: no automatic UI updates.

## Export PDF

```ts
import puppeteer from 'puppeteer';
const page = await browser.newPage();
await page.goto('/docs/...');
const pdf = await page.pdf({ width: 950, printBackground: true });
```

Hide nav with print CSS:

```css
@media print {
  #nd-docs-layout { --fd-sidebar-width: 0px !important }
  #nd-sidebar { display: none }
}
```

For accordions / tabs, swap MDX components in printing mode so hidden content is visible.

## Export EPUB

```bash
npm install fumadocs-epub
```

Requires `includeProcessedMarkdown: true` in `defineDocs.postprocess`.

```ts
import { exportEpub } from 'fumadocs-epub';
const buf = await exportEpub({
  source,
  title: '...',
  author: '...',
  cover: '...',
  includePages: ['*'],
  excludePages: [],
  css: '...',
  publicDir: 'public',
});
```

Serve via route handler with `application/epub+zip`. Protect endpoint (`Authorization: Bearer EXPORT_SECRET`).

Image resolution: relative paths (relative to MDX), `/public/...` (resolved via `publicDir`), and remote URLs (embedded as-is).

CLI: `fumadocs export epub --framework next` (or `--scaffold-only`).

## RSS

```ts
import RSS from 'feed';
import { source } from '@/lib/source';

export function GET() {
  const feed = new RSS.Feed({ /* ... */ });
  for (const page of source.getPages()) {
    feed.addItem({
      id: page.url,
      title: page.data.title,
      description: page.data.description,
      link: `https://example.com${page.url}`,
      date: page.data.lastModified,
    });
  }
  return new Response(feed.rss2(), { headers: { 'Content-Type': 'application/rss+xml' } });
}
```

Add `metadata.alternates.types['application/rss+xml']` in root layout.

## Misc integrations

- **Python codegen** (experimental): `pip install ./node_modules/fumadocs-python` → `fumapy-generate <pkg>` → JSON → `Python.convert()` + `Python.write()` to MDX. Doc strings must be MDX-safe.
- **Obsidian** (experimental): copy vault, run `fromVault({ dir, out })` from `fumadocs-obsidian`. Add `fumadocs-obsidian/ui` components to `getMDXComponents()`. Mermaid/math need separate enablement.
