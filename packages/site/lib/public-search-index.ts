import type { AdvancedIndex } from "fumadocs-core/search/server";
import { allPosts, allReleases, type Post, type Release } from "@/lib/blog";
import { protocolDocs, runtimeDocs } from "@/lib/source";

type SearchPage = {
  url: string;
  data: {
    title: string;
    description?: string;
    structuredData: AdvancedIndex["structuredData"];
  };
};

type TocEntry = {
  title: string;
  url: string;
  items?: TocEntry[];
};

type TocHeading = {
  id: string;
  title: string;
};

function byURL(left: { url: string }, right: { url: string }): number {
  return left.url.localeCompare(right.url);
}

function sortedByURL<T extends { url: string }>(items: T[]): T[] {
  return items.toSorted(byURL);
}

function joinContent(...parts: Array<string | undefined>): string {
  return parts
    .map(part => part?.trim())
    .filter((part): part is string => Boolean(part))
    .join("\n\n");
}

function slugFromHash(hashURL: string, fallback: string): string {
  const hash = hashURL.startsWith("#") ? hashURL.slice(1) : hashURL.split("#")[1];
  if (hash && hash.length > 0) {
    return hash;
  }

  return fallback
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

function flattenToc(entries: TocEntry[]): TocHeading[] {
  const flat: TocHeading[] = [];
  for (const entry of entries) {
    const id = slugFromHash(entry.url, entry.title);
    flat.push({ id, title: entry.title });

    if (entry.items?.length) {
      flat.push(...flattenToc(entry.items));
    }
  }
  return flat;
}

function buildDocIndexes(pages: SearchPage[], tag: string): AdvancedIndex[] {
  return sortedByURL(pages).map(page => ({
    title: page.data.title,
    description: page.data.description,
    structuredData: page.data.structuredData,
    id: page.url,
    url: page.url,
    tag,
  }));
}

function buildPostStructuredData(post: Post): AdvancedIndex["structuredData"] {
  const headings = flattenToc(post.toc);

  return {
    headings: headings.map(heading => ({
      id: heading.id,
      content: heading.title,
    })),
    contents: [
      {
        heading: undefined,
        content: joinContent(post.description, post.excerpt),
      },
      ...headings.map(heading => ({
        heading: heading.id,
        content: heading.title,
      })),
    ],
  };
}

function buildPostIndexes(posts: Post[]): AdvancedIndex[] {
  return sortedByURL(posts.map(post => ({ ...post, url: post.permalink }))).map(post => ({
    id: post.permalink,
    title: post.title,
    description: post.description,
    breadcrumbs: ["Blog"],
    tag: "Blog",
    structuredData: buildPostStructuredData(post),
    url: post.permalink,
  }));
}

function buildReleaseStructuredData(release: Release): AdvancedIndex["structuredData"] {
  const sections = [
    { id: "summary", title: "Summary", content: release.summary },
    { id: "added", title: "Added", content: release.added.join("\n") },
    { id: "changed", title: "Changed", content: release.changed.join("\n") },
    { id: "fixed", title: "Fixed", content: release.fixed.join("\n") },
    { id: "breaking", title: "Breaking", content: release.breaking.join("\n") },
  ].filter(section => section.content.trim().length > 0);

  return {
    headings: sections.map(section => ({
      id: section.id,
      content: section.title,
    })),
    contents: sections.map(section => ({
      heading: section.id,
      content: section.content,
    })),
  };
}

function buildReleaseIndexes(releases: Release[]): AdvancedIndex[] {
  return sortedByURL(
    releases.map(release => ({
      ...release,
      url: `/changelog#${release.version}`,
    }))
  ).map(release => ({
    id: release.url,
    title: release.version,
    description: release.summary,
    breadcrumbs: ["Changelog"],
    tag: "Changelog",
    structuredData: buildReleaseStructuredData(release),
    url: release.url,
  }));
}

export function buildPublicSearchIndexes(): AdvancedIndex[] {
  return [
    ...buildDocIndexes(runtimeDocs.getPages(), "Runtime"),
    ...buildDocIndexes(protocolDocs.getPages(), "AGH Network"),
    ...buildPostIndexes(allPosts()),
    ...buildReleaseIndexes(allReleases()),
  ].toSorted(byURL);
}
