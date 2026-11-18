"use client";

/** The same model as the app: sections of six, View all into a category, search into one flat list. */
import { Button, Empty, SearchInput, cn } from "@compozy/ui";
import { ArrowRight, SearchX } from "@compozy/ui/icons";
import { useState } from "react";

import { ALL_ICON, CATEGORY_ICONS } from "@/components/marketplace/categories";
import { EntryCard } from "@/components/marketplace/entry-card";
import { CONTAINER } from "@/components/site/layout";
import { H3 } from "@/components/site/primitives";
import {
  categoriesWithEntries,
  categoryLabel,
  inCategory,
  matchesCard,
  type MarketplaceCard,
  type MarketplaceCategoryId,
} from "@/lib/marketplace-cards";

const PAGE = 48;
const GRID = "grid gap-3 sm:grid-cols-2 xl:grid-cols-3";

function RailItem({
  label,
  count,
  icon: Glyph,
  active,
  onSelect,
}: {
  label: string;
  count: number;
  icon: typeof ALL_ICON;
  active: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      aria-current={active ? "true" : undefined}
      onClick={onSelect}
      className={cn(
        "flex h-10 w-auto shrink-0 cursor-pointer items-center gap-2.5 rounded-[10px] px-3 text-left text-[14.5px] whitespace-nowrap outline-none transition-colors focus-visible:ring-2 focus-visible:ring-brand lg:w-full lg:shrink",
        active
          ? "bg-site-light-080 text-fg"
          : "text-muted-foreground hover:bg-site-light-040 hover:text-fg"
      )}
    >
      <Glyph aria-hidden className="size-4 shrink-0" />
      <span className="min-w-0 truncate lg:flex-1">{label}</span>
      <span className="font-mono text-[12px] text-subtle tabular-nums">{count}</span>
    </button>
  );
}

function Results({
  entries,
  showCategory,
}: {
  entries: readonly MarketplaceCard[];
  showCategory: boolean;
}) {
  const [shown, setShown] = useState(PAGE);
  return (
    <>
      <div className={GRID}>
        {entries.slice(0, shown).map(entry => (
          <EntryCard key={entry.id} entry={entry} showCategory={showCategory} />
        ))}
      </div>
      {entries.length > shown ? (
        <div className="mt-8 flex justify-center">
          <Button variant="outline" onClick={() => setShown(value => value + PAGE)}>
            Load more
          </Button>
        </div>
      ) : null}
    </>
  );
}

export function Browser({
  entries,
  category,
  query,
  onCategory,
  onQuery,
}: {
  entries: readonly MarketplaceCard[];
  category: MarketplaceCategoryId | null;
  query: string;
  onCategory: (id: MarketplaceCategoryId | null) => void;
  onQuery: (value: string) => void;
}) {
  const needle = query.trim().toLowerCase();
  const categories = categoriesWithEntries(entries);
  const scope = category ? inCategory(entries, category) : entries;
  const found = needle ? scope.filter(entry => matchesCard(entry, needle)) : scope;
  /** Browse follows Featured, which already leaves room; the filtered views follow the hero directly. */
  const browsing = !category && !needle;
  const title = needle
    ? `Results for “${query.trim()}”`
    : category
      ? categoryLabel(category)
      : "All";
  return (
    <section
      id="catalog"
      className={cn(
        CONTAINER,
        "pb-28",
        // The page's `scroll-padding-top` (88px, `site-shell.css`) already clears the header; these
        // margins add only the draft's remaining offset, so a jump lands where the draft puts it.
        browsing ? "scroll-mt-2" : "-scroll-mt-16 pt-16 md:-scroll-mt-22 md:pt-24"
      )}
    >
      <div className="grid gap-8 lg:grid-cols-[248px_minmax(0,1fr)] lg:gap-12">
        <nav
          aria-label="Categories"
          className="-mx-1 flex gap-1 overflow-x-auto px-1 lg:sticky lg:top-24 lg:mx-0 lg:flex-col lg:self-start lg:overflow-visible lg:px-0"
        >
          <RailItem
            label="All"
            count={entries.length}
            icon={ALL_ICON}
            active={category === null}
            onSelect={() => onCategory(null)}
          />
          {categories.map(item => (
            <RailItem
              key={item.id}
              label={item.label}
              count={item.count}
              icon={CATEGORY_ICONS[item.id]}
              active={category === item.id}
              onSelect={() => onCategory(item.id)}
            />
          ))}
        </nav>
        <div className="min-w-0">
          <div className="mb-7 flex flex-wrap items-center justify-between gap-4">
            <H3>
              {title}{" "}
              <span className="site-heading-count ml-1 font-mono text-[15px] text-subtle tabular-nums">
                {found.length}
              </span>
            </H3>
            <SearchInput
              value={query}
              onChange={onQuery}
              placeholder={category ? `Search ${categoryLabel(category)}` : "Search the catalog"}
              aria-label="Search the catalog"
              containerClassName="h-10 w-full sm:w-[280px] [&_input]:text-[14px]"
            />
          </div>
          {found.length === 0 ? (
            <Empty
              framed
              icon={SearchX}
              title={`Nothing matches “${query.trim()}”`}
              description="Try the name of the tool or company, or browse a category."
            />
          ) : needle || category ? (
            <Results key={`${category}-${needle}`} entries={found} showCategory={Boolean(needle)} />
          ) : (
            <div className="flex flex-col gap-14">
              {categories.map(item => {
                /** Featured entries already sit above; a section with nothing else to show stays out. */
                const rest = inCategory(entries, item.id)
                  .filter(entry => !entry.featured)
                  .slice(0, 6);
                if (rest.length === 0) return null;
                return (
                  <div key={item.id}>
                    <div className="mb-4 flex items-baseline justify-between gap-4">
                      <h4 className="m-0 text-[18px] font-medium text-fg">{item.label}</h4>
                      <button
                        type="button"
                        onClick={() => onCategory(item.id)}
                        className="group/all inline-flex cursor-pointer items-center gap-1.5 bg-transparent p-0 text-[14px] text-muted-foreground outline-none transition-colors hover:text-fg focus-visible:text-fg"
                      >
                        View all · {item.count}
                        <ArrowRight
                          aria-hidden
                          className="size-3.5 transition-transform duration-300 group-hover/all:translate-x-0.5"
                        />
                      </button>
                    </div>
                    <div className={cn(GRID, "max-sm:[&>*:nth-child(n+4)]:hidden")}>
                      {rest.map(entry => (
                        <EntryCard key={entry.id} entry={entry} />
                      ))}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </div>
    </section>
  );
}
