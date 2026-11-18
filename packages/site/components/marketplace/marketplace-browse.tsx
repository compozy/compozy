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

function jumpToCatalog({ instant, focusSearch }: { instant: boolean; focusSearch: boolean }) {
  document
    .getElementById("catalog")
    ?.scrollIntoView?.({ behavior: instant ? "instant" : "smooth", block: "start" });
  if (focusSearch) {
    document.querySelector<HTMLInputElement>("#catalog input")?.focus({ preventScroll: true });
  }
}

/** The view a jump waits for: it runs after the render that shows this category and query. */
interface JumpTarget {
  category: MarketplaceCategoryId | null;
  query: string;
  instant: boolean;
  focusSearch: boolean;
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
  // A jump waits for the render that shows its target: Featured leaves or returns with the category
  // and the query, so jumping earlier would aim at where the catalog used to be. The category
  // arrives through the URL, possibly a render later; `jumpRequest` makes sure the check runs even
  // when nothing else changes.
  const pendingJump = useRef<JumpTarget | null>(null);
  const [jumpRequest, setJumpRequest] = useState(0);
  useEffect(() => {
    const target = pendingJump.current;
    if (!target || target.category !== category || target.query !== query) return;
    pendingJump.current = null;
    jumpToCatalog(target);
  }, [jumpRequest, category, query]);
  const requestJump = (target: JumpTarget) => {
    pendingJump.current = target;
    setJumpRequest(count => count + 1);
  };
  // A hard load of `#catalog` lands where the prerendered page (always All, with Featured) put the
  // catalog. This tree mounts with the URL's category already rendered, so land again from here.
  useEffect(() => {
    if (window.location.hash === "#catalog") jumpToCatalog({ instant: true, focusSearch: false });
  }, []);
  const browseCategory = (next: MarketplaceCategoryId | null) => {
    setQuery("");
    requestJump({ category: next, query: "", instant: false, focusSearch: false });
    if (next !== category) writeCategory(next);
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
          // Instant: typing moves on in the catalog's input, and a caret the browser keeps in view
          // would cut a smooth scroll short.
          requestJump({ category, query: value, instant: true, focusSearch: true });
        }}
      />
      {browsing ? <Featured entries={featured} /> : null}
      <Browser
        entries={entries}
        category={category}
        query={query}
        onCategory={browseCategory}
        onQuery={value => {
          setQuery(value);
          // Featured leaving or returning above would move the catalog out from under the reader.
          const nextBrowsing = category === null && value.trim() === "";
          if (nextBrowsing !== browsing) {
            requestJump({ category, query: value, instant: true, focusSearch: false });
          }
        }}
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
