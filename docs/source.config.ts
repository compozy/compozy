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

const navigationLinks = [
  {
    title: "Temporal Modes",
    url: "/docs/deployment/temporal-modes",
    description: "Choose between remote and standalone Temporal modes",
  },
  {
    title: "Embedded Temporal",
    url: "/docs/architecture/embedded-temporal",
    description: "Technical deep-dive on embedded Temporal server implementation",
  },
  {
    title: "Temporal Troubleshooting",
    url: "/docs/troubleshooting/temporal",
    description: "Common Temporal issues and solutions",
  },
];

const config = defineConfig({
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

(config as { navigationLinks?: typeof navigationLinks }).navigationLinks = navigationLinks;

export default config;
