import type { Post } from "#site/content";
import { ArrowUpRight, Clock } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import { AuthorMeta } from "./author-meta";
import { DateStamp } from "./date-stamp";
import { BulletDivider } from "./divider";
import { categoryLabel, formatReadingTime } from "./format";
import { MonoBadge } from "./mono-badge";
import { blogPostCover } from "@/lib/blog";
import { Eyebrow } from "@compozy/ui";

export interface FeaturedPostProps {
  post: Post;
  authorInitial: string;
}

export function FeaturedPost({ post, authorInitial }: FeaturedPostProps) {
  const readingTime = formatReadingTime(post.metadata.readingTime);
  const cover = resolveFeaturedCover(post);

  return (
    <article
      className={`grid gap-10 rounded-xl border border-line bg-canvas-soft p-7 lg:items-center ${cover ? "lg:grid-cols-[minmax(0,1.05fr)_minmax(0,1fr)]" : ""}`}
    >
      <div className="order-2 lg:order-1">
        <div className="flex flex-wrap items-center gap-3">
          <MonoBadge tone="accent">FEATURED</MonoBadge>
          <BulletDivider />
          <Eyebrow className="text-muted">{categoryLabel(post.category)}</Eyebrow>
          <BulletDivider />
          <DateStamp date={post.date} />
        </div>
        <h2 className="mt-6 max-w-[20ch] font-display text-site-feature-title font-normal leading-none tracking-tight text-fg">
          <Link href={post.permalink} className="transition-colors hover:text-accent">
            {post.title}
          </Link>
        </h2>
        <p className="mt-5 max-w-[54ch] text-base leading-7 text-muted">{post.description}</p>
        <div className="mt-7 flex flex-wrap items-center gap-4">
          <AuthorMeta handle={post.author} initial={authorInitial} size="sm" />
          <BulletDivider />
          <span className="inline-flex items-center gap-1.5 text-xs text-subtle">
            <Clock size={12} aria-hidden />
            <span>{readingTime} read</span>
          </span>
          <Link
            href={post.permalink}
            className="ml-auto inline-flex items-center gap-1.5 text-sm font-medium text-accent"
          >
            Read post <ArrowUpRight size={14} aria-hidden />
          </Link>
        </div>
      </div>
      {cover && (
        <div className="order-1 lg:order-2">
          <FeaturedCover
            src={cover.src}
            alt={post.cover ? "" : cover.alt}
            width={cover.width}
            height={cover.height}
          />
        </div>
      )}
    </article>
  );
}

function resolveFeaturedCover(
  post: Post
): { src: string; alt: string; width: number; height: number } | null {
  return blogPostCover(post);
}

interface FeaturedCoverProps {
  src: string;
  alt: string;
  width: number;
  height: number;
}

function FeaturedCover({ src, alt, width, height }: FeaturedCoverProps) {
  return (
    <div className="overflow-hidden rounded-xl border border-line bg-rail">
      <Image
        src={src}
        alt={alt}
        width={width}
        height={height}
        priority
        sizes="(min-width: 1024px) 38vw, 100vw"
        className="block h-auto w-full"
      />
    </div>
  );
}
