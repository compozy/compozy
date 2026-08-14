import type { MetadataRoute } from "next";
import { BLOG_CATEGORIES, allPosts } from "@/lib/blog";
import { entriesForKind, MARKETPLACE_KINDS } from "@/lib/marketplace-catalog";
import { docsSource } from "@/lib/source";
import { absoluteUrl, canonicalPath } from "@/lib/site-config";
import { loadChangelogReleases } from "@/lib/changelog/github-client";

export const revalidate = 300;

const LLM_DISCOVERY_PATHS = ["/llms.txt", "/llms-full.txt"] as const;

function pageEntry(path: string): MetadataRoute.Sitemap[number] {
  const isFile = LLM_DISCOVERY_PATHS.includes(path as (typeof LLM_DISCOVERY_PATHS)[number]);
  return {
    url: isFile ? absoluteUrl(path) : absoluteUrl(canonicalPath(path)),
    changeFrequency: "weekly",
    priority: path === "/" ? 1 : isFile ? 0.5 : 0.7,
  };
}

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const docsPaths = docsSource.getPages().map(page => page.url);
  const blogPaths = allPosts().map(post => post.permalink);
  const categoryPaths = BLOG_CATEGORIES.map(category => `/blog/categories/${category}`);
  const changelog = await loadChangelogReleases();
  const releasePaths =
    changelog.status === "ready"
      ? changelog.releases.map(release => `/changelog/${encodeURIComponent(release.version)}`)
      : [];
  const marketplacePaths = [
    "/marketplace",
    "/marketplace/bridges",
    "/marketplace/bundled/spec-cycle",
    ...MARKETPLACE_KINDS.flatMap(kind => [
      `/marketplace/${kind}`,
      ...entriesForKind(kind).map(entry => `/marketplace/${kind}/${entry.entry_id}`),
    ]),
  ];
  const paths = Array.from(
    new Set([
      "/",
      "/blog",
      "/changelog",
      "/changelog/feed.xml",
      ...releasePaths,
      ...docsPaths,
      ...marketplacePaths,
      ...blogPaths,
      ...categoryPaths,
      ...LLM_DISCOVERY_PATHS,
    ])
  ).sort();

  return paths.map(pageEntry);
}
