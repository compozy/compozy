import { COMPOZY_CODE_THEMES } from "@compozy/ui/lib/code-theme";
import { pageSchema } from "fumadocs-core/source/schema";
import { defineDocs, defineConfig } from "fumadocs-mdx/config";
import { z } from "zod";
import { MATURITY_LEVELS } from "./lib/doc-maturity";

/**
 * Keeps the authored MDX body (frontmatter blanked out by the loader) on `file.data.rawSource`, for
 * `postprocess.valueToExport` to compile into the page module.
 */
function remarkRawSource() {
  return (_tree: unknown, file: { value: unknown; data: Record<string, unknown> }) => {
    file.data.rawSource = String(file.value).replace(/^\n+/, "");
  };
}

// Maturity is a claim about the runtime, so it lives in frontmatter rather than prose: the masthead
// renders it in one place and COPY.md's claim standards apply to a fixed vocabulary instead of to
// whatever wording a page invents. Pages that document shipped behavior omit it.
export const docs = defineDocs({
  dir: "content/docs",
  docs: {
    schema: pageSchema.extend({
      maturity: z.enum(MATURITY_LEVELS).optional(),
    }),
    // Each page module carries its own text, because revalidation runs in a function that has no
    // `content/docs` files: `rawSource` is the authored body the CLI, API and SDK templates parse;
    // the processed Markdown (includes embedded, imports and exports dropped) feeds the llms routes.
    postprocess: {
      includeProcessedMarkdown: { headingIds: false },
      valueToExport: ["rawSource"],
    },
  },
});

export default defineConfig({
  mdxOptions: {
    remarkPlugins: [remarkRawSource],
    rehypeCodeOptions: {
      // The docs code frame labels each block by language and turns one-line shell commands into
      // a prompt line; the class on `code` is how the `pre` renderer learns the language.
      addLanguageClass: true,
      themes: {
        light: COMPOZY_CODE_THEMES.light,
        dark: COMPOZY_CODE_THEMES.dark,
      },
    },
  },
});
