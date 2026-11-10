/** The blog front page: a lead story, the latest as posters, then a numbered index of everything. */
import { Empty, SearchInput, cn } from "@compozy/ui";
import { SearchX } from "@compozy/ui/icons";

import { CONTAINER, HAIRLINE } from "../landing/layout";
import { Reveal } from "../landing/motion";
import { H1, H2, Lead, SecondaryCta } from "../landing/primitives";
import { IndexRow, LeadStory, PosterCard } from "./cards";
import { CATEGORIES, POSTS, TOPICS, groupByMonth, matchesPost, topicLabel } from "./posts";

export interface Filters {
  category: string | null;
  topic: string | null;
  query: string;
}

const TABS = [{ id: null, label: "All" }, ...CATEGORIES] as const;
const CHIP =
  "inline-flex h-8 cursor-pointer items-center gap-2 rounded-[9px] bg-transparent px-3 text-[14px] outline-none transition-colors focus-visible:ring-2 focus-visible:ring-brand";

function count(id: string | null) {
  return id ? POSTS.filter(post => post.category === id).length : POSTS.length;
}

function Toolbar({ filters, onFilters }: { filters: Filters; onFilters: (next: Filters) => void }) {
  return (
    <div>
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div
          role="group"
          aria-label="Category"
          className="inline-flex gap-1 rounded-[12px] border border-[color:var(--light-100)] p-1"
        >
          {TABS.map(tab => (
            <button
              key={tab.label}
              type="button"
              aria-pressed={filters.category === tab.id}
              onClick={() => onFilters({ ...filters, category: tab.id })}
              className={cn(
                CHIP,
                filters.category === tab.id
                  ? "bg-[color:var(--light-100)] text-fg"
                  : "text-muted-foreground hover:text-fg"
              )}
            >
              {tab.label}
              <span className="font-mono text-[12px] text-subtle tabular-nums">
                {count(tab.id)}
              </span>
            </button>
          ))}
        </div>
        <SearchInput
          value={filters.query}
          onChange={query => onFilters({ ...filters, query })}
          placeholder="Search posts"
          aria-label="Search posts"
          containerClassName="h-10 w-full sm:w-[280px] [&_input]:text-[14px]"
        />
      </div>
      <div role="group" aria-label="Topic" className="mt-4 flex flex-wrap gap-2">
        {TOPICS.map(({ tag, count: uses }) => (
          <button
            key={tag}
            type="button"
            aria-pressed={filters.topic === tag}
            onClick={() => onFilters({ ...filters, topic: filters.topic === tag ? null : tag })}
            className={cn(
              "inline-flex h-8 cursor-pointer items-center gap-2 rounded-pill border px-3.5 text-[13.5px] outline-none transition-colors focus-visible:ring-2 focus-visible:ring-brand",
              filters.topic === tag
                ? "border-[color:var(--light-350)] bg-[color:var(--light-100)] text-fg"
                : "border-[color:var(--light-100)] bg-transparent text-muted-foreground hover:text-fg"
            )}
          >
            {topicLabel(tag)}
            <span className="font-mono text-[12px] text-subtle tabular-nums">{uses}</span>
          </button>
        ))}
      </div>
    </div>
  );
}

export function BlogIndex({
  filters,
  onFilters,
  onOpen,
}: {
  filters: Filters;
  onFilters: (next: Filters) => void;
  onOpen: (slug: string) => void;
}) {
  const needle = filters.query.trim().toLowerCase();
  const filtering = Boolean(filters.category || filters.topic || needle);
  const found = POSTS.filter(
    post =>
      (!filters.category || post.category === filters.category) &&
      (!filters.topic || post.tags.includes(filters.topic)) &&
      (!needle || matchesPost(post, needle))
  );
  const lead = POSTS.find(post => post.featured) ?? POSTS[0];
  const latest = POSTS.filter(post => post !== lead).slice(0, 3);
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
        <a href="/blog/feed.xml">
          <SecondaryCta>RSS feed</SecondaryCta>
        </a>
      </div>
      {filtering ? null : (
        <>
          <Reveal className="mt-12">
            <LeadStory post={lead} onOpen={onOpen} />
          </Reveal>
          <div className="mt-6 grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
            {latest.map((post, order) => (
              <Reveal key={post.slug} delay={order * 0.07}>
                <PosterCard post={post} onOpen={onOpen} />
              </Reveal>
            ))}
          </div>
        </>
      )}
      <section
        id="all-posts"
        className={cn("scroll-mt-28", filtering ? "mt-12" : "mt-24 border-t pt-16", HAIRLINE)}
      >
        <div className="flex flex-wrap items-baseline justify-between gap-4">
          <H2 size="sm">All posts</H2>
          <p className="m-0 font-mono text-[13.5px] text-muted-foreground">
            {found.length} of {POSTS.length}
          </p>
        </div>
        <div className="mt-8">
          <Toolbar filters={filters} onFilters={onFilters} />
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
                    {month.posts.length} {month.posts.length === 1 ? "post" : "posts"}
                  </p>
                </div>
                <ol className="m-0 list-none p-0">
                  {month.posts.map(post => (
                    <IndexRow key={post.slug} post={post} onOpen={onOpen} />
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
