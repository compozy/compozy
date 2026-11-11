"use client";

/**
 * The blog front page: a lead story, the latest as posters, then a numbered index of everything.
 * The topic and search are mirrored to `?topic=` and `?q=`, and the category links carry them, so
 * switching category or coming back from a post keeps the filters.
 */
import { Empty, SearchInput, cn } from "@compozy/ui";
import { SearchX } from "@compozy/ui/icons";
import Link from "next/link";
import { useSearchParams } from "next/navigation";

import { useUrlBackedValue } from "@/components/site/hooks/use-url-backed-value";
import { CONTAINER, HAIRLINE } from "@/components/site/layout";
import { Entrance } from "@/components/site/motion";
import { H1, H2, Lead, SecondaryCta } from "@/components/site/primitives";
import type { PostAuthor } from "@/lib/blog";
import {
  filterEntries,
  groupByMonth,
  isFiltering,
  topTopics,
  topicLabel,
  type BlogEntry,
  type BlogFilters,
} from "@/lib/blog-index";

import { IndexRow, LeadStory, PosterCard } from "./cards";
import { categoryLabel } from "./format";

const CHIP =
  "inline-flex h-8 items-center gap-2 rounded-[9px] bg-transparent px-3 text-[14px] outline-none transition-colors focus-visible:ring-2 focus-visible:ring-brand";

function filterQuery(topic: string | null, query: string) {
  const params = new URLSearchParams();
  if (topic) params.set("topic", topic);
  if (query) params.set("q", query);
  const value = params.toString();
  return value ? `?${value}` : "";
}

function categoryHref(id: string | null, topic: string | null, query: string) {
  return `${id ? `/blog/categories/${id}` : "/blog"}${filterQuery(topic, query)}`;
}

/** Native `replaceState` keeps `useSearchParams` in sync without a navigation or a scroll reset. */
function writeFilters(topic: string | null, query: string) {
  const url = new URL(window.location.href);
  if (topic) url.searchParams.set("topic", topic);
  else url.searchParams.delete("topic");
  if (query) url.searchParams.set("q", query);
  else url.searchParams.delete("q");
  window.history.replaceState(null, "", url);
}

function Toolbar({
  entries,
  categories,
  filters,
  onTopic,
  onQuery,
}: {
  entries: readonly BlogEntry[];
  categories: readonly string[];
  filters: BlogFilters;
  onTopic: (topic: string | null) => void;
  onQuery: (query: string) => void;
}) {
  const tabs = [null, ...categories];
  return (
    <div>
      <div className="flex flex-wrap items-center justify-between gap-4">
        <nav
          aria-label="Category"
          className="inline-flex gap-1 rounded-[12px] border border-site-light-100 p-1"
        >
          {tabs.map(id => {
            const current = filters.category === id;
            return (
              <Link
                key={id ?? "all"}
                href={categoryHref(id, filters.topic, filters.query)}
                scroll={false}
                aria-current={current ? "page" : undefined}
                className={cn(
                  CHIP,
                  current ? "bg-site-light-100 text-fg" : "text-muted-foreground hover:text-fg"
                )}
              >
                {id ? categoryLabel(id) : "All"}
                <span className="font-mono text-[12px] text-subtle tabular-nums">
                  {id ? entries.filter(entry => entry.category === id).length : entries.length}
                </span>
              </Link>
            );
          })}
        </nav>
        <SearchInput
          value={filters.query}
          onChange={onQuery}
          placeholder="Search posts"
          aria-label="Search posts"
          containerClassName="h-10 w-full sm:w-[280px] [&_input]:text-[14px]"
        />
      </div>
      <div role="group" aria-label="Topic" className="mt-4 flex flex-wrap gap-2">
        {topTopics(entries).map(({ tag, count }) => (
          <button
            key={tag}
            type="button"
            aria-pressed={filters.topic === tag}
            onClick={() => onTopic(filters.topic === tag ? null : tag)}
            className={cn(
              "inline-flex h-8 cursor-pointer items-center gap-2 rounded-pill border px-3.5 text-[13.5px] outline-none transition-colors focus-visible:ring-2 focus-visible:ring-brand",
              filters.topic === tag
                ? "border-site-light-350 bg-site-light-100 text-fg"
                : "border-site-light-100 bg-transparent text-muted-foreground hover:text-fg"
            )}
          >
            {topicLabel(tag)}
            <span className="font-mono text-[12px] text-subtle tabular-nums">{count}</span>
          </button>
        ))}
      </div>
    </div>
  );
}

export interface BlogIndexProps {
  /** Every post, newest first. */
  entries: BlogEntry[];
  categories: readonly string[];
  /** The category route's category; `null` on `/blog`. */
  category: string | null;
  lead: BlogEntry;
  author: PostAuthor;
  /** The filters the URL carries; the index follows them, including changes made by navigation. */
  initialTopic?: string | null;
  initialQuery?: string;
}

export function BlogIndex({
  entries,
  categories,
  category,
  lead,
  author,
  initialTopic = null,
  initialQuery = "",
}: BlogIndexProps) {
  const [topic, setTopic] = useUrlBackedValue(initialTopic);
  const [query, setQuery] = useUrlBackedValue(initialQuery);
  const filters: BlogFilters = { category, topic, query };
  const changeTopic = (next: string | null) => {
    setTopic(next);
    writeFilters(next, query);
  };
  const changeQuery = (next: string) => {
    setQuery(next);
    writeFilters(topic, next);
  };
  const filtering = isFiltering(filters);
  const found = filterEntries(entries, filters);
  const latest = entries.filter(entry => entry.slug !== lead.slug).slice(0, 3);
  return (
    <div className={cn(CONTAINER, "pt-32 pb-28 md:pt-40")}>
      <div className="flex flex-wrap items-end justify-between gap-6">
        <div>
          <div className="[&_h1]:md:text-[80px]">
            <H1>Blog</H1>
          </div>
          <Lead className="mt-4 max-w-[560px] md:text-[19px]">
            Practical engineering for working with AI agents.
          </Lead>
        </div>
        <SecondaryCta href="/blog/feed.xml">RSS feed</SecondaryCta>
      </div>
      {filtering ? null : (
        <>
          <Entrance className="mt-12">
            <LeadStory entry={lead} author={author} />
          </Entrance>
          <div className="mt-6 grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
            {latest.map((entry, order) => (
              <Entrance key={entry.slug} delay={order * 0.07}>
                <PosterCard entry={entry} />
              </Entrance>
            ))}
          </div>
        </>
      )}
      <section
        id="all-posts"
        className={cn("scroll-mt-6", filtering ? "mt-12" : "mt-24 border-t pt-16", HAIRLINE)}
      >
        <div className="flex flex-wrap items-baseline justify-between gap-4">
          <H2 size="sm">All posts</H2>
          <p className="m-0 font-mono text-[13.5px] text-muted-foreground">
            {found.length} of {entries.length}
          </p>
        </div>
        <div className="mt-8">
          <Toolbar
            entries={entries}
            categories={categories}
            filters={filters}
            onTopic={changeTopic}
            onQuery={changeQuery}
          />
        </div>
        <div className="mt-10">
          {found.length === 0 ? (
            <Empty
              framed
              icon={SearchX}
              title="No post matches that"
              description="Try another word, or clear the category and topic."
            />
          ) : (
            groupByMonth(found).map(month => (
              <div
                key={month.label}
                className={cn(
                  "grid gap-x-12 border-t py-2 lg:grid-cols-[190px_minmax(0,1fr)]",
                  HAIRLINE
                )}
              >
                <div className="pt-6 lg:sticky lg:top-28 lg:self-start">
                  <p className="m-0 font-display text-[22px] leading-none tracking-[-0.01em] text-fg">
                    {month.label}
                  </p>
                  <p className="m-0 mt-2 text-[13.5px] text-subtle">
                    {month.entries.length} {month.entries.length === 1 ? "post" : "posts"}
                  </p>
                </div>
                <ol className="m-0 list-none p-0">
                  {month.entries.map(entry => (
                    <IndexRow key={entry.slug} entry={entry} />
                  ))}
                </ol>
              </div>
            ))
          )}
        </div>
      </section>
    </div>
  );
}

/** Opens the index on the URL's topic and search; render it inside a Suspense boundary. */
export function BlogIndexFromUrl(props: Omit<BlogIndexProps, "initialTopic" | "initialQuery">) {
  const params = useSearchParams();
  const requested = params.get("topic");
  // Only a topic the toolbar offers, so an active filter always has a pressed chip to clear it.
  const known = topTopics(props.entries).some(({ tag }) => tag === requested);
  return (
    <BlogIndex
      {...props}
      initialTopic={known ? requested : null}
      initialQuery={params.get("q") ?? ""}
    />
  );
}
