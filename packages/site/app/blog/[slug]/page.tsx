import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { categoryLabel } from "@/components/blog/format";
import { railSections } from "@/components/blog/heading-slug";
import { MdxContent } from "@/components/blog/mdx-content";
import { PostArticle } from "@/components/blog/post-article";
import {
  ArticleJsonLd,
  BreadcrumbListJsonLd,
  type BreadcrumbItem,
} from "@/components/seo/structured-data";
import {
  allPosts,
  authorByHandle,
  blogEntry,
  blogPostCover,
  postAuthor,
  postBySlug,
  relatedPosts,
} from "@/lib/blog";
import { createPageMetadata } from "@/lib/site-config";

interface PageProps {
  params: Promise<{ slug: string }>;
}

export function generateStaticParams() {
  return allPosts().map(post => ({ slug: post.slug.replace(/^posts\//, "") }));
}

export async function generateMetadata({ params }: PageProps): Promise<Metadata> {
  const { slug } = await params;
  const post = postBySlug(slug);
  if (!post) return {};
  const cover = blogPostCover(post);
  const author = authorByHandle(post.author);
  const metadata = createPageMetadata({
    title: post.title,
    description: post.description,
    path: post.permalink,
    image: {
      url: cover.src,
      alt: cover.alt,
      width: cover.width,
      height: cover.height,
    },
    keywords: post.tags,
  });
  return {
    ...metadata,
    authors: [{ name: author?.name ?? post.author, url: author?.github }],
    openGraph: {
      ...metadata.openGraph,
      type: "article",
      publishedTime: post.date,
      modifiedTime: post.updated ?? post.date,
      authors: author?.github ? [author.github] : undefined,
      section: categoryLabel(post.category),
      tags: post.tags,
    },
    robots: {
      index: true,
      follow: true,
      googleBot: { index: true, follow: true, "max-image-preview": "large" },
    },
  };
}

export default async function BlogPostPage({ params }: PageProps) {
  const { slug } = await params;
  const post = postBySlug(slug);
  if (!post) notFound();

  const author = authorByHandle(post.author);
  const cover = blogPostCover(post);
  const breadcrumbs: BreadcrumbItem[] = [
    { name: "Home", path: "/" },
    { name: "Blog", path: "/blog/" },
    { name: post.title, path: `${post.permalink}/` },
  ];

  return (
    <>
      <ArticleJsonLd
        title={post.title}
        description={post.description}
        path={post.permalink}
        imageUrl={cover.src}
        datePublished={post.date}
        dateModified={post.updated ?? post.date}
        authorName={author?.name ?? post.author}
        authorUrl={author?.github}
        keywords={post.tags}
      />
      <BreadcrumbListJsonLd items={breadcrumbs} />
      <PostArticle
        entry={blogEntry(post)}
        author={postAuthor(post)}
        sections={railSections(post.toc)}
        related={relatedPosts(post).map(blogEntry)}
      >
        <MdxContent code={post.body} />
      </PostArticle>
    </>
  );
}
