import { useSyncExternalStore } from "react";

export type ThreadViewMode = "overlay" | "fullpage";

/**
 * Width breakpoint that flips the thread route between right-rail overlay
 * (>=1024px per `_design.md` §3.2) and full-page render (<1024px per §3.3).
 */
export const THREAD_OVERLAY_BREAKPOINT_PX = 1024;

function buildQuery(): string {
  return `(min-width: ${THREAD_OVERLAY_BREAKPOINT_PX}px)`;
}

function subscribeMatchMedia(query: string, callback: () => void): () => void {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return () => undefined;
  }
  const mql = window.matchMedia(query);
  if (typeof mql.addEventListener === "function") {
    mql.addEventListener("change", callback);
    return () => mql.removeEventListener("change", callback);
  }
  // Older browsers (jsdom variants) fall back to the deprecated listener API.
  mql.addListener(callback);
  return () => mql.removeListener(callback);
}

function getMatches(query: string): boolean {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return true;
  }
  return window.matchMedia(query).matches;
}

export function useThreadViewMode(): ThreadViewMode {
  const query = buildQuery();
  const matches = useSyncExternalStore(
    callback => subscribeMatchMedia(query, callback),
    () => getMatches(query),
    () => true
  );
  return matches ? "overlay" : "fullpage";
}
