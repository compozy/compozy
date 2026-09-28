import { flushSync } from "react-dom";

export interface RunViewTransitionOptions {
  /** View-transition types matched by `:active-view-transition-type(...)` rules. */
  types?: readonly string[];
  /** Skip the animation (in-product reduce-motion flag); the update still runs. */
  reduced?: boolean;
}

type TransitionHandle = { updateCallbackDone: Promise<void> };
type StartViewTransition = (
  arg: (() => void) | { update: () => void; types?: string[] }
) => TransitionHandle & { finished: Promise<void>; ready: Promise<void> };

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
  const start =
    typeof document === "undefined"
      ? undefined
      : (document as Document & { startViewTransition?: StartViewTransition }).startViewTransition;
  if (typeof start !== "function" || options.reduced || prefersReducedMotion()) {
    update();
    return Promise.resolve();
  }
  const flushed = () => flushSync(update);
  const types = options.types && options.types.length > 0 ? [...options.types] : undefined;
  const transition =
    types && supportsTransitionTypes()
      ? start.call(document, { update: flushed, types })
      : start.call(document, flushed);
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
