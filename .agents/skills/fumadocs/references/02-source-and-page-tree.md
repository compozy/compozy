# Source API & Page Tree

Source URLs (verified May 2026): `fumadocs.dev/docs/page-conventions`, `/docs/headless/source-api`, `/docs/headless/page-tree`, `/docs/headless/source-api/source`, `/docs/headless/source-api/plugins`, `/docs/navigation`.

## `loader()` — the spine

```ts
import { loader } from 'fumadocs-core/source';
import { docs } from 'collections/server';
import { lucideIconsPlugin } from 'fumadocs-core/source/lucide-icons';

export const source = loader({
  source: docs.toFumadocsSource(),
  baseUrl: '/docs',
  // Optional: override URL per-page (i18n, custom routing)
  url: (slugs, locale) => locale ? `/${locale}/docs/${slugs.join('/')}` : `/docs/${slugs.join('/')}`,
  // Optional: custom slug rule (return undefined → fallback to default)
  slugs: (file) => undefined,
  // Optional: turn `icon` frontmatter into a ReactElement
  icon: (name) => undefined,
  // Optional: i18n config — generates per-locale trees
  // i18n,
  plugins: [lucideIconsPlugin()],
});
```

`source` instance methods:

| Method | Returns | Use for |
| --- | --- | --- |
| `getPage(slugs, locale?)` | single page | render a doc page |
| `getPages(locale?)` | `Page[]` | search index, sitemap, RSS |
| `getPageTree(locale?)` | `Root` tree | `<DocsLayout tree={...}>` |
| `getNodePage(node)` / `getNodeMeta(node)` | original file | recover from a tree node |
| `generateParams()` | `{ slug: string[]; lang?: string }[]` | Next.js `generateStaticParams` |
| `getLanguages()` | `Record<lang, Page[]>` | locale-aware sitemap |
| `serializePageTree(tree)` | JSON | non-RSC clients (drops JSX) |

## Source argument shapes

| Shape | Use |
| --- | --- |
| Single source | `docs.toFumadocsSource()` |
| Multi source | `{ docs: ..., openapi: ... }` — result objects carry a `type` discriminant |
| `StaticSource<T>` | object literal `{ files: [{ type: 'page' \| 'meta', path, data }] }` — paths must be **virtual** (`folder/file.mdx`); absolute or `./`-prefixed paths are rejected |
| `DynamicSource<T>` | `{ async files() {...}, configure(loader) {...} }` consumed by `dynamicLoader()` from `fumadocs-core/source/dynamic`. Cached — call `source.revalidate()` (or `source.revalidate('docs')` for multi-source) to invalidate |

## Plugins

Built-in: `lucideIconsPlugin()` (requires `lucide-react`). Custom plugin shape:

```ts
loader({
  plugins: ({ typedPlugin }) => [
    typedPlugin({
      transformStorage({ storage }) { /* virtual-FS hook before processing */ },
      transformPageTree: {
        file(node, file) {
          node.name = <>Custom JSX</>;
          return node;
        },
      },
    }),
  ],
});
```

`typedPlugin` gives accurate types for one loader; standalone `LoaderPlugin` is for cross-loader reusable plugins.

## Frontmatter (per MDX file)

| Field | Purpose |
| --- | --- |
| `title` | Page title (sidebar + `<h1>`) |
| `description` | Sidebar tooltip + meta tag |
| `icon` | Icon name; resolved by the `icon` handler passed to `loader()` (Fumadocs ships **no** icon library) |

Customise the schema via Fumadocs MDX `schema` option per collection (extend `pageSchema` from `fumadocs-core/source/schema`).

## Slug rules

| File | Slugs |
| --- | --- |
| `./dir/page.mdx` | `['dir', 'page']` |
| `./dir/index.mdx` | `['dir']` |
| `./(group)/page.mdx` | `['page']` (parens = folder group, no slug impact) |

## `meta.json` (per folder)

```json
{
  "title": "Display Name",
  "icon": "MyIcon",
  "description": "...",
  "defaultOpen": true,
  "collapsible": true,
  "root": false,
  "pages": ["index", "getting-started", "...", "[Vercel](https://vercel.com)"]
}
```

- `collapsible` defaults to `true`.
- Items default to alphabetical ordering; `pages` overrides ordering AND filters (only listed items appear).
- `root: true` marks a **root folder** — only items inside the open root are visible. Fumadocs UI renders root folders as Layout Tabs.

## `pages` directives

| Type | Syntax | Meaning |
| --- | --- | --- |
| Path | `./path/to/page` | path to a page or folder |
| Separator | `---Label---` or `---[Icon]Label---` | section divider (icon optional) |
| Link | `[Text](url)` / `[Icon][Text](url)` / `external:[Text](url)` | static sidebar link |
| Rest | `...` | inject remaining items, alphabetical |
| Reversed Rest | `z...a` | inject remaining items reversed |
| Extract | `...folder` | flatten items from a sub-folder |
| Except | `!item` | exclude an item from `...` / `z...a` |

## Page tree types (`fumadocs-core/page-tree`)

| Type | Shape |
| --- | --- |
| `Root` | `{ name, description?, children, fallback?, $id? }` |
| `Page` | `{ type: 'page', name, url, external?, description?, icon?, $id? }` |
| `Folder` | `{ type: 'folder', name, root?, defaultOpen?, collapsible?, index?, icon?, children, $id? }` (where `index` is an embedded `Page`) |
| `Separator` | `{ type: 'separator', name?, icon?, $id? }` |

Trees are sent to clients (sidebar, breadcrumb), so they must stay small and serializable — **no functions, no large data, no secrets**. UI components (`DocsLayout` etc. in `fumadocs-ui`) consume the tree; **the tree itself is computed in core** and is intentionally agnostic to the UI layer.

## Hard rules

- **No duplicated URLs anywhere in the tree.** Fumadocs locates the active node by `pathname` only.
- **`root` ≠ folder group.** `(parens)` removes the folder from the slug; `root: true` toggles "isolated tab" mode.
- **`pages` filters siblings.** Once you list any item, only listed items render — don't forget `...` for "everything else".
- **Slug collisions are fatal.** Wrap re-organised folders with `(group)` or override slugs in `loader({ slugs })`.

## i18n routing (`fumadocs-core/i18n`)

Two parsers:
- **`dot`** (default): `get-started.cn.mdx`, `meta.cn.json` (default locale omits the suffix).
- **`dir`**: `content/docs/{locale}/...` directory per locale.

See `references/05-search-and-i18n.md` for the full middleware/redirect setup.

## Page-tree helpers (`fumadocs-core/page-tree`)

- `findNeighbour(tree, pathname)` — previous/next links.
- `findSiblings(tree, pathname)` — sibling list.
- `findParent(tree, pathname)` — ancestor lookup.
- `findPath(tree, pathname)` — breadcrumb path.
- `getPageTreeRoots(tree)` — root folders (for Layout Tabs).

## Versioning

| Strategy | Use when |
| --- | --- |
| **Partial** (folders) | minor versions; render as Layout Tabs (`root: true`) |
| **Full** (Git branch + subdomain `v2.example.com`) | major versions where dependencies upgrade independently |

## Headless components (no UI required)

`fumadocs-core/breadcrumb`, `fumadocs-core/link`, `fumadocs-core/dynamic-link`, `fumadocs-core/toc`. Each is unstyled — wrap with the framework's `<Link>` and your own JSX. See the headless docs for `<AnchorProvider>` / `<ScrollProvider>` / `<TOCItem>` shapes when building a custom TOC.
