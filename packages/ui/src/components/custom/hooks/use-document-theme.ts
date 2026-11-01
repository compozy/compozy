import { useSyncExternalStore } from "react";

function subscribe(listener: () => void) {
  const observer = new MutationObserver(listener);
  observer.observe(document.documentElement, { attributeFilter: ["data-theme"], attributes: true });
  return () => observer.disconnect();
}

function snapshot(): "dark" | "light" {
  return document.documentElement.dataset.theme === "light" ? "light" : "dark";
}

/** The theme the document painted, for values that can't be picked from CSS alone. */
export function useDocumentTheme() {
  return useSyncExternalStore(subscribe, snapshot, () => "dark" as const);
}
