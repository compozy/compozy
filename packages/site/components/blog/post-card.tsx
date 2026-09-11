import type { Post } from "#site/content";
import { Clock } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import { DateStamp } from "./date-stamp";
import { BulletDivider } from "./divider";
import { categoryLabel, formatReadingTime } from "./format";
import { Eyebrow } from "@compozy/ui";
import { blogPostCover } from "@/lib/blog";

export interface PostCardProps {
  post: Post;
}

export function PostCard({ post }: PostCardProps) {
  const cover = blogPostCover(post);

  return (
    <article className="group flex min-h-58 flex-col overflow-hidden rounded-xl border border-line bg-canvas-soft">
      {cover && (
        <div className="relative aspect-video shrink-0 border-b border-line bg-rail">
          <Image
            src={cover.src}
            alt={post.cover ? "" : cover.alt}
            fill
            sizes="(min-width: 1280px) 420px, (min-width: 1024px) 33vw, (min-width: 640px) 50vw, 100vw"
            className="object-cover"
          />
        </div>
      )}
      <div className="flex flex-1 flex-col p-5">
        <div className="flex flex-wrap items-center gap-2.5">
          <Eyebrow className="text-accent">{categoryLabel(post.category)}</Eyebrow>
          <BulletDivider />
          <DateStamp date={post.date} />
        </div>
        <h3 className="mt-4 font-sans text-xl font-medium leading-tight tracking-tight text-fg transition-colors group-hover:text-accent">
          <Link href={post.permalink}>{post.title}</Link>
        </h3>
        <p className="mt-3 mb-5 text-sm leading-7 text-muted">{post.description}</p>
        <div className="mt-auto flex items-center justify-between border-t border-line pt-3.5">
          <Eyebrow className="text-muted">{post.author}</Eyebrow>
          <span className="inline-flex items-center gap-1.5 text-eyebrow text-subtle">
            <Clock size={11} aria-hidden />
            <span className="font-mono tracking-mono">
              {formatReadingTime(post.metadata.readingTime)}
            </span>
          </span>
        </div>
      </div>
    </article>
  );
}
