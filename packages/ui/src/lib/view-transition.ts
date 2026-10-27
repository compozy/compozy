import { addTransitionType, startTransition } from "react";
import { flushSync } from "react-dom";

// In-window animation goes through React's `<ViewTransition>`, with the vocabulary below.
// `runViewTransition` further down predates it and stays only for the window manager's own
// window-body swaps.
//
// Perf rules for every boundary you add:
// - Start from `default="none"` and opt in to exactly the triggers you want (`enter`, `exit`,
//   `update`, `share`). A bare `<ViewTransition>` defaults to "auto", so every unrelated
//   transition that touches its subtree (a TanStack Router navigation, any other
//   `startTransition`) would snapshot and cross-fade it.
// - Never wrap high-frequency surfaces: the streaming transcript, the terminal, drag and resize,
//   window geometry. Every element a transition names is captured as an old and a new image for
//   its whole duration, which is costly for large or constantly changing surfaces. That includes
//   anything given a permanent `viewTransitionName`: it is captured by any activated boundary.
// - A boundary only animates for updates React treats as transitions (`startTransition`,
//   `startTypedTransition`, a Suspense reveal, `useDeferredValue`); urgent updates never do.
// - Enter and exit only fire when the `<ViewTransition>` is the first thing the mounting or
//   unmounting component returns, before any DOM node.

/**
 * Transition types, passed to {@link startTypedTransition} so a boundary can pick its class by
 * cause. Browsers also expose them as `:active-view-transition-type(...)`.
 */
export const VIEW_TRANSITION_TYPE = {
  /** Going deeper: a list opens a detail view. */
  drillIn: "drill-in",
  /** Coming back: a detail view returns to its list. */
  drillOut: "drill-out",
} as const;

/**
 * The `view-transition-class` values defined in `tokens-runtime.css`, for a `<ViewTransition>`
 * trigger prop (`enter`, `exit`, `update`, `share` or `default`). Each class animates both the
 * old and the new snapshot, with opacity and transform only, and reduced motion cuts all of
 * them, so any class works on any trigger. Pair with `default="none"` (see the rules above).
 */
export const VIEW_TRANSITION_CLASS = {
  /** Cross-fade for a content swap: a segment or tab panel, a Suspense reveal (`update`). */
  fade: "vt-fade",
  /** The arriving view slides in from the end while the departing one fades (list to detail). */
  drillIn: "vt-drill-in",
  /** The arriving view slides in from the start while the departing one fades (detail to list). */
  drillOut: "vt-drill-out",
  /** A collection item rises into place as it appears (`enter`). */
  itemEnter: "vt-item-enter",
  /** A collection item sinks away as it leaves (`exit`). */
  itemExit: "vt-item-exit",
} as const;

/**
 * A per-type class map for a boundary that swaps a list and its detail: `drill-in` slides the
 * arriving view in from the end, `drill-out` from the start, and any other cause does not
 * animate. Give it to `enter` and `exit` of keyed boundaries, or to `update` of a persistent one:
 *
 * ```tsx
 * <ViewTransition key={view} default="none" enter={VIEW_TRANSITION_DRILL} exit={VIEW_TRANSITION_DRILL}>
 *   {view === "detail" ? <Detail /> : <List />}
 * </ViewTransition>
 * startTypedTransition(VIEW_TRANSITION_TYPE.drillIn, () => setView("detail"));
 * ```
 */
export const VIEW_TRANSITION_DRILL = {
  [VIEW_TRANSITION_TYPE.drillIn]: VIEW_TRANSITION_CLASS.drillIn,
  [VIEW_TRANSITION_TYPE.drillOut]: VIEW_TRANSITION_CLASS.drillOut,
  default: "none",
} as const;

/**
 * Runs `update` as a React transition tagged with `type`, so `<ViewTransition>` boundaries
 * can animate by cause (see {@link VIEW_TRANSITION_DRILL}). Types reset after each commit, and a
 * Suspense reveal that follows does not carry them. Call it from the event that causes the
 * change; the update must set state synchronously (a type added after an `await` is lost).
 * The perf rules above apply to the boundaries that listen.
 */
export function startTypedTransition(type: string, update: () => void): void {
  startTransition(() => {
    addTransitionType(type);
    update();
  });
}

export interface RunViewTransitionOptions {
  /** View-transition types matched by `:active-view-transition-type(...)` rules. */
  types?: readonly string[];
  /** Skip the animation (in-product reduce-motion flag); the update still runs. */
  reduced?: boolean;
}

function prefersReducedMotion(): boolean {
  return (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

function supportsTransitionTypes(): boolean {
  return (
    typeof CSS !== "undefined" &&
    typeof CSS.supports === "function" &&
    CSS.supports("selector(:active-view-transition-type(a))")
  );
}

/**
 * Runs `update` inside a same-document view transition when the engine supports
 * it and motion is allowed. Without a transition `update` runs synchronously;
 * with one, the engine calls it after capturing the old frame (next frame) and
 * React commits it via `flushSync`. Callers that need synchronous outcomes must
 * compute them outside `update`. The returned promise settles once `update` ran.
 *
 * The pre-React path: it serves the window manager's window-body drill (the `os-window-body`
 * rules in `tokens-runtime.css`) and the callers that still hand-name an element with
 * `viewTransitionName`. New in-window UI uses React's `<ViewTransition>` with
 * `VIEW_TRANSITION_CLASS` and `startTypedTransition` instead.
 */
export function runViewTransition(
  update: () => void,
  options: RunViewTransitionOptions = {}
): Promise<void> {
  if (
    typeof document === "undefined" ||
    typeof document.startViewTransition !== "function" ||
    options.reduced ||
    prefersReducedMotion()
  ) {
    update();
    return Promise.resolve();
  }
  const types = options.types && options.types.length > 0 ? [...options.types] : undefined;
  // flushSync stays lexically inside startViewTransition: React must commit the
  // new DOM before the engine captures the "new" snapshot.
  const transition =
    types && supportsTransitionTypes()
      ? document.startViewTransition({ update: () => flushSync(update), types })
      : document.startViewTransition(() => flushSync(update));
  // Skipped/aborted transitions (duplicate names, rapid re-entry) still ran the update.
  transition.finished.catch(() => undefined);
  transition.ready.catch(() => undefined);
  return transition.updateCallbackDone.catch(() => undefined);
}

/** Builds a valid, document-unique `view-transition-name` custom ident. */
export function viewTransitionName(...parts: readonly (string | number)[]): string {
  const segments: string[] = [];
  for (const part of parts) {
    const segment = String(part).replace(/[^A-Za-z0-9_-]+/g, "_");
    if (segment.length > 0) segments.push(segment);
  }
  return `vt-${segments.join("-")}`;
}
