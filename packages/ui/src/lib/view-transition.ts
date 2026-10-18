import { flushSync } from "react-dom";

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
 * Single swap point for React's `<ViewTransition>` once it ships in stable.
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
