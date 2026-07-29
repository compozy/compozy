import { COMPOZY_CODE_THEMES } from "@compozy/ui/lib/code-theme";
import { defineDocs, defineConfig } from "fumadocs-mdx/config";

export const docs = defineDocs({
  dir: "content/docs",
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
