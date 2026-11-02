import { useSyncExternalStore } from "react";

function subscribe(listener: () => void) {
  const observer = new MutationObserver(listener);
  observer.observe(document.documentElement, { attributeFilter: ["data-theme"], attributes: true });
  return () => observer.disconnect();
}

function snapshot(): "dark" | "light" {
  return document.documentElement.dataset.theme === "light" ? "light" : "dark";
}

/**
 * The theme the canvas painted. `proto.canvas.tsx` writes it to the document through `applyTheme`,
 * which never reaches the theme store `useThemePreference` reads, so in the gallery only the
 * document knows it.
 */
export function useCanvasTheme() {
  return useSyncExternalStore(subscribe, snapshot, () => "dark" as const);
}
