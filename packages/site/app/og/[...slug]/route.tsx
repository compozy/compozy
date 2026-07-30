import { notFound } from "next/navigation";
import { allPosts } from "@/lib/blog";
import { renderBlogOG } from "@/lib/og/templates/blog";
import { type DocsOGVariant, renderDocsOG } from "@/lib/og/templates/docs";
import { docsSource } from "@/lib/source";

export const dynamic = "force-static";
export const contentType = "image/png";

interface RouteParams {
  slug: string[];
}

const TRAILING = "image.png" as const;

function withImageSuffix(slug: string[]): string[] {
  return [...slug, TRAILING];
}

export function generateStaticParams(): RouteParams[] {
  const docs = docsSource
    .generateParams()
    .map(p => ({ slug: withImageSuffix(["docs", ...(p.slug ?? [])]) }));
  const blog = allPosts().map(post => ({
    slug: withImageSuffix(["blog", post.slug.replace(/^posts\//, "")]),
  }));

  return [...docs, ...blog];
}

interface ResolvedDoc {
  kind: "doc";
  variant: DocsOGVariant;
  title: string;
  description?: string;
  path: string;
}

interface ResolvedBlog {
  kind: "blog";
  title: string;
  description?: string;
  slug: string;
  date?: string;
  author?: string;
}

type Resolved = ResolvedDoc | ResolvedBlog;

function resolveDoc(rest: string[]): ResolvedDoc | null {
  const page = docsSource.getPage(rest);
  if (!page) return null;
  const variant: DocsOGVariant =
    rest[0] === "network" && rest[1] === "protocol" ? "protocol" : "docs";
  return {
    kind: "doc",
    variant,
    title: page.data.title,
    description: page.data.description,
    path: ["docs", ...rest].join("/"),
  };
}

function resolveBlog(rest: string[]): ResolvedBlog | null {
  const slug = rest.join("/");
  const post = allPosts().find(p => p.slug === `posts/${slug}` || p.slug === slug);
  if (!post) return null;
  return {
    kind: "blog",
    title: post.title,
    description: post.description,
    slug,
    date: post.date,
    author: post.author,
  };
}

export async function GET(_req: Request, { params }: { params: Promise<RouteParams> }) {
  const { slug } = await params;
  if (slug.length < 2 || slug[slug.length - 1] !== TRAILING) notFound();

  const [tree, ...rest] = slug.slice(0, -1);
  let resolved: Resolved | null = null;
  if (tree === "docs") {
    resolved = resolveDoc(rest);
  } else if (tree === "blog") {
    resolved = resolveBlog(rest);
  }
  if (!resolved) notFound();

  if (resolved.kind === "doc") {
    return renderDocsOG({
      variant: resolved.variant,
      title: resolved.title,
      description: resolved.description,
      path: resolved.path,
    });
  }
  return renderBlogOG({
    title: resolved.title,
    description: resolved.description,
    slug: resolved.slug,
    date: resolved.date,
    author: resolved.author,
  });
}
