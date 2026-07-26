"use client";

import { useEffect, useState } from "react";

function subscribeToReducedMotion(
  query: MediaQueryList,
  listener: (event: MediaQueryListEvent) => void
): () => void {
  if (typeof query.addEventListener === "function") {
    query.addEventListener("change", listener);
    return () => query.removeEventListener("change", listener);
  }
  const legacyQuery = query as MediaQueryList & {
    addListener?: (handler: (event: MediaQueryListEvent) => void) => void;
    removeListener?: (handler: (event: MediaQueryListEvent) => void) => void;
  };
  legacyQuery.addListener?.(listener);
  return () => legacyQuery.removeListener?.(listener);
}

/**
 * Returns true when the user has requested reduced motion via the OS.
 * SSR-safe: defaults to false server-side and on first client render.
 */
export function useReducedMotion(): boolean {
  const [reduced, setReduced] = useState(false);

  useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
      return;
    }
    const query = window.matchMedia("(prefers-reduced-motion: reduce)");
    setReduced(query.matches);
    const handler = (event: MediaQueryListEvent) => setReduced(event.matches);
    return subscribeToReducedMotion(query, handler);
  }, []);

  return reduced;
}
