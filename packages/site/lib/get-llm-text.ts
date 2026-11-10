import type { InferPageType } from "fumadocs-core/source";
import { SDK_GUIDE_PAGES } from "@/lib/sdk-reference-build";
import {
  expandSdkOverview,
  expandSdkResources,
  type SdkGuideLink,
} from "@/lib/sdk-reference-markdown";
import { docsSource } from "@/lib/source";
import { absoluteUrl } from "@/lib/site-config";

type DocsPage = InferPageType<typeof docsSource>;

/**
 * Both texts below are compiled into the page module by `source.config.ts`, so a revalidating route
 * never reads `content/docs` from disk: a deployed function does not ship those files.
 */

/** The page's MDX body as authored, without its frontmatter: what the page templates parse. */
export function getDocsPageSource(page: DocsPage): string {
  const raw = page.data._exports.rawSource;
  if (typeof raw !== "string") {
    throw new Error(
      `${page.path} has no compiled rawSource: check postprocess in source.config.ts`
    );
  }
  return raw;
}

/** The client guides the SDK overview lists, in sidebar order. */
function sdkGuides(): SdkGuideLink[] {
  return SDK_GUIDE_PAGES.flatMap(slug => {
    const page = docsSource.getPage(["sdk", slug]);
    return page
      ? [{ url: page.url, title: page.data.title, description: page.data.description ?? "" }]
      : [];
  });
}

export async function getLLMText(page: DocsPage): Promise<string> {
  // The authored Markdown with its includes embedded and its imports and exports dropped.
  const value = await page.data.getText("processed");
  const description = page.data.description ?? "";

  return [
    `# ${page.data.title}`,
    `URL: ${absoluteUrl(page.url)}`,
    "",
    description,
    "",
    // SDK pages hold components; their Markdown lists what they render.
    expandSdkOverview(expandSdkResources(value), sdkGuides),
  ].join("\n");
}
