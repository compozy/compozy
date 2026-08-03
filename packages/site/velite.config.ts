import { COMPOZY_CODE_THEMES } from "@compozy/ui/lib/code-theme";
import { defineConfig, s } from "velite";
import rehypePrettyCode, { type Options as RehypePrettyCodeOptions } from "rehype-pretty-code";

const wireKinds = ["greet", "whois", "say", "capability", "receipt", "trace"] as const;

const blogCategories = ["protocol", "runtime", "engineering", "network"] as const;

const prettyCodeOptions: Partial<RehypePrettyCodeOptions> = {
  theme: COMPOZY_CODE_THEMES.dark,
  keepBackground: false,
};

export default defineConfig({
  root: "content/blog",
  output: {
    data: ".velite",
    assets: "public/static/blog",
    base: "/static/blog/",
    name: "[name]-[hash:8].[ext]",
    clean: true,
    format: "esm",
  },
  collections: {
    posts: {
      name: "Post",
      pattern: "posts/**/*.mdx",
      schema: s
        .object({
          slug: s.path(),
          title: s.string().max(120),
          description: s.string().max(280),
          date: s.isodate(),
          updated: s.isodate().optional(),
          category: s.enum(blogCategories),
          tags: s.array(s.string()).default([]),
          author: s.string(),
          cover: s.image().optional(),
          kinds: s.array(s.enum(wireKinds)).default([]),
          featured: s.boolean().default(false),
          excerpt: s.excerpt({ length: 240 }),
          metadata: s.metadata(),
          toc: s.toc({ minDepth: 2, maxDepth: 3 }),
          body: s.mdx(),
        })
        .transform(post => ({
          ...post,
          permalink: `/blog/${post.slug.replace(/^posts\//, "")}`,
        })),
    },
    authors: {
      name: "Author",
      pattern: "authors/*.yml",
      schema: s.object({
        handle: s.string(),
        name: s.string(),
        bio: s.string().max(280),
        role: s.string().optional(),
        avatar: s.string(),
        github: s.string().url().optional(),
      }),
    },
  },
  mdx: {
    rehypePlugins: [[rehypePrettyCode, prettyCodeOptions]],
  },
});
