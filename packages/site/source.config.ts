import { COMPOZY_CODE_THEMES } from "@compozy/ui/lib/code-theme";
import { pageSchema } from "fumadocs-core/source/schema";
import { defineDocs, defineConfig } from "fumadocs-mdx/config";
import { z } from "zod";
import { MATURITY_LEVELS } from "./lib/doc-maturity";

// Maturity is a claim about the runtime, so it lives in frontmatter rather than prose: the masthead
// renders it in one place and COPY.md's claim standards apply to a fixed vocabulary instead of to
// whatever wording a page invents. Pages that document shipped behavior omit it.
export const docs = defineDocs({
  dir: "content/docs",
  docs: {
    schema: pageSchema.extend({
      maturity: z.enum(MATURITY_LEVELS).optional(),
    }),
  },
});

export default defineConfig({
  mdxOptions: {
    rehypeCodeOptions: {
      themes: {
        light: COMPOZY_CODE_THEMES.light,
        dark: COMPOZY_CODE_THEMES.dark,
      },
    },
  },
});
