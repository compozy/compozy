/** One post: an editorial opening, a numbered reading column, and a section rail that fills as you read. */
import { Button, cn } from "@compozy/ui";
import { ArrowLeft, Check, Link2 } from "@compozy/ui/icons";
import { m, useReducedMotion, useScroll } from "motion/react";
import { useRef, useState, type RefObject } from "react";

import { CONTAINER, HAIRLINE } from "../landing/layout";
import { H2, Small } from "../landing/primitives";
import { CoverArt, PosterCard } from "./cards";
import { useActiveHeading } from "./hooks/use-active-heading";
import {
  AUTHOR,
  categoryLabel,
  formatDate,
  numberOf,
  relatedPosts,
  topicLabel,
  type Post,
} from "./posts";
import { ArticleBody } from "./prose";

function SectionRail({ post, body }: { post: Post; body: RefObject<HTMLDivElement | null> }) {
  const ids = post.toc.map(item => item.id);
  const active = useActiveHeading(ids);
  const reduced = useReducedMotion();
  const { scrollYProgress } = useScroll({ target: body, offset: ["start 35%", "end 65%"] });
  return (
    <nav
      aria-label="On this page"
      className="hidden lg:sticky lg:top-28 lg:block lg:self-start lg:pl-12"
    >
      <Small className="mb-4">On this page</Small>
      <div className="relative">
        <span aria-hidden className="absolute inset-y-0 left-0 w-0.5 bg-[color:var(--light-120)]">
          <m.span
            style={{ scaleY: reduced ? 1 : scrollYProgress }}
            className="absolute inset-0 origin-top bg-[linear-gradient(180deg,#1df0d7,#1f87fc)]"
          />
        </span>
        <ol className="m-0 list-none p-0">
          {post.toc.map((item, order) => (
            <li key={item.id}>
              <a
                href={`#${item.id}`}
                aria-current={active === item.id ? "location" : undefined}
                className={cn(
                  "flex gap-3 py-2 pl-5 text-[14px]/[1.4] transition-colors",
                  active === item.id ? "text-fg" : "text-muted-foreground hover:text-fg"
                )}
              >
                <span className="font-mono text-[12px]/[1.65] text-subtle tabular-nums">
                  {String(order + 1).padStart(2, "0")}
                </span>
                {item.text}
              </a>
            </li>
          ))}
        </ol>
      </div>
    </nav>
  );
}

function CopyLink({ slug }: { slug: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <Button
      variant="outline"
      onClick={() => {
        void navigator.clipboard?.writeText(`https://compozy.com/blog/${slug}`);
        setCopied(true);
      }}
    >
      {copied ? <Check /> : <Link2 />}
      {copied ? "Link copied" : "Copy link"}
    </Button>
  );
}

function Fact({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <dt className="text-[13px] text-subtle">{label}</dt>
      <dd className="m-0 mt-1 text-[15px] text-fg">{children}</dd>
    </div>
  );
}

export function BlogPost({
  post,
  onBack,
  onCategory,
  onOpen,
}: {
  post: Post;
  onBack: () => void;
  onCategory: (id: string) => void;
  onOpen: (slug: string) => void;
}) {
  const body = useRef<HTMLDivElement>(null);
  return (
    <article className={cn(CONTAINER, "pt-[120px] pb-28 md:pt-[136px]")}>
      <nav
        aria-label="Breadcrumb"
        className="flex items-center gap-2.5 text-[14px] text-muted-foreground"
      >
        <button
          type="button"
          onClick={onBack}
          className="inline-flex cursor-pointer items-center gap-2 bg-transparent p-0 outline-none transition-colors hover:text-fg focus-visible:text-fg"
        >
          <ArrowLeft aria-hidden className="size-4" /> Blog
        </button>
        <span aria-hidden className="text-subtle">
          /
        </span>
        <button
          type="button"
          onClick={() => onCategory(post.category)}
          className="cursor-pointer bg-transparent p-0 outline-none transition-colors hover:text-fg focus-visible:text-fg"
        >
          {categoryLabel(post.category)}
        </button>
      </nav>
      <header className="mt-10 grid gap-x-16 gap-y-8 lg:grid-cols-[minmax(0,1fr)_300px] lg:items-end">
        <div>
          <p className="m-0 font-mono text-[13.5px] text-subtle">No. {numberOf(post)}</p>
          <h1 className="m-0 mt-5 font-display text-[40px]/[1.04] font-normal tracking-[-0.03em] text-balance text-fg md:text-[72px]/[1]">
            {post.title}
          </h1>
        </div>
        <p className="m-0 text-[18px]/[1.55] text-pretty text-muted-foreground lg:pb-2">
          {post.description}
        </p>
      </header>
      <div
        className={cn(
          "mt-10 flex flex-wrap items-center justify-between gap-6 border-y py-5",
          HAIRLINE
        )}
      >
        <dl className="m-0 flex flex-wrap items-center gap-x-12 gap-y-4">
          <a href={AUTHOR.url} className="flex items-center gap-3">
            <img src={AUTHOR.avatar} alt="" className="size-10 rounded-full" />
            <Fact label="Written by">{AUTHOR.name}</Fact>
          </a>
          <Fact label="Published">{formatDate(post.date)}</Fact>
          <Fact label="Reading time">{post.minutes} min</Fact>
        </dl>
        <CopyLink slug={post.slug} />
      </div>
      <div className="group/post relative mt-10 aspect-[16/9] overflow-hidden rounded-[24px] border border-[color:var(--light-090)] bg-black md:aspect-[21/8]">
        <CoverArt post={post} />
      </div>
      <div className="mt-16 grid lg:grid-cols-[1fr_minmax(0,720px)_1fr]">
        <div ref={body} className="blog-prose min-w-0 lg:col-start-2">
          <ArticleBody segments={post.segments} />
          <ul className="m-0 mt-16 flex list-none flex-wrap gap-2 p-0">
            {post.tags.map(tag => (
              <li
                key={tag}
                className="rounded-pill border border-[color:var(--light-120)] px-3.5 py-1 text-[13.5px] text-muted-foreground"
              >
                {topicLabel(tag)}
              </li>
            ))}
          </ul>
          <div className="mt-10 flex flex-wrap items-center justify-between gap-5 rounded-[20px] border border-[color:var(--light-100)] bg-[color:var(--light-025)] p-6">
            <a href={AUTHOR.url} className="flex items-center gap-4">
              <img src={AUTHOR.avatar} alt="" className="size-14 rounded-full" />
              <span>
                <span className="block text-[17px] font-medium text-fg">{AUTHOR.name}</span>
                <span className="block text-[14.5px] text-muted-foreground">{AUTHOR.role}</span>
              </span>
            </a>
            <CopyLink slug={post.slug} />
          </div>
        </div>
        <SectionRail post={post} body={body} />
      </div>
      <section className={cn("mt-24 border-t pt-16", HAIRLINE)}>
        <H2 size="sm">Keep reading</H2>
        <div className="mt-10 grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {relatedPosts(post).map(item => (
            <PosterCard key={item.slug} post={item} onOpen={onOpen} />
          ))}
        </div>
      </section>
    </article>
  );
}
