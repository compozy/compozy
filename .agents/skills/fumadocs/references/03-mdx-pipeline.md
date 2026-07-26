# MDX Pipeline (`fumadocs-mdx`)

Source URLs (verified May 2026): `fumadocs.dev/docs/mdx`, `/docs/mdx/collections`, `/docs/mdx/global`, `/docs/mdx/next`, `/docs/mdx/vite`, `/docs/mdx/loader/{bun,node}`, `/docs/mdx/entry/{server,browser,dynamic,import}`, `/docs/mdx/{typegen,include,last-modified,async,performance,workspace}`, `/docs/markdown/{math,mermaid,twoslash}`.

`fumadocs-mdx` is the **content processing layer** — not a CMS. It compiles Markdown / MDX + JSON / YAML into typed data accessible from React frameworks. Bundler-driven by default; runtime loaders exist for unbundled Node / Bun scripts.

## `source.config.ts` shape

The single config file at the repo root composes three primitives:

| Helper | Purpose |
| --- | --- |
| `defineConfig` | Global options for the entire MDX pipeline (`mdxOptions`, plugins, `workspaces`, `experimentalBuildCache`). **Default-export** it. |
| `defineCollections` | Single typed bucket of files (`type: 'doc' \| 'meta'`) with its own `dir`, `schema`, `mdxOptions`, `postprocess`, `async`, `dynamic`. |
| `defineDocs` | Convenience wrapper that emits a paired `doc` (Markdown / MDX) + `meta` (JSON / YAML) collection — the shape Fumadocs expects for tree-shaped docs. Use for the docs surface; use `defineCollections` for flat content (blog, changelog, marketing). |

Canonical snippet:

```ts
// source.config.ts
import { defineDocs, defineCollections, defineConfig } from 'fumadocs-mdx/config';
import { z } from 'zod';
import { pageSchema, metaSchema } from 'fumadocs-core/source/schema';

export const docs = defineDocs({
  dir: 'content/docs',
  docs: { schema: pageSchema.extend({ index: z.boolean().default(false) }) },
  meta: { schema: metaSchema },
});

export const blog = defineCollections({
  type: 'doc',
  dir: 'content/blog',
  schema: z.object({ title: z.string(), date: z.string() }),
});

export default defineConfig({
  // shared mdxOptions, plugins, workspaces…
});
```

## Frontmatter / Meta schemas

- `schema` validates frontmatter (`type: 'doc'`) or file body (`type: 'meta'`).
- **Validation runs at build time → output must be serialisable.**
- Standard Schema compatible (Zod, Valibot, …). Function form receives the transform context (`ctx.path`, etc.).
- For Fumadocs trees: extend `pageSchema` / `metaSchema` from `fumadocs-core/source/schema` so core fields stay typed.

## Collection options (`defineCollections`)

| Option | Type | Notes |
| --- | --- | --- |
| `dir` | `string \| string[]` | Source directory(s) |
| `files` | glob | Override the default `**/*` glob |
| `schema` | StandardSchema | Frontmatter (doc) or body (meta) validation |
| `mdxOptions` | object | **Wipes globals** — wrap with `applyMdxPreset(...)` to preserve docs preset |
| `postprocess` | object | doc-only; lifts build-time data into runtime exports |
| `async` | boolean | async-imported outputs (Webpack big win, Turbopack server-only) |
| `dynamic` | boolean | runtime compile via on-demand pipeline |

`postprocess` doc-only options:
- `includeProcessedMarkdown: true` → `await page.data.getText('processed')`.
- `valueToExport: ['dataName']` → re-exports a `vfile.data.*` value as a JS export.

## `.source/` output layout

Generated entries land in `.source/` and are imported via the recommended `tsconfig` alias `collections/* → .source/*`:

```
.source/
  server.ts        // server entry — eager
  browser.ts       // browser entry — async imports for client routing
  dynamic.ts       // dynamic entry — on-demand compile
  index.d.ts       // generated types (typegen)
```

`.source/` is regenerated on `dev` / `build`. Prime it manually with `npx fumadocs-mdx [config-path] [output-dir]` (defaults `source.config.ts` + `.source/`). Hook into `package.json` `postinstall` so types/entries exist before the dev server boots.

## Loaders & adapters

| Stack | Adapter / Plugin | Notes |
| --- | --- | --- |
| Next.js | `import { createMDX } from 'fumadocs-mdx/next'` | Wrap `next.config.mjs`. **ESM only** — prefer `.mjs`; for `next.config.ts` enable Native Node TS resolver |
| Vite / Waku | `import mdx from 'fumadocs-mdx/vite'` | `plugins: [mdx(MdxConfig)]` |
| Bun (script) | preload `'fumadocs-mdx/bun'` via `bunfig.toml` | Optional `postInstall({ configPath })` to (re)materialise `.source/` |
| Node (script, ESM) | `register('fumadocs-mdx/node/loader', import.meta.url)` | For unbundled scripts that need to `await import('./lib/source')` |

## Entry modes

| Entry | Import | Use it for |
| --- | --- | --- |
| `server` | `from 'collections/server'` | RSC / SSR / SSG. Eager, fastest first paint. Default choice |
| `browser` | `from 'collections/browser'` | Client routing (TanStack Start, React Router) — async-imported chunks. **Only `doc`/`docs` collections.** Returns a `createClientLoader` with `preload` + `useContent` |
| `dynamic` | `from 'collections/dynamic'` | On-demand compile at runtime. Pair with `dynamic: true` on the collection. Trades JS payload + cold latency for tiny build size — best for very large libraries |
| direct import | `import Page from '@/content/page.mdx'` | One-off MDX as a React component or as a Next.js `page.mdx`. No `loader()` indirection |

Pick by stack:
- RSC → `server`.
- Static export with client navigation → `browser`.
- >500 docs / build memory pressure → `dynamic`.
- Single bespoke MDX page → direct import.

Browser entry sketch:

```tsx
import browserCollections from 'collections/browser';
const clientLoader = browserCollections.docs.createClientLoader({
  component: ({ frontmatter, default: MDX }) => (
    <><h1>{frontmatter.title}</h1><MDX /></>
  ),
});
// loader: await clientLoader.preload(page.path);
// render: clientLoader.useContent(path)
```

## Dynamic features

- **Typegen** — `npx fumadocs-mdx [config-path] [output-dir]`. Hook into `package.json` `postinstall`.
- **Include** — `<include>./another.mdx</include>` reuses content (relative to importing file). Non-MDX targets render as fenced codeblock; supports `lang`, `meta`, region markers (`./code.ts#a`) via `//#region`/`//#endregion`, and `<section id>` / heading extraction. `cwd` resolves from cwd instead of current file. **Markdown (`.md`) needs `remark-directive`** (and often `rehype-raw`).
- **Last modified** — `plugins: [lastModified()]` in `defineConfig`. Reads Git history (full clone required, **no shallow clones**). Exposes `page.data.lastModified: Date`.
- **Async / Dynamic mode**:
  - `async: true` → async-imported outputs; helps with bundler cold start (Webpack much more than Turbopack).
  - `dynamic: true` → runtime compile via on-demand pipeline. Constraints: **no `import` / `export` in MDX** (pass components via `components` prop) and **images must be URL strings, not relative paths** (`/public/...`).
  - Either mode forces frontmatter-only access until `await page.data.load()` to obtain `body`, `toc`, etc.
- **Performance** — bundler-bound. ~500 MDX files is the comfortable ceiling before memory blows up. Escape hatches: lazy loading (above), or a custom remote source.
- **Workspaces** — multiple independent configs under one parent build:
  ```ts
  defineConfig({
    workspaces: {
      design: { dir, config: await import('./design/source.config.ts') },
    },
  });
  ```
  Each workspace is independent (no inheritance, `cwd` is workspace-local); outputs at `.source/{workspace}/*`.

## Default markdown features (built-in remark/rehype preset)

The default preset is built for docs sites:

| Plugin | Purpose |
| --- | --- |
| `remark-image` | Auto-static-imports for images, Next.js `<Image />`-friendly |
| `remark-heading` | TOC extraction (`vfile.data.toc`) |
| `remark-structure` | Search index (`vfile.data.structuredData`) |
| `rehype-code` | Shiki syntax highlighting; supports `title=…`, `lineNumbers`, `lineNumbers=4`, transformers like `// [!code ++]`, tab groups via `tab="…"` |
| `rehype-toc` | Export TOC as JSX |

Built-in syntax: callouts, cards, NPM-multi-manager codeblocks (`remark-npm`), steps, custom heading anchors (`# Heading [#slug]`), `[!toc]` / `[toc]` heading hints, GFM, internal-link prefetching.

Override with array (append) or function (control order):

```ts
defineConfig({
  mdxOptions: {
    remarkPlugins: [myRemark],
    rehypePlugins: (v) => [rehypeKatex, ...v], // function form when order matters
    rehypeCodeOptions: { /* … */ },
    remarkImageOptions: { placeholder: 'blur' }, // Next.js only
    remarkHeadingOptions: { /* … */ },
    // preset: 'minimal' to drop all defaults and accept raw ProcessorOptions
  },
});
```

**`mdxOptions` set on a collection wipes globals.** Use `applyMdxPreset({ … })` to preserve the docs preset.

## Math (KaTeX)

```bash
npm i remark-math rehype-katex katex
```

```ts
mdxOptions: {
  remarkPlugins: [remarkMath],
  rehypePlugins: (v) => [rehypeKatex, ...v], // before the syntax highlighter
}
```

Import `katex/dist/katex.css` in the root layout.

## Mermaid

No built-in wrapper — provide a `<Mermaid />` MDX component. Two recipes:

- Official `mermaid` + `next-themes` (client-only, lazy `import('mermaid')`, theme-aware re-render).
- `beautiful-mermaid` (server SVG render, falls back to Fumadocs `CodeBlock` on parse error).

Optional: convert ` ```mermaid ` codeblocks into the JSX form via `remarkMdxMermaid` from `fumadocs-core/mdx-plugins`.

## Twoslash (TS-aware code blocks)

```bash
npm i fumadocs-twoslash twoslash
```

- Add `transformerTwoslash()` to `rehypeCodeOptions.transformers`.
- **Predeclare `langs: ['js','jsx','ts','tsx']`** (Shiki cannot lazy-load langs inside Twoslash popups).
- Import `fumadocs-twoslash/twoslash.css` (Tailwind v4).
- Spread `fumadocs-twoslash/ui` into `getMDXComponents`.
- For Next.js, externalize `['typescript','twoslash']` in `serverExternalPackages`.
- Optional `createFileSystemTypesCache()` for build-time type cache.

## Default-on plugins

`remarkHeading`, `remarkImage`, `remarkStructure`, `rehypeCode` are pre-wired in Fumadocs MDX. **Adding them again duplicates ids/imports.** Reach into `fumadocs-core/mdx-plugins` only when adding new plugins or replacing the preset.

## Headless MDX plugins (drop-in)

| Plugin | Path | When |
| --- | --- | --- |
| `rehypeCode` | `fumadocs-core/mdx-plugins/rehype-code` | Custom Shiki config |
| `remarkDirectiveAdmonition` | `fumadocs-core/mdx-plugins/remark-admonition` | Docusaurus migration only (`:::tip`); JSX `<Callout>` is preferred |
| `remarkImage` | `fumadocs-core/mdx-plugins/remark-image` | Custom image hosting / placeholders |
| `remarkLLMs` | `fumadocs-core/mdx-plugins/remark-llms` | Stringify AST for LLM ingestion / `llms-full.txt` |
| `remarkMdxFiles` | `fumadocs-core/mdx-plugins/remark-mdx-files` | ASCII trees → `<Files>` JSX |
| `remarkNpm` | `fumadocs-core/mdx-plugins/remark-npm` | Install snippets → `<CodeBlockTabs>` |
| `remarkSteps` | `fumadocs-core/mdx-plugins/remark-steps` | Tutorials with `# Heading [step]` |
| `remarkTypeScriptToJavaScript` | `fumadocs-docgen/remark-ts2js` | TS/TSX with `ts2js` meta → TS / JS tabs (externalize `oxc-transform`) |

## Plugin order is load-bearing

- `rehype-katex` **before** the syntax highlighter.
- Twoslash transformer **after** Shiki defaults.
- Use the function form `(v) => [...]` whenever order matters.

## Common pitfalls (this layer)

See `references/07-pitfalls.md` for the consolidated catalogue.
