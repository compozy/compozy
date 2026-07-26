import type { Post } from "#site/content";
import { Clock } from "lucide-react";
import Link from "next/link";
import { DateStamp } from "./date-stamp";
import { BulletDivider } from "./divider";
import { categoryLabel, formatReadingTime } from "./format";
import { Eyebrow } from "@agh/ui";

export interface PostCardProps {
  post: Post;
}

export function PostCard({ post }: PostCardProps) {
  return (
    <article className="group flex min-h-58 flex-col rounded-xl border border-line bg-canvas-soft p-5">
      <div className="flex items-center gap-2.5">
        <Eyebrow className="text-accent">{categoryLabel(post.category)}</Eyebrow>
        <BulletDivider />
        <DateStamp date={post.date} />
      </div>
      <h3 className="mt-4 font-sans text-xl font-medium leading-tight tracking-tight text-fg transition-colors group-hover:text-accent">
        <Link href={post.permalink}>{post.title}</Link>
      </h3>
      <p className="mt-3 text-sm leading-7 text-muted">{post.description}</p>
      <div className="mt-auto flex items-center justify-between border-t border-line pt-3.5">
        <Eyebrow className="text-muted">{post.author}</Eyebrow>
        <span className="inline-flex items-center gap-1.5 text-eyebrow text-subtle">
          <Clock size={11} aria-hidden />
          <span className="font-mono tracking-mono">
            {formatReadingTime(post.metadata.readingTime)}
          </span>
        </span>
      </div>
    </article>
  );
}
