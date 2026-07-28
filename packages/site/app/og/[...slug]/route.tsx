import { notFound } from "next/navigation";
import { allPosts } from "@/lib/blog";
import { renderBlogOG } from "@/lib/og/templates/blog";
import { type DocsTree, renderDocsOG } from "@/lib/og/templates/docs";
import { protocolDocs, runtimeDocs } from "@/lib/source";

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
  const runtime = runtimeDocs
    .generateParams()
    .map(p => ({ slug: withImageSuffix(["runtime", ...(p.slug ?? [])]) }));
  const protocol = protocolDocs
    .generateParams()
    .map(p => ({ slug: withImageSuffix(["protocol", ...(p.slug ?? [])]) }));
  const blog = allPosts().map(post => ({
    slug: withImageSuffix(["blog", post.slug.replace(/^posts\//, "")]),
  }));

  return [...runtime, ...protocol, ...blog];
}

interface ResolvedDoc {
  kind: "doc";
  tree: DocsTree;
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

function resolveDoc(tree: DocsTree, rest: string[]): ResolvedDoc | null {
  const loader = tree === "runtime" ? runtimeDocs : protocolDocs;
  const page = loader.getPage(rest);
  if (!page) return null;
  return {
    kind: "doc",
    tree,
    title: page.data.title,
    description: page.data.description,
    path: [tree, ...rest].join("/"),
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
  if (tree === "runtime" || tree === "protocol") {
    resolved = resolveDoc(tree, rest);
  } else if (tree === "blog") {
    resolved = resolveBlog(rest);
  }
  if (!resolved) notFound();

  if (resolved.kind === "doc") {
    return renderDocsOG({
      tree: resolved.tree,
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
