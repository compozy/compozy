# Install & Setup

Source URLs (verified May 2026): `fumadocs.dev/docs/manual-installation`, `/docs/cli/create-fumadocs-app`, `/docs/deploying`, `/docs/deploying/static`.

## Adapter selection

Fumadocs runs inside an existing React framework. The adapter determines:
- Which `RootProvider` subpath to import.
- The catch-all route shape.
- Tailwind / CSS entry location.
- Whether `app/api/search/route.ts` (Next) or `route('api/search', ...)` (React Router) wires the default search endpoint.

| Adapter | Provider subpath | Catch-all docs route | Search route |
| --- | --- | --- | --- |
| Next.js | `fumadocs-ui/provider/next` | `app/<segment>/[[...slug]]/page.tsx` | `app/api/search/route.ts` |
| React Router | `fumadocs-ui/provider/react-router` | `route('docs/*', 'routes/docs.tsx')` | `route('api/search', 'routes/search.ts')` |
| TanStack Start | `fumadocs-ui/provider/tanstack` | `routes/docs/$.tsx` | `routes/api/search.ts` |
| Waku | `fumadocs-ui/provider/waku` | `pages/docs/[...slugs].tsx` | `pages/_api/api/search.ts` |

Switching adapters is non-trivial: every framework-specific file (root layout, route definitions, `RootProvider` import) must change in lockstep.

## Scaffold (recommended)

```bash
npx create-fumadocs-app
```

Programmatic:

```ts
import { create } from 'create-fumadocs-app';

await create({
  outputDir: 'my-docs',
  template: '+next+fuma-docs-mdx', // adapter+content combo
  packageManager: 'pnpm',
});
```

Templates follow the pattern `+<adapter>+fuma-docs-<content-source>`:
- `+next+fuma-docs-mdx`, `+next+fuma-docs-local-md`, `+next+fuma-docs-mdx-remote`
- `+react-router+fuma-docs-mdx`, etc.

The scaffold produces:
- `app/layout.tsx` with `<RootProvider>` mounted
- `app/(home)/page.tsx` (optional landing)
- `app/docs/layout.tsx` + `app/docs/[[...slug]]/page.tsx`
- `app/api/search/route.ts`
- `lib/source.ts` (`loader()`)
- `lib/layout.shared.tsx` (`baseOptions()`)
- `source.config.ts` (`defineDocs` + `defineConfig`)
- `content/docs/index.mdx`
- `next.config.mjs` (wrapped with `createMDX()`)
- `mdx-components.tsx` (`useMDXComponents`)

## Manual install — Next.js

Prereqs: **Next.js 16+, Tailwind CSS 4** (Tailwind 3 is silently broken).

```bash
npm i fumadocs-mdx fumadocs-core @types/mdx
npm i fumadocs-ui  # adds the UI theme
```

1. **`source.config.ts`** at the repo root (see `assets/source-config-template.ts`):
   ```ts
   import { defineDocs, defineConfig } from 'fumadocs-mdx/config';
   export const docs = defineDocs({ dir: 'content/docs' });
   export default defineConfig();
   ```
2. **`next.config.mjs`** (ESM only — `next.config.js` will fail):
   ```js
   import { createMDX } from 'fumadocs-mdx/next';
   const withMDX = createMDX();
   /** @type {import('next').NextConfig} */
   const config = {};
   export default withMDX(config);
   ```
3. **`tsconfig.json`** alias:
   ```json
   { "compilerOptions": { "paths": { "collections/*": ["./.source/*"] } } }
   ```
4. **`lib/source.ts`** (see `assets/source-template.ts`):
   ```ts
   import { loader } from 'fumadocs-core/source';
   import { docs } from 'collections/server';
   export const source = loader({ baseUrl: '/docs', source: docs.toFumadocsSource() });
   ```
5. **`global.css`** (Tailwind 4 entry):
   ```css
   @import 'tailwindcss';
   @import 'fumadocs-ui/css/neutral.css'; /* or black, vitepress, dusk, catppuccin, ocean, purple, solar, emerald, ruby, aspen, shadcn */
   @import 'fumadocs-ui/css/preset.css';
   ```
6. **`app/layout.tsx`**:
   ```tsx
   import { RootProvider } from 'fumadocs-ui/provider/next';
   export default function RootLayout({ children }: { children: React.ReactNode }) {
     return (
       <html lang="en" suppressHydrationWarning>
         <body className="flex flex-col min-h-screen">
           <RootProvider>{children}</RootProvider>
         </body>
       </html>
     );
   }
   ```
7. **`lib/layout.shared.tsx`** — exports `baseOptions()` returning `BaseLayoutProps` (`fumadocs-ui/layouts/shared`).
8. **`app/docs/layout.tsx`** + **`app/docs/[[...slug]]/page.tsx`** — see `references/04-ui-layouts-and-components.md` for the full shapes.
9. **`app/api/search/route.ts`**:
   ```ts
   import { source } from '@/lib/source';
   import { createFromSource } from 'fumadocs-core/search/server';
   export const { GET } = createFromSource(source);
   ```
10. **`mdx-components.tsx`** (see `assets/mdx-components-template.tsx`).

## Manual install — React Router

Prereqs: Tailwind 4, Fumadocs MDX configured via the **Vite** setup guide (creates `lib/source.ts`).

```bash
npm i fumadocs-core fumadocs-ui
```

`app/app.css`:
```css
@import 'tailwindcss';
@import 'fumadocs-ui/css/neutral.css';
@import 'fumadocs-ui/css/preset.css';
```

`app/routes.ts`:
```ts
import { type RouteConfig, index, route } from '@react-router/dev/routes';
export default [
  index('routes/home.tsx'),
  route('docs/*', 'routes/docs.tsx'),
  route('api/search', 'routes/search.ts'),
] satisfies RouteConfig;
```

`app/root.tsx` wraps `<RootProvider>` from `fumadocs-ui/provider/react-router` inside `<html>`/`<body>` together with React Router's `<Meta>`, `<Links>`, `<ScrollRestoration>`, `<Scripts>`.

## Manual install — TanStack Start

Prereqs: Tailwind 4, Fumadocs MDX (Vite setup).

```bash
npm i fumadocs-core fumadocs-ui
```

`styles/app.css` uses the same three imports. Files: `components/mdx.tsx`, `lib/layout.shared.tsx`, `routes/docs/$.tsx`, `routes/api/search.ts`.

`__root.tsx`:
```tsx
import { createRootRoute } from '@tanstack/react-router';
import { RootProvider } from 'fumadocs-ui/provider/tanstack';
import { HeadContent, Scripts } from '@tanstack/react-start';

export const Route = createRootRoute({
  component: () => (
    <html><body>
      <RootProvider>{/* <Outlet /> */}</RootProvider>
      <Scripts />
    </body></html>
  ),
  head: () => ({ /* meta, links */ }),
});
```

## Manual install — Waku

Prereqs: Tailwind 4, Fumadocs MDX (Vite setup).

```bash
npm i fumadocs-core fumadocs-ui
```

`src/styles/globals.css` — same three imports.
Files: `components/mdx.tsx`, `lib/layout.shared.tsx`, `pages/docs/_layout.tsx`, `pages/docs/[...slugs].tsx`, `pages/_api/api/search.ts`.

`components/provider.tsx` (must be `'use client'`):
```tsx
'use client';
import { RootProvider } from 'fumadocs-ui/provider/waku';
export default function Provider({ children }: { children: React.ReactNode }) {
  return <RootProvider>{children}</RootProvider>;
}
```

Mount it from `pages/_layout.tsx`.

## Deploying

| Target | Notes |
| --- | --- |
| Next.js + Vercel | Default Node runtime. Edge runtime is **unsupported**. |
| Next.js + Cloudflare | Use OpenNext (`https://opennext.js.org/cloudflare`). Raw Cloudflare Pages "Edge" is not supported. |
| Next.js + Docker | `source.config.ts` + `next.config.*` MUST be copied into the Docker `WORKDIR` during the deps install step, otherwise `fumadocs-mdx` cannot resolve config at build. |
| Next.js static export | `output: 'export'` in `next.config.mjs`. Pair with static-mode search (Orama static / FlexSearch static / hosted). |
| React Router SPA | `react-router.config.ts` `ssr: false` + `prerender({ getStaticPaths })`. **All loaders must be pre-rendered.** Use `createGetUrl('/docs')` + `getSlugs` from `fumadocs-core/source` and walk `glob('**/*.mdx', { cwd: 'content/docs' })`. |
| TanStack Start SPA | `tanstackStart({ spa: { enabled: true, prerender: { enabled: true }, pages: [...] } })` in `vite.config.ts`. Auto-crawls visible pages; list hidden ones explicitly. |
| Waku static | Every page must use `static` render mode. |

## CLI surfaces

Three distinct binaries — do not confuse them:

- **`create-fumadocs-app`** — scaffold projects (interactive or programmatic).
- **`@fumadocs/cli`** — Shadcn-style component fork tool:
  - `npx @fumadocs/cli` — initialise CLI config.
  - `npx @fumadocs/cli add <name...>` — vendor component source (e.g. `add banner files graph-view`).
  - `npx @fumadocs/cli customize` — fork layouts for full customisation.
  - `npx @fumadocs/cli tree <dir> <out>` — generate `Files`/`Folder`/`File` JSX/MDX from a directory.
  - Run with `@latest` so import rewrites match the current package layout.
- **`fumapress`** — open Markdown files in Fumadocs UI without scaffolding (`npx fumapress <dir>`).
