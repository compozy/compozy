"use client";

import { useEffect, useState } from "react";

/** One entry of the `/api/search` response (fumadocs advanced search). */
export interface SearchHit {
  id: string;
  url: string;
  type: "page" | "heading" | "text";
  content: string;
  breadcrumbs?: string[];
}

export interface SearchPageHit {
  url: string;
  title: string;
  /** The best matching heading or passage inside the page, when the match was in the body. */
  excerpt: string | null;
  breadcrumbs: string[];
}

const DELAY_MS = 120;
const cache = new Map<string, SearchPageHit[]>();

/** Index passages are Markdown (table rows included): the palette shows them as one plain line. */
function plainExcerpt(content: string): string {
  return content
    .replace(/^\s*\|?[\s:|-]+\|[\s:|-]*$/gm, " ")
    .replace(/[`*_|>#]/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

/** Collapses page, heading and passage hits into one row per docs page, in rank order. */
export function groupHits(hits: readonly SearchHit[]): SearchPageHit[] {
  const pages = new Map<string, SearchPageHit>();
  for (const hit of hits) {
    const url = hit.url.split("#", 1)[0] ?? hit.url;
    if (!url.startsWith("/docs")) continue;
    let page = pages.get(url);
    if (!page) {
      page = { url, title: "", excerpt: null, breadcrumbs: hit.breadcrumbs ?? [] };
      pages.set(url, page);
    }
    if (hit.type === "page") page.title = hit.content;
    else page.excerpt ??= plainExcerpt(hit.content);
  }
  return [...pages.values()].filter(page => page.title !== "");
}

/** The search route as the site serves it: `trailingSlash` would answer `/api/search` with a 308. */
const SEARCH_PATH = "/api/search/";

interface SearchState {
  key: string;
  attempt: number;
  hits: SearchPageHit[];
  failed: boolean;
}

/**
 * Queries the site's search index (the same Orama route that powered the old dialog). Only answers
 * are cached; a failed request shows as `failed` and `retry` asks again.
 */
export function useDocsSearch(query: string, tag: string | null) {
  const needle = query.trim();
  const key = `${needle}\n${tag ?? ""}`;
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<SearchState>({
    key: "",
    attempt: 0,
    hits: [],
    failed: false,
  });
  useEffect(() => {
    if (!needle) return;
    if (cache.has(key)) return;
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      const url = new URL(SEARCH_PATH, window.location.origin);
      url.searchParams.set("query", needle);
      if (tag) url.searchParams.set("tag", tag);
      fetch(url, { signal: controller.signal })
        .then(response => {
          if (!response.ok) throw new Error(`Search answered ${response.status}`);
          return response.json() as Promise<SearchHit[]>;
        })
        .then(hits => {
          const pages = groupHits(hits);
          cache.set(key, pages);
          setState({ key, attempt, hits: pages, failed: false });
        })
        .catch(() => {
          if (!controller.signal.aborted) setState({ key, attempt, hits: [], failed: true });
        });
    }, DELAY_MS);
    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [key, needle, tag, attempt]);
  const retry = () => setAttempt(current => current + 1);
  const cached = cache.get(key);
  if (!needle) return { hits: [], loading: false, failed: false, retry };
  if (cached) return { hits: cached, loading: false, failed: false, retry };
  const settled = state.key === key && state.attempt === attempt;
  // While a new query is in flight the previous results stay up, so the list does not flash empty.
  return { hits: state.hits, loading: !settled, failed: settled && state.failed, retry };
}
