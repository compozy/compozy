"use client";

import { useSyncExternalStore } from "react";

const REDUCED_MOTION_QUERY = "(prefers-reduced-motion: reduce)";

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

function getReducedMotionQuery(): MediaQueryList | null {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return null;
  }
  return window.matchMedia(REDUCED_MOTION_QUERY);
}

function subscribe(onStoreChange: () => void): () => void {
  const query = getReducedMotionQuery();
  if (!query) return () => {};
  return subscribeToReducedMotion(query, onStoreChange);
}

function getSnapshot(): boolean {
  return getReducedMotionQuery()?.matches ?? false;
}

function getServerSnapshot(): boolean {
  return false;
}

/**
 * Returns true when the user has requested reduced motion via the OS.
 * SSR-safe: defaults to false server-side and on first client render.
 */
export function useReducedMotion(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}
