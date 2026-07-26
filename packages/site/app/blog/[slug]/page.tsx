import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ArrowLeft, Clock } from "lucide-react";
import { Eyebrow } from "@agh/ui";
import { AuthorMeta } from "@/components/blog/author-meta";
import { ContinueReading } from "@/components/blog/continue-reading";
import { DateStamp } from "@/components/blog/date-stamp";
import { BulletDivider } from "@/components/blog/divider";
import { categoryLabel, formatReadingTime } from "@/components/blog/format";
import { MdxContent } from "@/components/blog/mdx-content";
import { TocRail } from "@/components/blog/toc-rail";
import { flattenToc } from "@/components/blog/toc-utils";
import {
  ArticleJsonLd,
  BreadcrumbListJsonLd,
  type BreadcrumbItem,
} from "@/components/seo/structured-data";
import {
  allPosts,
  authorByHandle,
  authorInitial,
  blogPostCover,
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
  return createPageMetadata({
    title: post.title,
    description: post.description,
    path: post.permalink,
    image: cover
      ? {
          url: cover.src,
          alt: cover.alt,
          width: cover.width,
          height: cover.height,
        }
      : undefined,
    keywords: post.tags,
  });
}

export default async function BlogPostPage({ params }: PageProps) {
  const { slug } = await params;
  const post = postBySlug(slug);
  if (!post) notFound();

  const author = authorByHandle(post.author);
  const initial = authorInitial(post.author);
  const related = relatedPosts(post);
  const readingTime = formatReadingTime(post.metadata.readingTime);
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
        imageUrl={cover?.src ?? `/og/blog/${slug}/image.png`}
        datePublished={post.date}
        dateModified={post.updated ?? post.date}
        authorName={author?.name ?? post.author}
        keywords={post.tags}
      />
      <BreadcrumbListJsonLd items={breadcrumbs} />
      <section className="border-b border-line px-4 pt-14 pb-9">
        <div className="mx-auto max-w-site-layout-width">
          <div className="max-w-190">
            <Link
              href="/blog"
              className="inline-flex items-center gap-1.5 text-small-body text-subtle hover:text-fg"
            >
              <ArrowLeft size={13} aria-hidden />
              <span>Back to blog</span>
            </Link>
            <div className="mt-7 flex flex-wrap items-center gap-3">
              <Eyebrow className="text-accent">BLOG</Eyebrow>
              <BulletDivider />
              <Eyebrow className="text-muted">{categoryLabel(post.category)}</Eyebrow>
              <BulletDivider />
              <DateStamp date={post.date} />
              <BulletDivider />
              <span className="inline-flex items-center gap-1.5 text-eyebrow text-subtle">
                <Clock size={11} aria-hidden />
                <Eyebrow>{readingTime} read</Eyebrow>
              </span>
            </div>
            <h1 className="mt-7 font-display text-site-article-title font-normal leading-none tracking-tight text-fg">
              {post.title}
            </h1>
            <p className="mt-6 max-w-[58ch] text-site-lead leading-normal text-muted">
              {post.description}
            </p>
            <div className="mt-9 flex items-center justify-between gap-4">
              <AuthorMeta
                handle={author?.name ?? post.author}
                initial={initial}
                role={author?.role}
                size="md"
                layout="stacked"
              />
            </div>
          </div>
        </div>
      </section>

      <section className="px-4 pt-8 pb-16">
        <div className="mx-auto grid min-w-0 max-w-site-layout-width gap-12 lg:grid-cols-[minmax(0,760px)_220px]">
          <article className="prose-blog min-w-0 max-w-none">
            <MdxContent code={post.body} />
          </article>
          <TocRail items={flattenToc(post.toc)} />
        </div>
      </section>

      <ContinueReading posts={related} />
    </>
  );
}
