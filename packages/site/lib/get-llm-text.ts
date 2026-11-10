import type { InferPageType } from "fumadocs-core/source";
import { remarkInclude } from "fumadocs-mdx/config";
import { remark } from "remark";
import remarkGfm from "remark-gfm";
import remarkMdx from "remark-mdx";
import type { docsSource } from "@/lib/source";
import { expandSdkResources } from "@/lib/sdk-reference-markdown";
import { absoluteUrl } from "@/lib/site-config";

const processor = remark().use(remarkMdx).use(remarkInclude).use(remarkGfm);

type DocsPage = InferPageType<typeof docsSource>;

const FRONTMATTER = /^---\r?\n[\s\S]*?\r?\n---\r?\n/;

/** The page's MDX body as authored, without its frontmatter. */
export async function getDocsPageSource(page: DocsPage): Promise<string> {
  return (await page.data.getText("raw")).replace(FRONTMATTER, "");
}

export async function getLLMText(page: DocsPage): Promise<string> {
  const value = await getDocsPageSource(page);
  const path = page.data.info.fullPath;
  const processed = await processor.process({ path, value });
  const description = page.data.description ?? "";

  return [
    `# ${page.data.title}`,
    `URL: ${absoluteUrl(page.url)}`,
    "",
    description,
    "",
    // Generated SDK stubs hold one component; their Markdown lists the methods it renders. It is
    // expanded after the MDX pass, which would read `<name>` in a description as JSX.
    expandSdkResources(String(processed.value)),
  ].join("\n");
}
