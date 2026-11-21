import { type AnyRouter, useElementScrollRestoration } from "@tanstack/react-router";

/**
 * The router restores scroll by an element's DOM path on every navigation, and in the desktop shell
 * every focus change is a navigation: it wrote offsets recorded while a pane was hidden (0), or
 * another pane's offset after a move, into whatever window content sat at that path. Window panes
 * keep their own offsets (`WindowPaneHosts`); only the pages outside the shell keep the router's.
 */
const ROUTES_OUTSIDE_SHELL = new Set(["/design-system"]);

/**
 * Restricts the router's scroll restoration to the pages outside the shell. Call it after
 * `createRouter`, which must not get `scrollRestoration` itself: given it there, the router tracks
 * every scroll in the document and records the offsets on each navigation from construction on,
 * for shell routes that never read them. Set afterwards, the option only decides which renders
 * restore; a page outside the shell starts the tracking with {@link useRouterScrollTracking}.
 */
export function restoreScrollOutsideShell(router: AnyRouter): void {
  router.update({
    ...router.options,
    scrollRestoration: ({ location }) => ROUTES_OUTSIDE_SHELL.has(location.pathname),
  });
}

/**
 * Starts the router's scroll tracking (once per document) for a page that restores by it: every
 * scroll is recorded from here on, saved on each navigation away and restored on the way back.
 */
export function useRouterScrollTracking(): void {
  useElementScrollRestoration({ getElement: () => window });
}
