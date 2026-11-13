import { observeElementOffset, type VirtualItem, type Virtualizer } from "@tanstack/react-virtual";
import { type RefObject, useEffect } from "react";

import { isRenderedElement, SCROLL_RESTORE_EVENT } from "@/lib/scroll-owner";

import type { ThreadScrollStore } from "./thread-scroll-context";
import { threadScrollMode } from "./thread-scroll-store";
import { pinToEnd } from "./use-thread-scroll-gestures";

/** The row at the viewport's top edge and how far into it the reader is. */
interface ReadingAnchor {
  key: VirtualItem["key"];
  offset: number;
}

/**
 * The virtualizer's offset, except while the viewport is not rendered: a hidden scroller reports
 * 0, and taking that as the reader's offset would move the rendered range to the top and unmount
 * the rows the reader was on.
 */
export function observeRenderedElementOffset(
  instance: Virtualizer<HTMLDivElement, HTMLDivElement>,
  publish: (offset: number, isScrolling: boolean) => void
): (() => void) | undefined {
  return observeElementOffset(instance, (offset, isScrolling) => {
    const viewport = instance.scrollElement;
    if (viewport !== null && !isRenderedElement(viewport)) return;
    publish(offset, isScrolling);
  });
}

/**
 * Where the transcript resumes when its pane is rendered again (after a tab switch, a move into
 * a split, a collapsed switcher). Nothing about the transcript changes while it is not rendered,
 * so it resumes from the state it had: following pins the end again; otherwise the row the
 * reader was on returns to the same distance under the top edge, read from the virtualizer's
 * measurements rather than as a pixel offset, so a width change that reflowed the rows above
 * keeps the same message in view. The pane host announces the show with
 * {@link SCROLL_RESTORE_EVENT} before the frame paints, and leaves this viewport's offsets to it.
 */
export function useThreadScrollResume(
  viewportRef: RefObject<HTMLDivElement | null>,
  virtualizer: Virtualizer<HTMLDivElement, HTMLDivElement>,
  store: ThreadScrollStore
): void {
  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) return undefined;
    let anchor: ReadingAnchor | null = null;
    const read = () => {
      if (!isRenderedElement(viewport)) return;
      const item = virtualizer.getVirtualItemForOffset(viewport.scrollTop);
      anchor =
        item === undefined ? null : { key: item.key, offset: viewport.scrollTop - item.start };
    };
    const resume = () => {
      if (!isRenderedElement(viewport)) return;
      if (threadScrollMode(store.getSnapshot().context) === "following-end") {
        pinToEnd(viewport, store);
        return;
      }
      const reading = anchor;
      if (reading === null) return;
      const item = virtualizer.measurementsCache.find(entry => entry.key === reading.key);
      if (item === undefined) return;
      const top = item.start + Math.min(reading.offset, item.size);
      if (Math.abs(viewport.scrollTop - top) < 1) return;
      viewport.scrollTop = top;
      store.trigger.programmaticScrollApplied({ scrollTop: viewport.scrollTop });
    };
    read();
    viewport.addEventListener("scroll", read, { passive: true });
    viewport.addEventListener(SCROLL_RESTORE_EVENT, resume);
    return () => {
      viewport.removeEventListener("scroll", read);
      viewport.removeEventListener(SCROLL_RESTORE_EVENT, resume);
    };
  }, [store, viewportRef, virtualizer]);
}
