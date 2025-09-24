import { source } from "@/lib/source";
import type { InferPageType } from "fumadocs-core/source";
import { remarkInclude } from "fumadocs-mdx/config";
import { remark } from "remark";
import remarkGfm from "remark-gfm";
import remarkMdx from "remark-mdx";

const processor = remark()
  .use(remarkMdx)
  // needed for Fumadocs MDX
  .use(remarkInclude)
  .use(remarkGfm);

export async function getLLMText(page: InferPageType<typeof source>) {
  const file = (page.data as typeof page.data & { _file?: { absolutePath: string } })._file;
  const content = (page.data as typeof page.data & { content?: string }).content ?? "";
  const processed = await processor.process({
    path: file?.absolutePath ?? page.url,
    value: content,
  });

  return `# ${page.data.title}
URL: ${page.url}

${page.data.description}

${processed.value}`;
}
