"use client";

/**
 * The browsing half of `/marketplace`: hero search, Featured and the catalog share one view state.
 * The category is mirrored to `?category=` so an entry page can link back into it; search stays local.
 */
import { useSearchParams } from "next/navigation";
import { useState } from "react";

import { Browser } from "@/components/marketplace/browser";
import { Featured } from "@/components/marketplace/featured";
import { MarketplaceHero } from "@/components/marketplace/hero";
import {
  categoriesWithEntries,
  isCategoryId,
  type MarketplaceCard,
  type MarketplaceCategoryId,
} from "@/lib/marketplace-view";

interface View {
  category: MarketplaceCategoryId | null;
  query: string;
}

function jumpToCatalog() {
  document.getElementById("catalog")?.scrollIntoView({ behavior: "smooth", block: "start" });
}

function writeCategory(category: MarketplaceCategoryId | null) {
  const url = new URL(window.location.href);
  if (category) url.searchParams.set("category", category);
  else url.searchParams.delete("category");
  window.history.replaceState(null, "", url);
}

export function MarketplaceBrowse({
  entries,
  initialCategory,
}: {
  entries: readonly MarketplaceCard[];
  initialCategory: MarketplaceCategoryId | null;
}) {
  const [view, setView] = useState<View>({ category: initialCategory, query: "" });
  const featured = entries.filter(entry => entry.featured);
  const categoryCount = categoriesWithEntries(entries).length;
  const browseCategory = (category: MarketplaceCategoryId | null) => {
    setView({ category, query: "" });
    writeCategory(category);
    requestAnimationFrame(jumpToCatalog);
  };
  const search = (query: string) => setView(current => ({ ...current, query }));
  const browsing = view.category === null && view.query.trim() === "";
  return (
    <>
      <MarketplaceHero
        entries={entries}
        categoryCount={categoryCount}
        query={view.query}
        onQuery={query => {
          search(query);
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
        category={view.category}
        query={view.query}
        onCategory={browseCategory}
        onQuery={search}
      />
    </>
  );
}

/** Reads the starting category from the URL; render it inside a Suspense boundary. */
export function MarketplaceBrowseFromUrl({ entries }: { entries: readonly MarketplaceCard[] }) {
  const requested = useSearchParams().get("category");
  const initialCategory = isCategoryId(requested) ? requested : null;
  return <MarketplaceBrowse entries={entries} initialCategory={initialCategory} />;
}
