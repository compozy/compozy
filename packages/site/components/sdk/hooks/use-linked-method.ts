"use client";

/**
 * The method the URL hash names on a resource page. A link to a method past the preview (from search,
 * another page, or the method index) has no target until the list renders it, so the page shows every
 * method and scrolls to it once it exists.
 */
import { useEffect, useSyncExternalStore } from "react";

function subscribe(listener: () => void): () => void {
  window.addEventListener("hashchange", listener);
  return () => window.removeEventListener("hashchange", listener);
}

function readHash(): string {
  const hash = window.location.hash.slice(1);
  try {
    return decodeURIComponent(hash);
  } catch {
    // A malformed escape names no method.
    return hash;
  }
}

export function useLinkedMethod(ids: readonly string[], preview: number): boolean {
  const hash = useSyncExternalStore(subscribe, readHash, () => "");
  const beyond = ids.indexOf(hash) >= preview;
  useEffect(() => {
    if (beyond) document.getElementById(hash)?.scrollIntoView();
  }, [beyond, hash]);
  return beyond;
}
