"use client";

/**
 * The method the URL hash names on a resource page. A method past the preview waits in a closed
 * `<details>`; a link to it (from search, another page, or the method index) opens the full list and
 * scrolls to the method once it is shown.
 */
import { useEffect, useSyncExternalStore } from "react";

import { fragmentId } from "@/lib/url-fragment";

function subscribe(listener: () => void): () => void {
  window.addEventListener("hashchange", listener);
  return () => window.removeEventListener("hashchange", listener);
}

function readHash(): string {
  return fragmentId(window.location.hash);
}

export function useLinkedMethod(ids: readonly string[], preview: number): boolean {
  const hash = useSyncExternalStore(subscribe, readHash, () => "");
  const beyond = ids.indexOf(hash) >= preview;
  useEffect(() => {
    if (beyond) document.getElementById(hash)?.scrollIntoView();
  }, [beyond, hash]);
  return beyond;
}
