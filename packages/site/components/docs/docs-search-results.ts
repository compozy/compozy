/** How the docs palette groups what search returns: suggestions for an empty query, else hits by section. */
import type { SearchPageHit } from "@/components/docs/hooks/use-docs-search";
import type { DocsTab } from "@/lib/docs-shell";

export const LIMIT = 8;

export interface DocsSearchSuggestion {
  url: string;
  title: string;
  description: string;
}

export function headOf(url: string) {
  return url.replace(/^\/docs\/?/, "").split("/")[0] ?? "";
}

function pathOf(hit: SearchPageHit) {
  return hit.breadcrumbs.join(" / ");
}

export interface Row {
  url: string;
  title: string;
  detail: string;
  path: string;
}

export interface ResultGroup {
  key: string;
  heading: string;
  rows: Row[];
}

function rowOf(hit: SearchPageHit): Row {
  return { url: hit.url, title: hit.title, detail: hit.excerpt ?? pathOf(hit), path: pathOf(hit) };
}

/** The palette's groups, in display order: suggestions for an empty query, else hits by section. */
export function resultGroups(
  props: { tabs: readonly DocsTab[]; suggested: readonly DocsSearchSuggestion[] },
  query: string,
  hits: readonly SearchPageHit[],
  tag: DocsTab | null
): ResultGroup[] {
  if (!query.trim()) {
    const rows = props.suggested.map(page => ({
      url: page.url,
      title: page.title,
      detail: page.description,
      path: "",
    }));
    return [{ key: "suggested", heading: "Start here", rows }];
  }
  // Pages that sit before the first section (the docs home, "How to use these docs") get their own group.
  const sectioned = new Set(props.tabs.flatMap(tab => tab.heads));
  const intro = tag === null ? hits.filter(hit => !sectioned.has(headOf(hit.url))) : [];
  const groups: ResultGroup[] = [];
  if (intro.length > 0)
    groups.push({
      key: "intro",
      heading: `Docs · ${intro.length}`,
      rows: intro.slice(0, LIMIT).map(rowOf),
    });
  // With a section picked, only that section's hits show. The hits of the previous answer stay in
  // hand while the filtered one loads, and none from another section may be opened meanwhile.
  for (const tab of tag ? [tag] : props.tabs) {
    const matched = hits.filter(hit => tab.heads.includes(headOf(hit.url)));
    if (matched.length === 0) continue;
    groups.push({
      key: tab.id,
      heading: `${tab.label} · ${matched.length}`,
      rows: matched.slice(0, LIMIT).map(rowOf),
    });
  }
  return groups;
}
