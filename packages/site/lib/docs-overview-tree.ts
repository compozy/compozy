import type { Folder, Item, Node, Root } from "fumadocs-core/page-tree";
import type { ContentStorage, PageTreeTransformer } from "fumadocs-core/source";

/**
 * Single owner of how a docs category presents its own landing page.
 *
 * Fumadocs decides that per folder: when `meta.json` lists `"index"` the landing page becomes a
 * normal child row, otherwise it is attached as `folder.index` and the category header turns into a
 * link. Leaving the choice to each `meta.json` produced a sidebar where some categories navigated on
 * click and others only expanded, and the generated `cli-reference` tree could never follow an
 * authored convention at all.
 *
 * This transformer removes the choice: every category keeps a plain expand/collapse header and
 * exposes its landing page as the first child, labeled `Overview`.
 *
 * Opting out stays possible through the Fumadocs-native `"!index"` entry, which removes the landing
 * page from the tree entirely — there is nothing left to hoist.
 */
const OVERVIEW_LABEL = "Overview";
const INDEX_EXTENSIONS = [".mdx", ".md"];

function indexFilePaths(folderPath: string): Set<string> {
  const base = folderPath.length > 0 ? `${folderPath}/index` : "index";
  return new Set(INDEX_EXTENSIONS.map(extension => `${base}${extension}`));
}

function findIndexChild(children: Node[], folderPath: string): Item | undefined {
  const paths = indexFilePaths(folderPath);
  return children.find(
    (child): child is Item =>
      child.type === "page" && child.$ref !== undefined && paths.has(child.$ref)
  );
}

/**
 * Nodes are mutated in place on purpose: the builder memoizes them by file path and strips its
 * internal bookkeeping from those exact objects after the tree is assembled. Returning copies would
 * detach the tree from that cleanup pass.
 */
export function hoistOverviewPage(folder: Folder, folderPath: string): Folder {
  const overview = folder.index ?? findIndexChild(folder.children, folderPath);
  if (!overview) return folder;
  overview.name = OVERVIEW_LABEL;
  folder.children = [overview, ...folder.children.filter(child => child !== overview)];
  delete folder.index;
  return folder;
}

/**
 * The tree root is not a folder, so the folder hook never reaches it: its landing page is a plain
 * top-level item whose name comes straight from frontmatter. The sidebar row and the page heading
 * want different words there — the row is one entry among eight groups and reads `Overview`, while
 * the page itself is the whole-system landing and titles itself accordingly — so the row is renamed
 * here and the frontmatter `title` stays free to be the heading. Ordering is left alone: the root
 * `meta.json` already lists `index` first.
 */
export function renameRootOverview(root: Root): Root {
  const overview = findIndexChild(root.children, "");
  if (overview) {
    overview.name = OVERVIEW_LABEL;
  }
  return root;
}

/**
 * Built per call site rather than exported as a constant: `PageTreeTransformer<S>` is contravariant
 * in its storage type, so a constant pinned to one storage cannot be shared by loaders that each
 * infer a collection-specific storage from their source.
 */
export function overviewPageTransformer<S extends ContentStorage>(): PageTreeTransformer<S> {
  return {
    folder(node, folderPath) {
      return hoistOverviewPage(node, folderPath);
    },
    root(node) {
      return renameRootOverview(node);
    },
  };
}
