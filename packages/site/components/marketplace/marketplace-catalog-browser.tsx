"use client";

import { Empty, SearchInput } from "@compozy/ui";
import { SearchX } from "lucide-react";
import { useId, useState } from "react";
import type { ExtensionEntry } from "@/lib/marketplace-catalog";
import { MarketplaceEntryCard } from "./marketplace-entry-card";

/**
 * The one browsable catalog: search only, no kind tabs, no filters, no display mode (board 01, the
 * cards-only listing family). The filter is client state, so it owns the input and the grid
 * together. Entries arrive as parsed feed objects — plain JSON, safe across the boundary.
 */

interface SearchableEntry {
  entry: ExtensionEntry;
  haystack: string;
}

/** Folded once per entry, not once per keystroke: the needle is what changes while typing. */
function toSearchable(entries: readonly ExtensionEntry[]): SearchableEntry[] {
  return entries.map(entry => ({
    entry,
    haystack: [
      entry.name,
      entry.description,
      entry.entry_id,
      entry.install_slug,
      entry.author ?? "",
      entry.tier,
      ...(entry.inputs?.map(input => input.id) ?? []),
    ]
      .join(" ")
      .toLowerCase(),
  }));
}

export function MarketplaceCatalogBrowser({ entries }: { entries: readonly ExtensionEntry[] }) {
  const [query, setQuery] = useState("");
  const statusId = useId();
  const trimmed = query.trim();

  // Both derivations are cached by the React Compiler; no manual memoization.
  const searchable = toSearchable(entries.toSorted((a, b) => a.name.localeCompare(b.name)));
  const needle = trimmed.toLowerCase();
  const visible: ExtensionEntry[] = [];
  for (const candidate of searchable) {
    if (needle.length === 0 || candidate.haystack.includes(needle)) {
      visible.push(candidate.entry);
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p id={statusId} aria-live="polite" className="text-small-body text-subtle">
          {trimmed
            ? `${visible.length} of ${entries.length} match “${trimmed}”`
            : `${entries.length} extensions, listed by name`}
        </p>
        <SearchInput
          value={query}
          onChange={setQuery}
          placeholder="Search extensions…"
          aria-label="Search extensions"
          aria-describedby={statusId}
          containerClassName="w-full sm:w-64"
        />
      </div>

      {visible.length > 0 ? (
        <div className="grid gap-4 lg:grid-cols-2">
          {visible.map(entry => (
            <MarketplaceEntryCard key={entry.entry_id} entry={entry} />
          ))}
        </div>
      ) : (
        <Empty
          framed
          icon={SearchX}
          title={`No extensions match “${trimmed}”`}
          description="Search matches names, descriptions, ids, install slugs, authors, and input ids."
        />
      )}
    </div>
  );
}
