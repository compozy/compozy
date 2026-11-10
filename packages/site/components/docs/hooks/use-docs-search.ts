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

/** Queries the site's search index (the same Orama route that powered the old dialog). */
export function useDocsSearch(query: string, tag: string | null) {
  const needle = query.trim();
  const key = `${needle}\n${tag ?? ""}`;
  const [state, setState] = useState<{ key: string; hits: SearchPageHit[] }>({
    key: "",
    hits: [],
  });
  useEffect(() => {
    if (!needle) return;
    if (cache.has(key)) return;
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      const url = new URL("/api/search", window.location.origin);
      url.searchParams.set("query", needle);
      if (tag) url.searchParams.set("tag", tag);
      fetch(url, { signal: controller.signal })
        .then(response => (response.ok ? response.json() : []))
        .then((hits: SearchHit[]) => {
          const pages = groupHits(hits);
          cache.set(key, pages);
          setState({ key, hits: pages });
        })
        .catch(() => {
          if (!controller.signal.aborted) setState({ key, hits: [] });
        });
    }, DELAY_MS);
    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [key, needle, tag]);
  const cached = cache.get(key);
  if (!needle) return { hits: [], loading: false };
  // While a new query is in flight the previous results stay up, so the list does not flash empty.
  return { hits: cached ?? state.hits, loading: !cached && state.key !== key };
}
