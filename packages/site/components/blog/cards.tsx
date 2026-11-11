/** Composed covers: brand art, a number and the title set on it, so no post ships a bare image. */
import { cn } from "@compozy/ui";
import { ArrowRight } from "@compozy/ui/icons";
import Link from "next/link";

import type { PostAuthor } from "@/lib/blog";
import { formatPostDate, type BlogEntry } from "@/lib/blog-index";

import { AuthorAvatar } from "./author-avatar";
import { CoverArt } from "./cover-art";
import { categoryLabel } from "./format";

const LABEL = "font-mono text-[12.5px] text-fg/80";

function CoverLabels({ entry }: { entry: BlogEntry }) {
  return (
    <span className="absolute inset-x-6 top-5 flex items-center justify-between">
      <span className={LABEL}>{categoryLabel(entry.category)}</span>
      <span className={LABEL}>No. {entry.number}</span>
    </span>
  );
}

/** A portrait poster: the title lives on the cover. */
export function PosterCard({ entry, className }: { entry: BlogEntry; className?: string }) {
  return (
    <Link
      href={entry.permalink}
      className={cn(
        "group/post relative block aspect-[4/5] w-full overflow-hidden rounded-[22px] border border-site-light-090 bg-black p-0 text-left outline-none transition-[border-color] duration-300 hover:border-site-light-250 focus-visible:ring-2 focus-visible:ring-brand",
        className
      )}
    >
      <CoverArt
        cover={entry.cover}
        sizes="(min-width: 1024px) 400px, (min-width: 640px) 50vw, 100vw"
        className="h-[62%]"
      />
      <span aria-hidden className="blog-shade absolute inset-0" />
      <CoverLabels entry={entry} />
      <span className="absolute inset-x-0 bottom-0 block p-6">
        <span className="block font-display text-[25px]/[1.14] tracking-[-0.02em] text-balance text-fg">
          {entry.title}
        </span>
        <span className="mt-3 line-clamp-2 block text-[14.5px]/[1.5] text-muted-foreground">
          {entry.description}
        </span>
        <span className="mt-5 flex items-center justify-between text-[13.5px] text-subtle">
          <span>
            {formatPostDate(entry.date)} · {entry.minutes} min
          </span>
          <ArrowRight
            aria-hidden
            className="size-4 text-fg/0 transition-[color,transform] duration-300 group-hover/post:translate-x-1 group-hover/post:text-fg"
          />
        </span>
      </span>
    </Link>
  );
}

/** The lead story: art on one side, the pitch on the other. */
export function LeadStory({ entry, author }: { entry: BlogEntry; author: PostAuthor }) {
  return (
    <Link
      href={entry.permalink}
      className="group/post grid w-full overflow-hidden rounded-[28px] border border-site-light-100 bg-[#0e0e0e] p-0 text-left outline-none transition-[border-color] duration-300 hover:border-site-light-250 focus-visible:ring-2 focus-visible:ring-brand lg:grid-cols-[1.3fr_1fr]"
    >
      <span className="relative block min-h-[260px] overflow-hidden bg-black lg:min-h-[480px]">
        <CoverArt cover={entry.cover} sizes="(min-width: 1024px) 720px, 100vw" priority />
        <span aria-hidden className="blog-shade-side absolute inset-0 hidden lg:block" />
        <CoverLabels entry={entry} />
        <span
          aria-hidden
          className="absolute bottom-3 left-6 font-display text-[120px] leading-none tracking-[-0.05em] text-fg/90 lg:text-[176px]"
        >
          {entry.number}
        </span>
      </span>
      <span className="flex flex-col justify-between gap-10 p-7 lg:p-11">
        <span className="block">
          <span className={LABEL}>Featured</span>
          <span className="mt-5 block font-display text-[32px]/[1.08] tracking-[-0.025em] text-balance text-fg lg:text-[46px]/[1.04]">
            {entry.title}
          </span>
          <span className="mt-5 block text-[17px]/[1.6] text-pretty text-muted-foreground">
            {entry.description}
          </span>
        </span>
        <span className="flex flex-wrap items-center justify-between gap-4">
          <span className="flex items-center gap-3">
            <AuthorAvatar author={author} className="size-9 text-[14px]" />
            <span>
              <span className="block text-[14.5px] text-fg">{author.name}</span>
              <span className="block text-[13px] text-subtle">
                {formatPostDate(entry.date)} · {entry.minutes} min read
              </span>
            </span>
          </span>
          <span className="inline-flex items-center gap-2 text-[15px] text-fg">
            Read the post
            <ArrowRight
              aria-hidden
              className="size-4 transition-transform duration-300 group-hover/post:translate-x-1"
            />
          </span>
        </span>
      </span>
    </Link>
  );
}

/** One line of the index. Its cover peeks in on hover. */
export function IndexRow({ entry }: { entry: BlogEntry }) {
  return (
    <li className="border-t border-site-light-080 first:border-t-0">
      <Link
        href={entry.permalink}
        className="group/post relative grid w-full grid-cols-[40px_minmax(0,1fr)] items-baseline gap-x-5 bg-transparent py-6 text-left outline-none focus-visible:ring-2 focus-visible:ring-brand lg:grid-cols-[52px_minmax(0,1fr)_130px_70px_20px]"
      >
        <span className="font-mono text-[13px] text-subtle">{entry.number}</span>
        <span className="block min-w-0">
          <span className="block font-display text-[21px]/[1.2] tracking-[-0.015em] text-fg transition-colors group-hover/post:text-fg lg:text-[24px]/[1.2]">
            {entry.title}
          </span>
          <span className="mt-1.5 line-clamp-1 block max-w-[70ch] text-[15px] text-muted-foreground">
            {entry.description}
          </span>
          <span className="mt-2 block text-[13.5px] text-subtle lg:hidden">
            {categoryLabel(entry.category)} · {formatPostDate(entry.date)} · {entry.minutes} min
          </span>
        </span>
        <span className="hidden text-[14px] text-muted-foreground lg:block">
          {categoryLabel(entry.category)}
        </span>
        <span className="hidden text-[14px] text-subtle tabular-nums lg:block">
          {entry.minutes} min
        </span>
        <ArrowRight
          aria-hidden
          className="hidden size-4 self-center text-fg/0 transition-[color,transform] duration-300 group-hover/post:translate-x-1 group-hover/post:text-fg lg:block"
        />
        <span
          aria-hidden
          className="pointer-events-none absolute top-1/2 right-[250px] z-10 hidden aspect-[16/10] w-[210px] -translate-y-1/2 rotate-2 overflow-hidden rounded-[14px] border border-site-light-200 bg-black opacity-0 shadow-[0_30px_60px_-20px_#000] transition-[opacity,transform] duration-300 group-hover/post:-rotate-2 group-hover/post:opacity-100 xl:block"
        >
          <CoverArt cover={entry.cover} sizes="210px" />
        </span>
      </Link>
    </li>
  );
}
