import { useEffect } from "react";
import { useSelector, useStore } from "@xstate/store-react";

import { useDocumentVisible } from "@/hooks/use-document-visible";

import { isAppendOnlyUpdate } from "../lib/session-smooth-reveal";
import { smoothStreamedTextLogic } from "./smooth-streamed-text-store";

/**
 * Smooth reveal of streamed prose (ADR-008). While `animate` holds, the
 * returned text trails the real text at a cadence that tracks arrival and
 * snaps to the full text the moment the stream settles or the text is
 * rewritten. When `animate` is false (stream done, reduced motion, the
 * client-local toggle off, no animation frames available) the real text is
 * returned as-is — presentation only, never a delay of finished content.
 * A hidden document has nobody to animate for: the clock stops and the text
 * snaps, so returning to the window shows the final text at once.
 */
export function useSmoothStreamedText(text: string, animate: boolean): string {
  const documentVisible = useDocumentVisible();
  const active = animate && documentVisible && typeof requestAnimationFrame === "function";
  const store = useStore(smoothStreamedTextLogic, { text });
  const emittedCount = useSelector(store, snapshot => snapshot.context.emittedCount);
  const target = useSelector(store, snapshot => snapshot.context.target);

  useEffect(() => {
    store.trigger.textObserved({ text, active });
  }, [active, store, text]);

  useEffect(() => () => store.trigger.disposed(), [store]);

  // A rewrite or settled stream is visible in the same render, before the clock observes it.
  return active && isAppendOnlyUpdate(target, text) ? text.slice(0, emittedCount) : text;
}
