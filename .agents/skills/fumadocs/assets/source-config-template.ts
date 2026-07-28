// Canonical `source.config.ts` for Fumadocs MDX.
// Lives at the repo root. ESM only — must be imported from a `.mjs` Next config
// (or a TS config when the Native Node TypeScript Resolver is enabled).
//
// Replace the bracketed comments with project-specific overrides; keep the
// imports stable so `pageSchema` / `metaSchema` extensions stay typed.

import { defineCollections, defineConfig, defineDocs } from "fumadocs-mdx/config";
import { metaSchema, pageSchema } from "fumadocs-core/source/schema";
import { z } from "zod";

// ───────────────────────────────────────────────────────────────────────────
// Tree-shaped docs surface (sidebar, nested folders, meta.json).
// Use `defineDocs` ONLY for the docs surface — flat content (blog, changelog,
// marketing pages) belongs in `defineCollections`.
// ───────────────────────────────────────────────────────────────────────────
export const docs = defineDocs({
  dir: "content/docs",
  docs: {
    schema: pageSchema.extend({
      // Add project-specific frontmatter fields here. Keep them serialisable —
      // Standard Schema runs at build time and the result is shipped to the client.
      // index: z.boolean().default(false),
    }),
    // postprocess: { includeProcessedMarkdown: true } // enable for `getText('processed')` / EPUB / llms-full.txt
  },
  meta: {
    schema: metaSchema.extend({
      // Add project-specific meta.json fields here.
      // showInSidebar: z.boolean().default(true),
    }),
  },
});

// ───────────────────────────────────────────────────────────────────────────
// Flat content surface (blog, changelog, releases). Each entry has its own
// schema and is independent of the docs page tree.
// ───────────────────────────────────────────────────────────────────────────
export const blog = defineCollections({
  type: "doc",
  dir: "content/blog",
  schema: z.object({
    title: z.string(),
    description: z.string().optional(),
    date: z.coerce.date(),
    author: z.string().optional(),
    draft: z.boolean().default(false),
  }),
});

// ───────────────────────────────────────────────────────────────────────────
// Global config — `mdxOptions`, plugins, workspaces.
//
// Avoid setting `mdxOptions` at the collection level; it WIPES this preset.
// If a collection genuinely needs an override, wrap with `applyMdxPreset(...)`
// to keep the docs preset (`remark-image`, `remark-heading`, `remark-structure`,
// `rehype-code`, `rehype-toc`).
// ───────────────────────────────────────────────────────────────────────────
export default defineConfig({
  mdxOptions: {
    // Use the function form whenever order matters.
    // rehypePlugins: (v) => [rehypeKatex, ...v],
    rehypeCodeOptions: {
      themes: {
        light: "vitesse-light",
        dark: "vitesse-dark",
      },
      // langs: ['ts', 'tsx', 'js', 'jsx', 'go', 'sh'], // pre-declare for Twoslash
      // transformers: [transformerTwoslash()],
    },
    // remarkImageOptions: { placeholder: 'blur' }, // Next.js only
    // remarkHeadingOptions: { /* ... */ },
    // preset: 'minimal', // drop default preset and accept raw ProcessorOptions
  },
  // workspaces: {
  //   design: { dir: 'design', config: await import('./design/source.config.ts') },
  // },
  // experimentalBuildCache: true,
});

// Recommended `tsconfig.json` alias (add manually):
// {
//   "compilerOptions": {
//     "paths": {
//       "collections/*": ["./.source/*"]
//     }
//   }
// }
//
// `.source/` is regenerated on `dev` / `build`. Prime it manually with:
//   npx fumadocs-mdx
// Hook into `package.json#postinstall` so types/entries exist before the dev
// server boots.
