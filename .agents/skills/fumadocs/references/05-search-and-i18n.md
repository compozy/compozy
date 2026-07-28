# Search & Internationalization

Source URLs (verified May 2026): `fumadocs.dev/docs/search`, `/docs/search/{algolia,custom,flexsearch,mixedbread,orama,orama-cloud,typesense}`, `/docs/headless/search/*`, `/docs/internationalization`, `/docs/internationalization/{next,react-router,tanstack-start}`, `/docs/headless/internationalization/{config,middleware}`.

## Search adapters — pick by need

| Adapter | Hosting | Sweet spot | Module |
| --- | --- | --- | --- |
| `orama` | Self-hosted | Default. Typed schema, vector capable, free | `fumadocs-core/search/server` (`createFromSource` / `createSearchAPI('advanced', ...)`) |
| `flexsearch` | Self-hosted | Tiny / medium docs, smallest bundle | `fumadocs-core/search/flexsearch` (`flexsearchFromSource(source)`) |
| `algolia` | Hosted | Enterprise scale, polished relevance (free tier requires logo) | `fumadocs-core/search/algolia` |
| `orama-cloud` | Hosted | Scale without ops | `fumadocs-core/search/orama-cloud` |
| `mixedbread` | Hosted | Semantic / vector / NL queries | `fumadocs-core/search/mixedbread` |
| `typesense` | Self-hosted | OSS scale + faceting (community adapter) | `typesense-fumadocs-adapter/client` |
| `trieve` | Hosted | RAG / hybrid retrieval | `fumadocs-core/search/trieve` |
| custom | — | Hand-roll a `/static.json` route from `source.getPages().structuredData` | `fumadocs-core/search/server` + framework route |

Client side is uniform: `useDocsSearch({ type: 'fetch' | 'algolia' | 'static' | ..., locale })` from `fumadocs-core/search/client`.

## Server route conventions

### Next.js (`app/api/search/route.ts`)

Keep it **outside `[lang]/`** so middleware doesn't rewrite it.

```ts
import { source } from '@/lib/source';
import { createFromSource } from 'fumadocs-core/search/server';
export const { GET } = createFromSource(source);
```

### React Router

```ts
// app/routes.ts
import { type RouteConfig, route } from '@react-router/dev/routes';
export default [
  route('api/search', 'routes/search.ts'),
] satisfies RouteConfig;
```

### TanStack Start

`routes/api/search.ts` — server function or `+server` route.

### Static export (no server)

Use `type: 'static'`. Run `create()` from `@orama/orama` client-side; the index ships as a JSON asset. For FlexSearch, `flexsearchStaticClient`.

## Custom search index

Export `structuredData` per page (`source.getPages()`) to a static JSON route (e.g. `app/static.json/route.ts`), then read at build/runtime:

```ts
// app/static.json/route.ts
import { source } from '@/lib/source';
export function GET() {
  return Response.json(
    source.getPages().map((page) => ({
      id: page.url,
      url: page.url,
      title: page.data.title,
      description: page.data.description,
      structuredData: page.data.structuredData,
    })),
  );
}
```

## Wiring the dialog

Every adapter shows the same pattern — build a `<SearchDialog>` client component using `useDocsSearch`, then inject via `<RootProvider search={{ SearchDialog }}>`. If `RootProvider` lives in a server tree, wrap it in a small `'use client'` `Provider` so functions can cross.

```tsx
'use client';
import { useDocsSearch } from 'fumadocs-core/search/client';
import { /* primitives */ } from 'fumadocs-ui/components/dialog/search';

export function CustomSearchDialog(props) {
  const { search, setSearch, query } = useDocsSearch({ type: 'fetch' });
  return (
    <SearchDialog open={props.open} onOpenChange={props.onOpenChange}>
      <SearchDialogOverlay />
      <SearchDialogContent>
        <SearchDialogHeader>
          <SearchDialogIcon />
          <SearchDialogInput value={search} onValueChange={setSearch} />
          <SearchDialogClose />
        </SearchDialogHeader>
        <SearchDialogList items={query.data} />
      </SearchDialogContent>
    </SearchDialog>
  );
}
```

## Tag / category filter

Pass `links` (preset filter chips) on the `search` prop, or render `<TagsList>` + `<TagsListItem>` inside `<SearchDialogFooter>`:

```tsx
const { search, setSearch, query, tag, setTag } = useDocsSearch({ type: 'fetch', tag });
```

## Indexing semantics

- **Server modes** (`fetch`, `orama` server, `algolia`, etc.) index **lazily on first request** from `source.getPages().structuredData`; pick up dynamic sources (e.g. `openapiSource()`) automatically.
- **Static modes** serialize the index at build time. **Re-builds are required** to pick up content changes.

## Algolia-specific

`liteClient(appId, apiKey)` + `useDocsSearch({ type: 'algolia', client, indexName, locale })`. Pre-render `/static.json` for index sync.

## i18n configuration

```ts
// lib/i18n.ts
import { defineI18n } from 'fumadocs-core/i18n';

export const i18n = defineI18n({
  defaultLanguage: 'en',
  languages: ['en', 'cn'],
  hideLocale: 'default-locale', // 'always' | 'default-locale' | 'never'
  fallbackLanguage: 'cn', // null disables fallback
  parser: 'dot', // or 'dir'
});
```

Pass to `loader({ i18n })` so every locale gets its own page tree.

### `hideLocale` modes

| Mode | Behaviour |
| --- | --- |
| `'always'` | Rewrites via `NextResponse.rewrite` and stores the locale in a cookie. **Beware static-cache and SEO indexing implications.** |
| `'default-locale'` | Default locale omits the prefix; non-default locales prefix. |
| `'never'` | Every locale prefixes. |

### `fallbackLanguage`

| Value | Behaviour |
| --- | --- |
| `'<lang>'` | Missing translations fall back to that language |
| `null` | No fallback; missing files 404 (use `name.$.md` / `meta.$.json` for shared-across-locales files) |

### Parser modes

- `dot` (default): `get-started.cn.mdx`, `meta.cn.json` (default locale omits suffix).
- `dir`: `content/docs/{locale}/...` directory per locale.

## Middleware — Next.js (`proxy.ts`)

```ts
import { createI18nMiddleware } from 'fumadocs-core/i18n/middleware';
import { i18n } from '@/lib/i18n';

export default createI18nMiddleware(i18n);

export const config = {
  matcher: ['/((?!api|_next/static|_next/image|favicon.ico).*)'],
};
```

You can swap in `next-intl`. **The matcher must exclude every static route** — otherwise API/search routes get rewritten and 404.

## React Router

Declare `:lang` (or `:lang?` for optional) in `app/routes.ts`; pull locale via `useParams()` in `app/root.tsx`.

```ts
// app/routes.ts
route(':lang?', [
  route('docs/*', 'routes/docs.tsx'),
]);
```

## TanStack Start

`useParams({ strict: false })` inside `__root.tsx`; same `i18nUI.provider(lang)` pattern as Next.js.

## UI translations

```ts
// lib/i18n.ts
import { defineI18nUI } from 'fumadocs-ui/i18n';

export const i18nUI = defineI18nUI(i18n, {
  en: { displayName: 'English', search: 'Search' },
  cn: { displayName: '中文', search: '搜尋文檔' },
});
```

Mount via `<RootProvider i18n={i18nUI.provider(lang)}>`. Localised layout props via `baseOptions(locale)`.

## Search i18n

Built-in Orama supports `locale` in `useDocsSearch` and a per-locale index. Cloud providers usually expose multilingual indexes natively.

## Navigation in MDX

Fumadocs' own layouts (sidebar/TOC) handle locale internally. For your own links in MDX, prepend `${lang}` manually or use `<DynamicLink href="/[lang]/...">` from `fumadocs-core/dynamic-link`.

## Static export

| Adapter | Static-mode wiring |
| --- | --- |
| Orama | `type: 'static'` on both server (`exportSearchIndexes`) and UI (`useDocsSearch({ type: 'static' })`) |
| FlexSearch | `flexsearchStaticClient` ships the index as a client asset |
| Algolia / Orama-Cloud / Mixedbread | Hosted — works statically without extra config |

**Wiring only the server or only the UI yields silent runtime errors.** Configure both ends or accept a hosted provider.

## Common pitfalls (this layer)

- **Search adapter mismatch.** `useDocsSearch({ type: 'fetch' })` only matches the Orama API route; switching to Algolia/Trieve/Mixedbread requires changing both the server route and the client `type`.
- **Provider boundary.** `useDocsSearch` lives in a client component. If `RootProvider` is rendered from a server tree, wrap it in a `'use client'` provider so dialog functions can cross.
- **Locale slug collisions.** When `[lang]/` and a literal route share a top-level segment (e.g. `app/api`), the i18n middleware matcher must exclude `api`, `_next/static`, `_next/image`, and any custom static folders.
- **`hideLocale: 'always'` cookie.** Bad for fully static hosting and search indexing; prefer `'default-locale'` unless you control the cache layer.
- **Static search drift.** Static modes index at build; CMS / OpenAPI / Obsidian / Python pipelines that change *between* builds will silently miss content.

See `references/07-pitfalls.md` for the consolidated catalogue.
