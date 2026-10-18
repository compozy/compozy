import { useSyncExternalStore } from "react";

const listeners = new Set<() => void>();
let observer: MutationObserver | null = null;

function notifyThemeChange(): void {
  for (const listener of listeners) listener();
}

// A theme switch lands as a `data-theme` / class change on <html>; one shared
// observer serves every subscriber.
function subscribe(listener: () => void): () => void {
  if (typeof document === "undefined" || typeof MutationObserver === "undefined") {
    return () => undefined;
  }
  listeners.add(listener);
  if (!observer) {
    observer = new MutationObserver(notifyThemeChange);
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme", "class", "style"],
    });
  }
  return () => {
    listeners.delete(listener);
    if (listeners.size === 0) {
      observer?.disconnect();
      observer = null;
    }
  };
}

/** The computed value of a root custom property in the active theme ("" when unset). */
export function useComputedToken(token: string): string {
  return useSyncExternalStore(
    subscribe,
    () =>
      typeof document === "undefined"
        ? ""
        : getComputedStyle(document.documentElement).getPropertyValue(token).trim(),
    () => ""
  );
}
