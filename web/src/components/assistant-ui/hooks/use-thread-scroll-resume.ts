import { observeElementOffset, type VirtualItem, type Virtualizer } from "@tanstack/react-virtual";
import { type RefObject, useEffect, useRef } from "react";

import { isRenderedElement, SCROLL_RESTORE_EVENT } from "@/lib/scroll-owner";

import type { ThreadScrollStore } from "./thread-scroll-context";
import { threadScrollMode } from "./thread-scroll-store";
import { pinToEnd } from "./use-thread-scroll-gestures";

/** Frames the resumed row is held in place while the rows re-measure after a show. */
const RESUME_MAX_FRAMES = 60;
/** Consecutive frames the resumed row must rest before the hold ends early. */
const RESUME_STABLE_FRAMES = 4;

/** The row at the viewport's top edge, how far into it the reader is, and the raw offset. */
interface ReadingAnchor {
  key: VirtualItem["key"];
  /** The reader's depth into the row in pixels: kept when the row only grew (a reply streamed). */
  depth: number;
  /** The same depth as a share of the row's height: kept when a width change reflowed the row. */
  fraction: number;
  /** The viewport's width when read. */
  width: number;
  /** The viewport's offset when read: the place when the row left the list while hidden. */
  top: number;
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
 * a split, a collapsed switcher). Following pins the end again; a turn that started while the
 * pane was hidden gets its new-turn anchor (`placeTurn`); otherwise the row the reader was on
 * returns to the same depth under the top edge, read from the virtualizer's measurements. At
 * the same width that depth is kept in pixels, so a reply that grew while hidden does not move
 * the text the reader was on; after a width change it is kept as a share of the row's height,
 * so the reflowed row keeps the same text in view. The rows re-measure over the next frames
 * (a new width, rows that mounted on show) and the virtualizer compensates for them, so the
 * row is held there until it rests, for at most {@link RESUME_MAX_FRAMES}, or until the reader's
 * gesture, a mode change, or the controller's own move (a message jump, an older page landing:
 * the returned `endHold`). A row that left the list while hidden falls back to the raw offset.
 * The pane host announces the show with {@link SCROLL_RESTORE_EVENT} before the frame paints,
 * and leaves this viewport's offsets to it.
 */
export function useThreadScrollResume(
  viewportRef: RefObject<HTMLDivElement | null>,
  virtualizer: Virtualizer<HTMLDivElement, HTMLDivElement>,
  store: ThreadScrollStore,
  placeTurn: RefObject<(viewport: HTMLDivElement) => boolean>
): RefObject<() => void> {
  const endHold = useRef<() => void>(() => undefined);
  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) return undefined;
    let anchor: ReadingAnchor | null = null;
    let frame = 0;
    let holding = false;
    /** The offset and content size the hold last left: a move off it with no growth is not ours. */
    let held = { top: 0, size: 0 };
    const release = () => {
      holding = false;
      cancelAnimationFrame(frame);
    };
    const read = () => {
      if (!isRenderedElement(viewport)) return;
      if (holding) {
        // The hold's own writes and the virtualizer's compensation for re-measured rows are not
        // the reader's; any other move (autoscroll, assistive tech, a scrollbar drag) is.
        const moved = Math.abs(viewport.scrollTop - held.top) >= 1;
        if (!moved || virtualizer.getTotalSize() !== held.size) return;
        release();
      }
      const top = viewport.scrollTop;
      const item = virtualizer.getVirtualItemForOffset(top);
      if (item === undefined) {
        anchor = null;
        return;
      }
      const depth = Math.max(0, top - item.start);
      const fraction = item.size > 0 ? Math.min(1, depth / item.size) : 0;
      anchor = { key: item.key, depth, fraction, width: viewport.clientWidth, top };
    };
    /** Where the anchored row puts the viewport now, by its current measurement. */
    const targetOf = (reading: ReadingAnchor): number => {
      const item = virtualizer.measurementsCache.find(entry => entry.key === reading.key);
      if (item === undefined) return reading.top;
      // Same width: the text above the reader did not move, whatever was appended below it.
      if (Math.abs(viewport.clientWidth - reading.width) < 1) {
        return item.start + Math.min(reading.depth, item.size);
      }
      return item.start + reading.fraction * item.size;
    };
    const place = (reading: ReadingAnchor): boolean => {
      const top = targetOf(reading);
      if (Math.abs(viewport.scrollTop - top) < 1) return false;
      viewport.scrollTop = top;
      store.trigger.programmaticScrollApplied({ scrollTop: viewport.scrollTop });
      return true;
    };
    const settle = () => {
      held = { top: viewport.scrollTop, size: virtualizer.getTotalSize() };
    };
    /** Re-applies `step` each frame until the viewport rests; `step` places it. */
    const hold = (step: () => void) => {
      release();
      holding = true;
      settle();
      const mode = threadScrollMode(store.getSnapshot().context);
      let frames = 0;
      let stable = 0;
      const tick = () => {
        frames += 1;
        if (!isRenderedElement(viewport)) return release();
        // A new turn or a jump to the end owns the viewport from here on.
        if (threadScrollMode(store.getSnapshot().context) !== mode) return release();
        const before = viewport.scrollTop;
        step();
        settle();
        stable = Math.abs(viewport.scrollTop - before) >= 1 ? 0 : stable + 1;
        if (stable >= RESUME_STABLE_FRAMES || frames >= RESUME_MAX_FRAMES) return release();
        frame = requestAnimationFrame(tick);
      };
      frame = requestAnimationFrame(tick);
    };
    const resume = () => {
      if (!isRenderedElement(viewport)) return;
      release();
      const mode = threadScrollMode(store.getSnapshot().context);
      if (mode === "following-end") {
        pinToEnd(viewport, store);
        return;
      }
      // A turn that started while hidden is placed, and held while its rows re-measure.
      if (mode === "anchoring-new-turn" && placeTurn.current(viewport)) {
        hold(() => void placeTurn.current(viewport));
        return;
      }
      const reading = anchor;
      if (reading === null) return;
      place(reading);
      hold(() => void place(reading));
    };
    endHold.current = release;
    read();
    viewport.addEventListener("scroll", read, { passive: true });
    viewport.addEventListener(SCROLL_RESTORE_EVENT, resume);
    // The reader's input ends a hold still settling.
    viewport.addEventListener("wheel", release, { passive: true });
    viewport.addEventListener("pointerdown", release, { passive: true });
    viewport.addEventListener("keydown", release, { passive: true });
    viewport.addEventListener("touchstart", release, { passive: true });
    return () => {
      release();
      endHold.current = () => undefined;
      viewport.removeEventListener("scroll", read);
      viewport.removeEventListener(SCROLL_RESTORE_EVENT, resume);
      viewport.removeEventListener("wheel", release);
      viewport.removeEventListener("pointerdown", release);
      viewport.removeEventListener("keydown", release);
      viewport.removeEventListener("touchstart", release);
    };
  }, [placeTurn, store, viewportRef, virtualizer]);
  return endHold;
}
