/** Composed covers: brand art, a number and the title set on it, so no post ships a bare image. */
import { cn } from "@compozy/ui";
import { ArrowRight } from "@compozy/ui/icons";

import { coverOf } from "./covers";
import { AUTHOR, categoryLabel, formatDate, numberOf, type Post } from "./posts";

const LABEL = "font-mono text-[12.5px] text-fg/80";

export function CoverArt({ post, className }: { post: Post; className?: string }) {
  return (
    <img
      src={coverOf(post.slug)}
      alt=""
      draggable={false}
      loading="lazy"
      className={cn(
        "absolute inset-0 size-full object-cover transition-transform duration-[1400ms] ease-out group-hover/post:scale-[1.06]",
        className
      )}
    />
  );
}

function CoverLabels({ post }: { post: Post }) {
  return (
    <span className="absolute inset-x-6 top-5 flex items-center justify-between">
      <span className={LABEL}>{categoryLabel(post.category)}</span>
      <span className={LABEL}>No. {numberOf(post)}</span>
    </span>
  );
}

/** A portrait poster: the title lives on the cover. */
export function PosterCard({
  post,
  onOpen,
  className,
}: {
  post: Post;
  onOpen: (slug: string) => void;
  className?: string;
}) {
  return (
    <button
      type="button"
      onClick={() => onOpen(post.slug)}
      className={cn(
        "group/post relative block aspect-[4/5] w-full cursor-pointer overflow-hidden rounded-[22px] border border-[color:var(--light-090)] bg-black p-0 text-left outline-none transition-[border-color] duration-300 hover:border-[color:var(--light-250)] focus-visible:ring-2 focus-visible:ring-brand",
        className
      )}
    >
      <CoverArt post={post} className="h-[62%] object-center" />
      <span aria-hidden className="blog-shade absolute inset-0" />
      <CoverLabels post={post} />
      <span className="absolute inset-x-0 bottom-0 block p-6">
        <span className="block font-display text-[25px]/[1.14] tracking-[-0.02em] text-balance text-fg">
          {post.title}
        </span>
        <span className="mt-3 line-clamp-2 block text-[14.5px]/[1.5] text-muted-foreground">
          {post.description}
        </span>
        <span className="mt-5 flex items-center justify-between text-[13.5px] text-subtle">
          <span>
            {formatDate(post.date)} · {post.minutes} min
          </span>
          <ArrowRight
            aria-hidden
            className="size-4 text-fg/0 transition-[color,transform] duration-300 group-hover/post:translate-x-1 group-hover/post:text-fg"
          />
        </span>
      </span>
    </button>
  );
}

/** The lead story: art on one side, the pitch on the other. */
export function LeadStory({ post, onOpen }: { post: Post; onOpen: (slug: string) => void }) {
  return (
    <button
      type="button"
      onClick={() => onOpen(post.slug)}
      className="group/post grid w-full cursor-pointer overflow-hidden rounded-[28px] border border-[color:var(--light-100)] bg-[#0e0e0e] p-0 text-left outline-none transition-[border-color] duration-300 hover:border-[color:var(--light-250)] focus-visible:ring-2 focus-visible:ring-brand lg:grid-cols-[1.3fr_1fr]"
    >
      <span className="relative block min-h-[260px] overflow-hidden bg-black lg:min-h-[480px]">
        <CoverArt post={post} />
        <span aria-hidden className="blog-shade-side absolute inset-0 hidden lg:block" />
        <CoverLabels post={post} />
        <span
          aria-hidden
          className="absolute bottom-3 left-6 font-display text-[120px] leading-none tracking-[-0.05em] text-fg/90 lg:text-[176px]"
        >
          {numberOf(post)}
        </span>
      </span>
      <span className="flex flex-col justify-between gap-10 p-7 lg:p-11">
        <span className="block">
          <span className={LABEL}>Featured</span>
          <span className="mt-5 block font-display text-[32px]/[1.08] tracking-[-0.025em] text-balance text-fg lg:text-[46px]/[1.04]">
            {post.title}
          </span>
          <span className="mt-5 block text-[17px]/[1.6] text-pretty text-muted-foreground">
            {post.description}
          </span>
        </span>
        <span className="flex flex-wrap items-center justify-between gap-4">
          <span className="flex items-center gap-3">
            <img src={AUTHOR.avatar} alt="" className="size-9 rounded-full" />
            <span>
              <span className="block text-[14.5px] text-fg">{AUTHOR.name}</span>
              <span className="block text-[13px] text-subtle">
                {formatDate(post.date)} · {post.minutes} min read
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
    </button>
  );
}

/** One line of the index. Its cover peeks in on hover. */
export function IndexRow({ post, onOpen }: { post: Post; onOpen: (slug: string) => void }) {
  return (
    <li className="border-t border-[color:var(--light-080)] first:border-t-0">
      <button
        type="button"
        onClick={() => onOpen(post.slug)}
        className="group/post relative grid w-full cursor-pointer grid-cols-[40px_minmax(0,1fr)] items-baseline gap-x-5 bg-transparent py-6 text-left outline-none focus-visible:ring-2 focus-visible:ring-brand lg:grid-cols-[52px_minmax(0,1fr)_130px_70px_20px]"
      >
        <span className="font-mono text-[13px] text-subtle">{numberOf(post)}</span>
        <span className="block min-w-0">
          <span className="block font-display text-[21px]/[1.2] tracking-[-0.015em] text-fg transition-colors group-hover/post:text-fg lg:text-[24px]/[1.2]">
            {post.title}
          </span>
          <span className="mt-1.5 line-clamp-1 block max-w-[70ch] text-[15px] text-muted-foreground">
            {post.description}
          </span>
          <span className="mt-2 block text-[13.5px] text-subtle lg:hidden">
            {categoryLabel(post.category)} · {formatDate(post.date)} · {post.minutes} min
          </span>
        </span>
        <span className="hidden text-[14px] text-muted-foreground lg:block">
          {categoryLabel(post.category)}
        </span>
        <span className="hidden text-[14px] text-subtle tabular-nums lg:block">
          {post.minutes} min
        </span>
        <ArrowRight
          aria-hidden
          className="hidden size-4 self-center text-fg/0 transition-[color,transform] duration-300 group-hover/post:translate-x-1 group-hover/post:text-fg lg:block"
        />
        <span
          aria-hidden
          className="pointer-events-none absolute top-1/2 right-[250px] z-10 hidden aspect-[16/10] w-[210px] -translate-y-1/2 rotate-2 overflow-hidden rounded-[14px] border border-[color:var(--light-200)] bg-black opacity-0 shadow-[0_30px_60px_-20px_#000] transition-[opacity,transform] duration-300 group-hover/post:-rotate-2 group-hover/post:opacity-100 xl:block"
        >
          <img
            src={coverOf(post.slug)}
            alt=""
            className="absolute inset-0 size-full object-cover"
          />
        </span>
      </button>
    </li>
  );
}
