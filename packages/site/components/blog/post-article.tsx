/** One post: an editorial opening, a numbered reading column, and a section rail that fills as you read. */
import { cn } from "@compozy/ui";
import { ArrowLeft } from "@compozy/ui/icons";
import Link from "next/link";
import type { ReactNode } from "react";

import { CONTAINER, HAIRLINE } from "@/components/site/layout";
import { H2 } from "@/components/site/primitives";
import type { PostAuthor } from "@/lib/blog";
import { formatPostDate, topicLabel, type BlogEntry } from "@/lib/blog-index";

import { AuthorAvatar } from "./author-avatar";
import { PosterCard } from "./cards";
import { CopyLink } from "./copy-link";
import { CoverArt } from "./cover-art";
import { categoryLabel } from "./format";
import { PROSE } from "./prose-rhythm";
import { ReadingLayout, type RailSection } from "./reading-layout";

function Fact({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <dt className="text-[13px] text-subtle">{label}</dt>
      <dd className="m-0 mt-1 text-[15px] text-fg">{children}</dd>
    </div>
  );
}

function AuthorLink({ author, children }: { author: PostAuthor; children: ReactNode }) {
  const className = "flex items-center gap-3";
  return author.url ? (
    <a href={author.url} target="_blank" rel="noopener noreferrer" className={className}>
      {children}
    </a>
  ) : (
    <div className={className}>{children}</div>
  );
}

export function PostArticle({
  entry,
  author,
  sections,
  related,
  children,
}: {
  entry: BlogEntry;
  author: PostAuthor;
  sections: readonly RailSection[];
  related: readonly BlogEntry[];
  /** The rendered MDX body. */
  children: ReactNode;
}) {
  return (
    <article className={cn(CONTAINER, "pt-[120px] pb-28 md:pt-[136px]")}>
      <nav
        aria-label="Breadcrumb"
        className="flex items-center gap-2.5 text-[14px] text-muted-foreground"
      >
        <Link
          href="/blog"
          className="inline-flex items-center gap-2 outline-none transition-colors hover:text-fg focus-visible:text-fg"
        >
          <ArrowLeft aria-hidden className="size-4" /> Blog
        </Link>
        <span aria-hidden className="text-subtle">
          /
        </span>
        <Link
          href={`/blog/categories/${entry.category}`}
          className="outline-none transition-colors hover:text-fg focus-visible:text-fg"
        >
          {categoryLabel(entry.category)}
        </Link>
      </nav>
      <header className="mt-10 grid gap-x-16 gap-y-8 lg:grid-cols-[minmax(0,1fr)_300px] lg:items-end">
        <div>
          <p className="m-0 font-mono text-[13.5px] text-subtle">No. {entry.number}</p>
          <h1 className="m-0 mt-5 font-display text-[40px]/[1.04] font-normal tracking-[-0.03em] text-balance text-fg md:text-[72px]/[1]">
            {entry.title}
          </h1>
        </div>
        <p className="m-0 text-[18px]/[1.55] text-pretty text-muted-foreground lg:pb-2">
          {entry.description}
        </p>
      </header>
      <div
        className={cn(
          "mt-10 flex flex-wrap items-center justify-between gap-6 border-y py-5",
          HAIRLINE
        )}
      >
        <dl className="m-0 flex flex-wrap items-center gap-x-12 gap-y-4">
          <AuthorLink author={author}>
            <AuthorAvatar author={author} className="size-10 text-[15px]" />
            <Fact label="Written by">{author.name}</Fact>
          </AuthorLink>
          <Fact label="Published">{formatPostDate(entry.date)}</Fact>
          <Fact label="Reading time">{entry.minutes} min</Fact>
        </dl>
        <CopyLink permalink={entry.permalink} />
      </div>
      <div className="group/post relative mt-10 aspect-[16/9] overflow-hidden rounded-[24px] border border-site-light-090 bg-black md:aspect-[21/8]">
        <CoverArt cover={entry.cover} sizes="(min-width: 1280px) 1200px, 100vw" priority />
      </div>
      <ReadingLayout sections={sections}>
        <div className={cn("blog-body", PROSE)}>{children}</div>
        <ul className="m-0 mt-16 flex list-none flex-wrap gap-2 p-0">
          {entry.tags.map(tag => (
            <li
              key={tag}
              className="rounded-pill border border-site-light-120 px-3.5 py-1 text-[13.5px] text-muted-foreground"
            >
              {topicLabel(tag)}
            </li>
          ))}
        </ul>
        <div className="mt-10 flex flex-wrap items-center justify-between gap-5 rounded-[20px] border border-site-light-100 bg-site-light-025 p-6">
          <AuthorLink author={author}>
            <AuthorAvatar author={author} className="size-14 text-[20px]" />
            <span>
              <span className="block text-[17px] font-medium text-fg">{author.name}</span>
              {author.role ? (
                <span className="block text-[14.5px] text-muted-foreground">{author.role}</span>
              ) : null}
            </span>
          </AuthorLink>
          <CopyLink permalink={entry.permalink} />
        </div>
      </ReadingLayout>
      <section className={cn("mt-24 border-t pt-16", HAIRLINE)}>
        <H2 size="sm">Keep reading</H2>
        <div className="mt-10 grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {related.map(item => (
            <PosterCard key={item.slug} entry={item} />
          ))}
        </div>
      </section>
    </article>
  );
}
