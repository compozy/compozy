import type { InferPageType } from "fumadocs-core/source";
import { remarkInclude } from "fumadocs-mdx/config";
import { remark } from "remark";
import remarkGfm from "remark-gfm";
import remarkMdx from "remark-mdx";
import type { runtimeDocs } from "@/lib/source";
import { absoluteUrl } from "@/lib/site-config";

const processor = remark().use(remarkMdx).use(remarkInclude).use(remarkGfm);

type DocsPage = InferPageType<typeof runtimeDocs>;
type DocsPageDataExtras = DocsPage["data"] & {
  _file?: { absolutePath: string };
  content?: string;
};

export async function getLLMText(page: DocsPage): Promise<string> {
  const data = page.data as DocsPageDataExtras;
  const value = data.content ?? "";
  const path = data._file?.absolutePath ?? page.url;
  const processed = await processor.process({ path, value });
  const description = page.data.description ?? "";

  return [
    `# ${page.data.title}`,
    `URL: ${absoluteUrl(page.url)}`,
    "",
    description,
    "",
    String(processed.value),
  ].join("\n");
}
