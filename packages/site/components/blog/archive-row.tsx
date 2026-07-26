import type { Post } from "#site/content";
import { ArrowUpRight } from "lucide-react";
import Link from "next/link";
import { DateStamp } from "./date-stamp";
import { categoryLabel, formatReadingTime } from "./format";
import { Eyebrow } from "@agh/ui";

export interface ArchiveRowProps {
  post: Post;
}

export function ArchiveRow({ post }: ArchiveRowProps) {
  return (
    <Link
      href={post.permalink}
      className="group grid grid-cols-1 items-start gap-3 border-b border-line py-5 transition-colors hover:bg-canvas-soft sm:grid-cols-[104px_minmax(0,1fr)] sm:gap-x-5 lg:grid-cols-[88px_minmax(0,1fr)_minmax(96px,140px)_70px_16px] lg:gap-6"
    >
      <DateStamp date={post.date} format="compact" className="sm:pt-1" />
      <div className="min-w-0">
        <p className="font-sans text-site-lead font-medium leading-snug tracking-tight text-fg group-hover:text-accent">
          {post.title}
        </p>
        <p className="mt-1.5 text-sm leading-6 text-muted">{post.description}</p>
        {post.tags.length > 0 && (
          <div className="mt-2.5 flex flex-wrap gap-1.5">
            <Eyebrow className="rounded-chip bg-elevated px-1.5 py-0.5 text-subtle">
              {categoryLabel(post.category)}
            </Eyebrow>
            {post.tags.map(tag => (
              <span
                key={tag}
                className="rounded-chip bg-elevated px-1.5 py-0.5 font-mono text-badge tracking-mono text-subtle"
              >
                {tag}
              </span>
            ))}
          </div>
        )}
      </div>
      <Eyebrow className="min-w-0 truncate text-muted sm:col-start-2 lg:col-start-auto lg:pt-1">
        {post.author}
      </Eyebrow>
      <span className="font-mono text-eyebrow tracking-mono text-subtle sm:col-start-2 lg:col-start-auto lg:pt-1">
        {formatReadingTime(post.metadata.readingTime)}
      </span>
      <ArrowUpRight
        size={16}
        aria-hidden
        className="hidden self-center text-subtle group-hover:text-accent lg:block"
      />
    </Link>
  );
}
