# UI: Layouts, Components, MDX wiring

Source URLs (verified May 2026): `fumadocs.dev/docs/ui`, `/docs/ui/{theme,translations,search,layouts,components}`, `/docs/ui/layouts/{docs,notebook,flux,home-layout,page,nav,links,root-provider}`, `/docs/ui/components/*`.

`fumadocs-ui` is the opinionated, batteries-included theme. Built on Radix UI by default; can swap to Base UI via `@fumadocs/base-ui`.

## Theming and tokens

**Tailwind v4 only.** Three `@import` lines in your global stylesheet:

```css
@import 'tailwindcss';
@import 'fumadocs-ui/css/neutral.css';   /* or black, vitepress, dusk, catppuccin, ocean, purple, solar, emerald, ruby, aspen */
@import 'fumadocs-ui/css/preset.css';
```

For shadcn-themed apps, replace the colour file with `fumadocs-ui/css/shadcn.css` — Fumadocs UI inherits shadcn tokens. The preset rewrites default border / text / background colours (preflight changes — expect global resets).

### CSS variables

Override under `@theme` and `.dark`:

```css
@theme {
  --color-fd-background: ...;
  --color-fd-foreground: ...;
  --color-fd-muted: ...;
  --color-fd-popover: ...;
  --color-fd-card: ...;
  --color-fd-border: ...;
  --color-fd-primary: ...;
  --color-fd-secondary: ...;
  --color-fd-accent: ...;
  --color-fd-ring: ...;
}
```

All surfaces use `fd-*` prefixed tokens — never invent unprefixed ones.

### Layout sizing

| Variable | Default |
| --- | --- |
| `--fd-layout-width` | ~97rem |
| `--fd-sidebar-width` | computed |
| `--fd-toc-width` | computed |
| `--fd-nav-height` | computed |
| `--fd-banner-height` | computed |
| `--fd-toc-popover-height` | computed |

The Docs/Notebook grid (`#nd-docs-layout`) uses these to position sidebar/header/TOC/main. **Override `--fd-nav-height` with `!important`** when supplying a custom navbar.

### Typography

Built-in `prose` plugin (forked from `@tailwindcss/typography`) is enabled via the preset. If you must use `@tailwindcss/typography` instead, set `className: wysiwyg` on the plugin to avoid clashes.

### Light / dark + RTL

Theme switching via `next-themes`, bundled inside `RootProvider`. RTL: set `dir="rtl"` on **both** `<body>` and `<RootProvider>` — half-applied RTL leaves the sidebar in LTR.

## `RootProvider`

Must wrap the app at the root layout. Per-framework subpath:

| Framework | Import |
| --- | --- |
| Next.js | `fumadocs-ui/provider/next` |
| React Router | `fumadocs-ui/provider/react-router` |
| TanStack Start | `fumadocs-ui/provider/tanstack` |
| Waku | `fumadocs-ui/provider/waku` |

Props:

| Prop | Shape |
| --- | --- |
| `search` | `{ enabled, SearchDialog, hotKey, options, preload, links }` |
| `theme` | `{ enabled }` — disables `next-themes` if `false` |
| `i18n` | `{ translations, locale, ... }` |
| `dir` | `'ltr' \| 'rtl'` |

**Provider mismatch is silent.** Wrong subpath ⇒ search dialog, theme switcher, i18n, hot keys all silently no-op.

## Layouts (when to use which)

All layouts share `BaseLayoutProps` (`nav`, `links`, `githubUrl`, `slots`, `themeSwitch`, `searchToggle`, `i18n`, `children`). Convention: hoist into `lib/layout.shared.tsx` (`baseOptions()`) and spread per-layout.

| Layout | Module | Use case | Mental model |
| --- | --- | --- | --- |
| **Docs** | `fumadocs-ui/layouts/docs` | Classic sidebar docs site | "I have a knowledge base, give me a sidebar." |
| **Notebook** | `fumadocs-ui/layouts/notebook` | Denser, app-like docs (top-tab nav possible) | "Tighter, product-style shell — like Linear or Vercel docs." Sidebar/navbar **cannot** be replaced. |
| **Home** | `fumadocs-ui/layouts/home` | Marketing / landing pages that share docs chrome | "Marketing surface that shares the docs chrome." |
| **Flux** | `fumadocs-ui/layouts/flux` | Aggressively minimal, experimental | **Client-only** (no unserialisable RSC props), no `tocPopover`, optional `renderNavigationPanel`. |
| **Page** (per-doc) | `fumadocs-ui/layouts/<layout>/page` | Article wrapper inside a layout | TOC + breadcrumb + footer + body for one page. |

### `DocsLayout` props (most common)

```tsx
<DocsLayout
  tree={source.getPageTree()}
  sidebar={{ enabled, prefetch, banner, components: { Separator, Item, Folder } }}
  tabs={[{ title, url, urls?, description? }] /* or false to disable, or { transform } */}
  tabMode="navbar" /* notebook only: 'sidebar' | 'navbar' */
  nav={{ enabled, title, url, transparentMode, component }}
  links={[/* see Links section */]}
  githubUrl="https://github.com/owner/repo"
  themeSwitch={{ enabled }}
  searchToggle={{ enabled }}
  slots={{ /* see Slots section */ }}
  i18n
  containerProps={{ className }}
>
  {children}
</DocsLayout>
```

### `DocsPage` props (per-doc shell)

```tsx
import { DocsPage, DocsTitle, DocsDescription, DocsBody } from 'fumadocs-ui/layouts/<layout>/page';

<DocsPage
  toc={page.data.toc}
  full={page.data.full /* true → fills width, TOC becomes popover */}
  tableOfContent={{ /* TOC config */ }}
  tableOfContentPopover={{ container, header, footer, style: 'normal' /* | 'clerk' */, list }}
  breadcrumb={{ enabled }}
  footer={{ enabled }}
  slots={{ toc: { provider, main, popover }, footer, breadcrumb, container }}
>
  <DocsTitle>{page.data.title}</DocsTitle>
  <DocsDescription>{page.data.description}</DocsDescription>
  <DocsBody>{/* MDX */}</DocsBody>
</DocsPage>
```

**Page imports must match the layout.** Switching to Notebook means `import { DocsPage } from 'fumadocs-ui/layouts/notebook/page'` (or `flux/page`). Mismatching the import is the most common breakage.

### `links` prop (shared across layouts)

| Item type | Shape |
| --- | --- |
| Link | `{ icon, text, url, secondary?, active: 'url' \| 'nested-url' \| 'none' }` |
| Icon | `{ type: 'icon', label, icon, url }` |
| Custom | `{ type: 'custom', children, secondary?, on?: 'nav' \| 'menu' \| 'all' }` |
| Menu | `{ type: 'menu', text, items: [...] }` |
| Shortcut | `githubUrl: 'https://github.com/...'` (drops a GitHub icon link automatically) |

### Sidebar tabs (root folders)

Mark a folder with `meta.json { "root": true }` or pass `tabs={[{ title, url, urls?, description? }]}`. Disable with `tabs={false}`. Customise icons via `tabs={{ transform: (option, node) => ... }}`.

### Sidebar slots / banner / prefetch

- `sidebar.components.{ Separator, Item, Folder, ... }` — swap individual sidebar pieces.
- `sidebar.banner` — adds a banner at the top.
- `sidebar.prefetch: false` — turn off when on Vercel and serverless quotas are tight.

### Page slots

`slots={{ toc: { provider, main, popover }, footer, breadcrumb, container }}`. Generate scaffolds with `npx @fumadocs/cli add slots/docs/page/<toc|footer|breadcrumb|container>`.

## Search UI

Default search dialog is mounted by `RootProvider`. Configure or disable from the `search` prop:

```tsx
<RootProvider search={{
  enabled: true,
  SearchDialog,
  hotKey: [{ display: 'K', key: 'k' }],
  preload: true,
  options: { /* fetch options */ },
  links: [/* preset filter chips */],
}}>
```

Default hotkeys: `⌘K` / `Ctrl+K`.

### Custom dialog

Pass a custom component as `SearchDialog`. Build it with the primitive set from `fumadocs-ui/components/dialog/search`:

`SearchDialog`, `SearchDialogOverlay`, `SearchDialogContent`, `SearchDialogHeader`, `SearchDialogIcon`, `SearchDialogInput`, `SearchDialogClose`, `SearchDialogList`, `SearchDialogFooter`, `SearchDialogListItem`.

Drive results with `useDocsSearch({ type: 'fetch' | 'static' | 'orama' | ..., locale })` from `fumadocs-core/search/client`. See `references/05-search-and-i18n.md` for adapter wiring.

### Locale-aware search

Read `useI18n()` from `fumadocs-ui/contexts/i18n` and pass `locale` into `useDocsSearch`.

### Tag / category filters

Provide `links` (preset filter chips) on the `search` prop, or pass them through to `<SearchDialogList />`. Customise the markdown rendering of result snippets via `<SearchDialogListItem renderMarkdown={(text) => ...} />` (highlights are emitted as `<mark />`).

## Translations / i18n hooks

- Define a `Partial<Translations>` map from `fumadocs-ui/i18n` (e.g. `{ search: '搜尋文檔' }`) and pass it into `RootProvider` via `i18n={{ translations }}`.
- For multi-locale, pair with `fumadocs-core/i18n` config + middleware (see `references/05-search-and-i18n.md`).
- `useI18n()` (`fumadocs-ui/contexts/i18n`) exposes the current `locale` for client components.
- Layout-aware navigation: shared layouts forward locale into the page tree by reading the locale prefix on `loader()`.
- **Locale toggle layout shift.** Translations swap labels (search placeholder, tab labels) — use `useI18n()` instead of hard-coded strings, otherwise the navbar reflows on locale change.

## MDX wiring

Default component map at `fumadocs-ui/mdx` (Cards, Callouts, Code Blocks, Headings, plus `a`, `pre`, `img`, table elements). Import + spread:

```tsx
import defaultComponents from 'fumadocs-ui/mdx';
import { CodeBlock, Pre } from 'fumadocs-ui/components/codeblock';
import * as TabsComponents from 'fumadocs-ui/components/tabs';
import { ImageZoom } from 'fumadocs-ui/components/image-zoom';
import { createRelativeLink } from 'fumadocs-ui/mdx';
import type { MDXComponents } from 'mdx/types';

export function getMDXComponents(components?: MDXComponents): MDXComponents {
  return {
    ...defaultComponents,
    ...TabsComponents,
    pre: ({ ref: _ref, ...props }) => (
      <CodeBlock {...props}><Pre>{props.children}</Pre></CodeBlock>
    ),
    img: (props) => <ImageZoom {...(props as any)} />,
    ...components,
  } satisfies MDXComponents;
}
```

### Strip the `pre` `ref` prop

React 19 forwards `ref` as a normal prop on `pre`; spreading it into `CodeBlock` breaks `forwardRef`. Always strip it: `({ ref: _ref, ...props })`.

### Override headings, tables, links

Replace the keys (`h1`, `h2`, `table`, `a`, etc.) in the returned record.

### Relative MDX links

Use `createRelativeLink(source, page)` from `fumadocs-ui/mdx` as the `a` mapping in `page.tsx` (Server Components only) so `[link](./other.mdx)` resolves through `loader()`:

```tsx
<MdxContent components={getMDXComponents({ a: createRelativeLink(source, page) })} />
```

## Component cheat sheet

Source root: `fumadocs.dev/docs/ui/components`. Every component also has `npx @fumadocs/cli add <name>` to vendor it locally for editing.

| Component | Module | What / when | Key gotcha |
| --- | --- | --- | --- |
| `Accordion` / `Accordions` | `fumadocs-ui/components/accordion` | FAQ-style collapsibles in MDX | Hash linking: matching `id` opens the accordion when the URL hash matches. Swap to Base UI primitives via `@fumadocs/base-ui`. |
| `AutoTypeTable` | `fumadocs-typescript/ui` | Auto-generate prop tables from TS types | **Server Component only.** Cache via `createFileSystemGeneratorCache('.next/fumadocs-typescript')` for serverless. |
| `Banner` | `fumadocs-ui/components/banner` | Site-wide announcement | `id` enables dismiss + persistence. Default `changeLayout` injects `<style>` to shrink sidebar height — disable when stacking custom layouts. |
| `CodeBlock` / `Pre` | `fumadocs-ui/components/codeblock` | MDX-rendered Shiki code blocks | Map onto `pre` in `getMDXComponents` and **strip the conflicting `ref`** before spreading. |
| `DynamicCodeBlock` | `fumadocs-ui/components/dynamic-codeblock` | Runtime/lazy Shiki highlighting outside MDX | Client variant uses React 19 Suspense; languages/themes stream lazily. |
| `ServerCodeBlock` | `fumadocs-ui/components/codeblock.rsc` | Pre-rendered server variant | Use when you can pre-render. |
| `Files` / `Folder` / `File` | `fumadocs-ui/components/files` | Render directory trees | No real interactivity beyond expand/collapse. |
| `GithubInfo` | `fumadocs-ui/components/github-info` | Stars/forks badge | Without `token` you'll hit GitHub anonymous rate limits. Wrap as a `links: [{ type: 'custom', children: <GithubInfo .../> }]`. |
| `GraphView` | vendored via `npx @fumadocs/cli add graph-view` | Obsidian-style page graph | Requires `defineDocs({ docs: { postprocess: { extractLinkReferences: true } } })`. |
| `ImageZoom` | `fumadocs-ui/components/image-zoom` | Click-to-zoom for `img` | Override the MDX `img` mapping; on Next.js a default `sizes` is injected for `<Image />` if missing. |
| `InlineTOC` | `fumadocs-ui/components/inline-toc` | Inline collapsible TOC | Needs `page.data.toc`. Distinct from the layout TOC rail. |
| `Steps` / `Step` | `fumadocs-ui/components/steps` | Numbered tutorial sections | Without imports, use Tailwind utilities `fd-steps` / `fd-step`; `[&_h3]:fd-step` auto-styles headings. |
| `Tabs` / `Tab` | `fumadocs-ui/components/tabs` | Code/content tabs | Without `value`, identity falls back to children index — re-renders may misalign tabs. `groupId` shares state across tabs site-wide; pair with `persist` for `localStorage`. `id` + URL hash deep-links to a tab; `updateAnchor` syncs hash on switch. |
| `TypeTable` | `fumadocs-ui/components/type-table` | Hand-written prop tables | Manual counterpart to `AutoTypeTable`. |

## CLI customise

Once you `npx @fumadocs/cli add layouts/docs` or `npx @fumadocs/cli customize`, **import the layout type from your local copy, not `fumadocs-ui`** (e.g. `@/components/layout/docs`). Trade-off: no automatic UI updates from the upstream package.

## Common pitfalls (this layer)

See `references/07-pitfalls.md` for the consolidated catalogue.
