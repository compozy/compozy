import { useSyncExternalStore } from "react";

const REDUCED_MOTION_QUERY = "(prefers-reduced-motion: reduce)";

function subscribe(callback: () => void): () => void {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return () => undefined;
  }
  const mql = window.matchMedia(REDUCED_MOTION_QUERY);
  if (typeof mql.addEventListener === "function") {
    mql.addEventListener("change", callback);
    return () => mql.removeEventListener("change", callback);
  }
  // Older browsers (jsdom variants) fall back to the deprecated listener API.
  mql.addListener(callback);
  return () => mql.removeListener(callback);
}

function getMatches(): boolean {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return false;
  }
  return window.matchMedia(REDUCED_MOTION_QUERY).matches;
}

/**
 * Tracks the user's `prefers-reduced-motion` setting reactively, mirroring the
 * `useThreadViewMode` matchMedia store. Motion-driven affordances read this to
 * render a static tree instead of an animated one: the tokens.css reduced-motion
 * guard already stills the keyframes, but rendering a different tree keeps the
 * animation classes out of the DOM entirely (task 30 streaming indicator).
 */
export function usePrefersReducedMotion(): boolean {
  return useSyncExternalStore(subscribe, getMatches, () => false);
}
