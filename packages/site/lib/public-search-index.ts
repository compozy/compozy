import type { AdvancedIndex } from "fumadocs-core/search/server";
import { allPosts, type Post } from "@/lib/blog";
import { loadChangelogReleases } from "@/lib/changelog/github-client";
import type { ChangelogRelease } from "@/lib/changelog/types";
import { MARKETPLACE_KIND_META } from "@/components/marketplace/marketplace-kind-meta";
import { docsGroupForUrl } from "@/lib/docs-navigation";
import { bridgeProviders } from "@/lib/marketplace-bridges";
import { devCycleExtension } from "@/lib/marketplace-bundled";
import {
  bundledDevCycleDescription,
  MARKETPLACE_DESCRIPTION,
  marketplaceBridgesDescription,
} from "@/lib/marketplace-copy";
import { entriesForKind, installCommand } from "@/lib/marketplace-catalog";
import { docsSource } from "@/lib/source";

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

const DOC_PATH_LABELS: Readonly<Record<string, string>> = {
  cli: "CLI reference",
  api: "API reference",
  protocol: "Protocol spec",
  guide: "Implementation guide",
  mcp: "MCP",
  openai: "OpenAI",
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

function formatDocPathSegment(segment: string): string {
  const knownLabel = DOC_PATH_LABELS[segment];
  if (knownLabel) {
    return knownLabel;
  }

  const label = segment.replaceAll("-", " ");
  return label.charAt(0).toUpperCase() + label.slice(1);
}

function buildDocBreadcrumbs(url: string): string[] {
  const pathname = url.split(/[?#]/, 1)[0] ?? "";
  const segments = pathname.split("/").filter(Boolean);
  const ancestorCount = Math.max(1, segments.length - 1);

  // The leading `docs` segment renders as the page's D14 sidebar group so results mirror the IA.
  return segments
    .slice(0, ancestorCount)
    .map(segment => (segment === "docs" ? docsGroupForUrl(url) : formatDocPathSegment(segment)));
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

function buildDocIndexes(pages: SearchPage[]): AdvancedIndex[] {
  // Tag = the D14 sidebar group of the page, so search results mirror the sidebar IA.
  return sortedByURL(pages).map(page => ({
    title: page.data.title,
    description: page.data.description,
    structuredData: page.data.structuredData,
    id: page.url,
    url: page.url,
    breadcrumbs: buildDocBreadcrumbs(page.url),
    tag: docsGroupForUrl(page.url),
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

function buildReleaseStructuredData(release: ChangelogRelease): AdvancedIndex["structuredData"] {
  return {
    headings: release.headings.map(heading => ({
      id: heading.id,
      content: heading.label,
    })),
    contents: [{ heading: undefined, content: joinContent(release.summary, release.body) }],
  };
}

function buildReleaseIndexes(releases: ChangelogRelease[]): AdvancedIndex[] {
  return sortedByURL(
    releases.map(release => ({
      ...release,
      url: `/changelog/${encodeURIComponent(release.version)}`,
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

function buildMarketplaceIndexes(): AdvancedIndex[] {
  const overview: AdvancedIndex = {
    id: "/marketplace",
    url: "/marketplace",
    title: "Marketplace",
    description: MARKETPLACE_DESCRIPTION,
    breadcrumbs: ["Marketplace"],
    tag: "Marketplace",
    structuredData: { headings: [], contents: [] },
  };

  const kinds = MARKETPLACE_KIND_META.flatMap<AdvancedIndex>(meta => {
    const kindUrl = `/marketplace/${meta.kind}`;
    const entries = entriesForKind(meta.kind);
    return [
      {
        id: kindUrl,
        url: kindUrl,
        title: `${meta.title} — Marketplace`,
        description: meta.description,
        breadcrumbs: ["Marketplace"],
        tag: "Marketplace",
        structuredData: { headings: [], contents: [] },
      },
      ...entries.map<AdvancedIndex>(entry => ({
        id: `${kindUrl}/${entry.entry_id}`,
        url: `${kindUrl}/${entry.entry_id}`,
        title: entry.name,
        description: entry.description,
        breadcrumbs: ["Marketplace", meta.title],
        tag: "Marketplace",
        structuredData: {
          headings: [],
          contents: [
            {
              heading: undefined,
              content: joinContent(entry.description, installCommand(meta.kind, entry)),
            },
          ],
        },
      })),
    ];
  });

  const bridges: AdvancedIndex[] = [
    {
      id: "/marketplace/bridges",
      url: "/marketplace/bridges",
      title: "Bridges — Marketplace",
      description: marketplaceBridgesDescription(bridgeProviders.length),
      breadcrumbs: ["Marketplace"],
      tag: "Marketplace",
      structuredData: { headings: [], contents: [] },
    },
    ...bridgeProviders.map<AdvancedIndex>(provider => ({
      id: `/marketplace/bridges#${provider.platform}`,
      url: `/marketplace/bridges#${provider.platform}`,
      title: `${provider.displayName} bridge`,
      description: provider.description,
      breadcrumbs: ["Marketplace", "Bridges"],
      tag: "Marketplace",
      structuredData: {
        headings: [],
        contents: [{ heading: undefined, content: provider.description }],
      },
    })),
  ];

  const bundled: AdvancedIndex[] = [
    {
      id: "/marketplace/bundled/dev-cycle",
      url: "/marketplace/bundled/dev-cycle",
      title: `${devCycleExtension.displayName} — Marketplace`,
      description: bundledDevCycleDescription(devCycleExtension.description),
      breadcrumbs: ["Marketplace"],
      tag: "Marketplace",
      structuredData: {
        headings: [],
        contents: [
          {
            heading: undefined,
            content: joinContent(
              devCycleExtension.description,
              devCycleExtension.statusCommand,
              devCycleExtension.loops.map(loop => loop.name).join(", "),
              devCycleExtension.skills.join(", ")
            ),
          },
        ],
      },
    },
  ];

  return [overview, ...kinds, ...bridges, ...bundled];
}

export async function buildPublicSearchIndexes(): Promise<AdvancedIndex[]> {
  const changelog = await loadChangelogReleases();
  const releases = changelog.status === "ready" ? changelog.releases : [];
  return [
    ...buildDocIndexes(docsSource.getPages()),
    ...buildMarketplaceIndexes(),
    ...buildPostIndexes(allPosts()),
    ...buildReleaseIndexes(releases),
  ].toSorted(byURL);
}
