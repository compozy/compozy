/** Client-safe view model for the blog front page: entries, filters, months and topics. */

export interface BlogEntryCover {
  src: string;
  width: number;
  height: number;
  blurDataURL?: string;
}

/** One post as the front page and the cards see it; serializable, without the MDX body. */
export interface BlogEntry {
  slug: string;
  permalink: string;
  title: string;
  description: string;
  date: string;
  category: string;
  tags: string[];
  featured: boolean;
  minutes: number;
  /** Stable issue number: the oldest post is 01. */
  number: string;
  cover: BlogEntryCover;
}

export interface BlogFilters {
  category: string | null;
  topic: string | null;
  query: string;
}

export interface BlogMonth {
  label: string;
  entries: BlogEntry[];
}

export interface BlogTopic {
  tag: string;
  count: number;
}

export function monthOf(iso: string): string {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "long",
    year: "numeric",
    timeZone: "UTC",
  });
}

export function formatPostDate(iso: string): string {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    timeZone: "UTC",
  });
}

/** Consecutive entries that share a month, in the order given. */
export function groupByMonth(entries: readonly BlogEntry[]): BlogMonth[] {
  const months = new Map<string, BlogEntry[]>();
  for (const entry of entries) {
    const label = monthOf(entry.date);
    months.set(label, [...(months.get(label) ?? []), entry]);
  }
  return Array.from(months, ([label, list]) => ({ label, entries: list }));
}

/** The most used tags, most used first, ties alphabetical. */
export function topTopics(entries: readonly BlogEntry[], limit = 8): BlogTopic[] {
  const counts = new Map<string, number>();
  for (const entry of entries)
    for (const tag of entry.tags) counts.set(tag, (counts.get(tag) ?? 0) + 1);
  return Array.from(counts, ([tag, count]) => ({ tag, count }))
    .toSorted((a, b) => b.count - a.count || a.tag.localeCompare(b.tag))
    .slice(0, limit);
}

export function topicLabel(tag: string): string {
  return tag.replace(/-/g, " ");
}

export function isFiltering(filters: BlogFilters): boolean {
  return Boolean(filters.category || filters.topic || filters.query.trim());
}

export function filterEntries(entries: readonly BlogEntry[], filters: BlogFilters): BlogEntry[] {
  const needle = filters.query.trim().toLowerCase();
  return entries.filter(
    entry =>
      (!filters.category || entry.category === filters.category) &&
      (!filters.topic || entry.tags.includes(filters.topic)) &&
      (!needle ||
        `${entry.title} ${entry.description} ${entry.tags.join(" ")}`
          .toLowerCase()
          .includes(needle))
  );
}
