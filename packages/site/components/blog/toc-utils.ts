export interface TocEntryNode {
  title: string;
  url: string;
  items?: TocEntryNode[];
}

export interface TocItem {
  title: string;
  url: string;
  depth: number;
}

export function flattenToc(entries: TocEntryNode[], depth = 2): TocItem[] {
  const flat: TocItem[] = [];
  for (const entry of entries) {
    flat.push({ title: entry.title, url: entry.url, depth });
    if (entry.items?.length) {
      flat.push(...flattenToc(entry.items, depth + 1));
    }
  }
  return flat;
}
