"use client";

import { Search } from "lucide-react";
import { useId, useState, type ReactNode } from "react";
import type { MarketplaceEntry, MarketplaceKind } from "@/lib/marketplace-catalog";
import { MarketplaceEntryCard } from "./marketplace-entry-card";

/**
 * The filter is client state, so it owns the input and the grid together. Entries arrive as the
 * already-parsed feed objects — plain JSON, safe to serialize across the boundary — and the tab strip
 * is server-rendered and passed through, so the toolbar reads as one row without duplicating the
 * route knowledge here.
 */

interface SearchableEntry {
  entry: MarketplaceEntry;
  haystack: string;
}

/** Folded once per entry, not once per keystroke: the needle is what changes while typing. */
function toSearchable(entries: MarketplaceEntry[]): SearchableEntry[] {
  const searchable: SearchableEntry[] = [];
  for (const entry of entries) {
    const tags = "tags" in entry && Array.isArray(entry.tags) ? entry.tags.join(" ") : "";
    const slug = "install_slug" in entry ? entry.install_slug : "";
    searchable.push({
      entry,
      haystack:
        `${entry.name} ${entry.description} ${entry.entry_id} ${slug} ${tags}`.toLowerCase(),
    });
  }
  return searchable;
}

export function MarketplaceKindBrowser({
  kind,
  entries,
  description,
  noun,
  tabs,
}: {
  kind: MarketplaceKind;
  entries: MarketplaceEntry[];
  description: string;
  /** Plural noun for the no-match line: "No skills match “x”." */
  noun: string;
  tabs: ReactNode;
}) {
  const [query, setQuery] = useState("");
  const inputId = useId();
  const trimmed = query.trim();

  // Both derivations are cached by the React Compiler; no manual memoization.
  const searchable = toSearchable(entries);
  const needle = trimmed.toLowerCase();
  const visible: MarketplaceEntry[] = [];
  for (const candidate of searchable) {
    if (needle.length === 0 || candidate.haystack.indexOf(needle) !== -1) {
      visible.push(candidate.entry);
    }
  }

  return (
    <>
      <div className="flex flex-wrap items-end gap-5 border-b border-line">
        {tabs}
        <div className="relative mb-1.5 ms-auto w-full sm:w-55">
          <Search
            aria-hidden
            className="pointer-events-none absolute start-2.5 top-1/2 size-3.5 -translate-y-1/2 text-subtle"
          />
          <input
            id={inputId}
            type="search"
            value={query}
            onChange={event => setQuery(event.target.value)}
            placeholder="Filter entries…"
            aria-label={`Filter ${noun}`}
            className="h-search w-full rounded-md border border-line bg-canvas py-0 ps-7.5 pe-2.5 text-eyebrow leading-none text-fg transition-colors outline-none placeholder:text-subtle focus-visible:border-line-strong"
          />
        </div>
      </div>

      <p className="mt-6.5 max-w-[76ch] text-sm leading-[1.65] text-muted">{description}</p>

      {visible.length > 0 ? (
        <div className="mt-5 flex flex-col gap-4">
          {visible.map(entry => (
            <MarketplaceEntryCard key={entry.entry_id} kind={kind} entry={entry} />
          ))}
        </div>
      ) : (
        <p className="py-9 text-center text-small-body text-subtle">
          No {noun} match <code className="font-mono text-fg">“{trimmed}”</code>.
        </p>
      )}
    </>
  );
}
