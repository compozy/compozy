/** The same model as the app: sections of six, View all into a category, search into one flat list. */
import { Button, Empty, SearchInput, cn } from "@compozy/ui";
import { ArrowRight, SearchX } from "@compozy/ui/icons";
import { useState } from "react";

import { CONTAINER } from "../landing/layout";
import { H3 } from "../landing/primitives";
import { ALL_ICON, CATEGORIES, ENTRIES, categoryLabel, inCategory, matches } from "./catalog";
import { EntryCard } from "./entry-card";

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
  icon: (typeof CATEGORIES)[number]["icon"];
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
          ? "bg-[color:var(--light-080)] text-fg"
          : "text-muted-foreground hover:bg-[color:var(--light-040)] hover:text-fg"
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
  onOpen,
  showCategory,
}: {
  entries: typeof ENTRIES;
  onOpen: (id: string) => void;
  showCategory: boolean;
}) {
  const [shown, setShown] = useState(PAGE);
  return (
    <>
      <div className={GRID}>
        {entries.slice(0, shown).map(entry => (
          <EntryCard key={entry.id} entry={entry} onOpen={onOpen} showCategory={showCategory} />
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
  category,
  query,
  onCategory,
  onQuery,
  onOpen,
}: {
  category: string | null;
  query: string;
  onCategory: (id: string | null) => void;
  onQuery: (value: string) => void;
  onOpen: (id: string) => void;
}) {
  const needle = query.trim().toLowerCase();
  const scope = category ? inCategory(category) : ENTRIES;
  const found = needle ? scope.filter(entry => matches(entry, needle)) : scope;
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
        browsing ? "scroll-mt-24" : "scroll-mt-6 pt-16 md:scroll-mt-0 md:pt-24"
      )}
    >
      <div className="grid gap-8 lg:grid-cols-[248px_minmax(0,1fr)] lg:gap-12">
        <nav
          aria-label="Categories"
          className="-mx-1 flex gap-1 overflow-x-auto px-1 lg:sticky lg:top-24 lg:mx-0 lg:flex-col lg:self-start lg:overflow-visible lg:px-0"
        >
          <RailItem
            label="All"
            count={ENTRIES.length}
            icon={ALL_ICON}
            active={category === null}
            onSelect={() => onCategory(null)}
          />
          {CATEGORIES.map(item => (
            <RailItem
              key={item.id}
              label={item.label}
              count={inCategory(item.id).length}
              icon={item.icon}
              active={category === item.id}
              onSelect={() => onCategory(item.id)}
            />
          ))}
        </nav>
        <div className="min-w-0">
          <div className="mb-7 flex flex-wrap items-center justify-between gap-4">
            <H3>
              {title}{" "}
              <span className="marketplace-count ml-1 font-mono text-[15px] text-subtle tabular-nums">
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
            <Results
              key={`${category}-${needle}`}
              entries={found}
              onOpen={onOpen}
              showCategory={Boolean(needle)}
            />
          ) : (
            <div className="flex flex-col gap-14">
              {CATEGORIES.map(item => {
                const entries = inCategory(item.id);
                if (entries.length === 0) return null;
                return (
                  <div key={item.id}>
                    <div className="mb-4 flex items-baseline justify-between gap-4">
                      <h4 className="m-0 text-[18px] font-medium text-fg">{item.label}</h4>
                      <button
                        type="button"
                        onClick={() => onCategory(item.id)}
                        className="group/all inline-flex cursor-pointer items-center gap-1.5 bg-transparent p-0 text-[14px] text-muted-foreground outline-none transition-colors hover:text-fg focus-visible:text-fg"
                      >
                        View all · {entries.length}
                        <ArrowRight
                          aria-hidden
                          className="size-3.5 transition-transform duration-300 group-hover/all:translate-x-0.5"
                        />
                      </button>
                    </div>
                    <div className={cn(GRID, "max-sm:[&>*:nth-child(n+4)]:hidden")}>
                      {entries
                        .filter(entry => !entry.featured)
                        .slice(0, 6)
                        .map(entry => (
                          <EntryCard key={entry.id} entry={entry} onOpen={onOpen} />
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
