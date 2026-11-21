import { useElementScrollRestoration } from "@tanstack/react-router";

/**
 * Starts the router's scroll tracking (once per document) for a page that restores by it (`restoreScrollOutsideShell`): every
 * scroll is recorded from here on, saved on each navigation away and restored on the way back.
 */
export function useRouterScrollTracking(): void {
  useElementScrollRestoration({ getElement: () => window });
}
