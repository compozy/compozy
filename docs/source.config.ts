import { defineConfig, defineDocs, frontmatterSchema, metaSchema } from "fumadocs-mdx/config";

// You can customise Zod schemas for frontmatter and `meta.json` here
// see https://fumadocs.vercel.app/docs/mdx/collections#define-docs
export const docs = defineDocs({
  dir: "content/docs",
  docs: {
    schema: frontmatterSchema,
  },
  meta: {
    schema: metaSchema,
  },
});

export default defineConfig({
  mdxOptions: {
    rehypeCodeOptions: {
      themes: {
        light: "vitesse-light",
        dark: "vitesse-dark",
      },
      langs: [
        "yaml",
        "yml",
        "typescript",
        "javascript",
        "tsx",
        "jsx",
        "json",
        "bash",
        "shell",
        "css",
        "html",
        "markdown",
      ],
    },
  },
});
