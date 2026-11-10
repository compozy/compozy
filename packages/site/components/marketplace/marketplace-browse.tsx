"use client";

/**
 * The browsing half of `/marketplace`: hero search, Featured and the catalog share one view state.
 * The category is mirrored to `?category=` so an entry page can link back into it; search stays local.
 */
import { useSearchParams } from "next/navigation";
import { useEffect, useRef, useState } from "react";

import { Browser } from "@/components/marketplace/browser";
import { Featured } from "@/components/marketplace/featured";
import { MarketplaceHero } from "@/components/marketplace/hero";
import {
  categoriesWithEntries,
  isCategoryId,
  type MarketplaceCard,
  type MarketplaceCategoryId,
} from "@/lib/marketplace-cards";

function jumpToCatalog() {
  document.getElementById("catalog")?.scrollIntoView?.({ behavior: "smooth", block: "start" });
}

/** Native `replaceState` keeps `useSearchParams` in sync without a navigation or a scroll reset. */
function writeCategory(category: MarketplaceCategoryId | null) {
  const url = new URL(window.location.href);
  if (category) url.searchParams.set("category", category);
  else url.searchParams.delete("category");
  window.history.replaceState(null, "", url);
}

/**
 * Controlled by the URL: the category comes in as a prop and a pick goes out through
 * `writeCategory`, so `/marketplace` (header, footer, back) and `?category=` always agree with the
 * view. Search stays local.
 */
export function MarketplaceBrowse({
  entries,
  category,
}: {
  entries: readonly MarketplaceCard[];
  category: MarketplaceCategoryId | null;
}) {
  const [query, setQuery] = useState("");
  const featured = entries.filter(entry => entry.featured);
  const categoryCount = categoriesWithEntries(entries).length;
  // The jump waits for the render the new category causes: Featured leaves the page with it, so
  // jumping earlier would aim at where the catalog used to be.
  const jumpPending = useRef(false);
  useEffect(() => {
    if (!jumpPending.current) return;
    jumpPending.current = false;
    jumpToCatalog();
  }, [category]);
  const browseCategory = (next: MarketplaceCategoryId | null) => {
    setQuery("");
    jumpPending.current = next !== category;
    writeCategory(next);
    if (next === category) jumpToCatalog();
  };
  const browsing = category === null && query.trim() === "";
  return (
    <>
      <MarketplaceHero
        entries={entries}
        categoryCount={categoryCount}
        query={query}
        onQuery={value => {
          setQuery(value);
          requestAnimationFrame(() => {
            jumpToCatalog();
            document
              .querySelector<HTMLInputElement>("#catalog input")
              ?.focus({ preventScroll: true });
          });
        }}
      />
      {browsing ? <Featured entries={featured} /> : null}
      <Browser
        entries={entries}
        category={category}
        query={query}
        onCategory={browseCategory}
        onQuery={setQuery}
      />
    </>
  );
}

/** Reads the category from the URL on every change; render it inside a Suspense boundary. */
export function MarketplaceBrowseFromUrl({ entries }: { entries: readonly MarketplaceCard[] }) {
  const requested = useSearchParams().get("category");
  return (
    <MarketplaceBrowse entries={entries} category={isCategoryId(requested) ? requested : null} />
  );
}
