import { useSyncExternalStore } from "react";

export const WIDE_BROWSER_MIN_WIDTH = 1024;

function wideQuery(): MediaQueryList | null {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") return null;
  return window.matchMedia(`(min-width: ${WIDE_BROWSER_MIN_WIDTH}px)`);
}

function subscribe(callback: () => void): () => void {
  const mql = wideQuery();
  if (!mql) return () => {};
  mql.addEventListener("change", callback);
  return () => mql.removeEventListener("change", callback);
}

function getSnapshot(): boolean {
  const mql = wideQuery();
  if (mql) return mql.matches;
  if (typeof window !== "undefined") return window.innerWidth >= WIDE_BROWSER_MIN_WIDTH;
  return true;
}

function getServerSnapshot(): boolean {
  return true;
}

/** True when the browser viewport is wide enough (`>= 1024px`) for the desktop web shell. */
export function useIsWideBrowser(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}
