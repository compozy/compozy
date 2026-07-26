import { AGH_CODE_THEMES } from "@agh/ui/lib/code-theme";
import { defineDocs, defineConfig } from "fumadocs-mdx/config";

export const runtime = defineDocs({
  dir: "content/runtime",
});

export const protocol = defineDocs({
  dir: "content/protocol",
});

export default defineConfig({
  mdxOptions: {
    rehypeCodeOptions: {
      themes: {
        light: AGH_CODE_THEMES.light,
        dark: AGH_CODE_THEMES.dark,
      },
    },
  },
});
