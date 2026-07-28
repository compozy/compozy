import type { MetadataRoute } from "next";
import { BLOG_CATEGORIES, allPosts } from "@/lib/blog";
import { protocolDocs, runtimeDocs } from "@/lib/source";
import { absoluteUrl, canonicalPath } from "@/lib/site-config";

export const dynamic = "force-static";

const LLM_DISCOVERY_PATHS = ["/llms.txt", "/llms-full.txt"] as const;

function pageEntry(path: string): MetadataRoute.Sitemap[number] {
  const isFile = LLM_DISCOVERY_PATHS.includes(path as (typeof LLM_DISCOVERY_PATHS)[number]);
  return {
    url: isFile ? absoluteUrl(path) : absoluteUrl(canonicalPath(path)),
    changeFrequency: "weekly",
    priority: path === "/" ? 1 : isFile ? 0.5 : 0.7,
  };
}

export default function sitemap(): MetadataRoute.Sitemap {
  const docsPaths = [...runtimeDocs.getPages(), ...protocolDocs.getPages()].map(page => page.url);
  const blogPaths = allPosts().map(post => post.permalink);
  const categoryPaths = BLOG_CATEGORIES.map(category => `/blog/categories/${category}`);
  const paths = Array.from(
    new Set([
      "/",
      "/blog",
      "/changelog",
      ...docsPaths,
      ...blogPaths,
      ...categoryPaths,
      ...LLM_DISCOVERY_PATHS,
    ])
  ).sort();

  return paths.map(pageEntry);
}
