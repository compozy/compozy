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
  /** The reader's depth into the row as a share of its height, so a reflow keeps the same text. */
  fraction: number;
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
 * returns to the same depth under the top edge. That depth is kept as a share of the row's
 * height, read from the virtualizer's measurements, so a width change that reflowed the rows
 * keeps the same message and the same text in view. The rows re-measure over the next frames
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
    const release = () => {
      holding = false;
      cancelAnimationFrame(frame);
    };
    const read = () => {
      // While the row is held, the scrolls are the hold's own writes, not the reader's.
      if (holding || !isRenderedElement(viewport)) return;
      const top = viewport.scrollTop;
      const item = virtualizer.getVirtualItemForOffset(top);
      if (item === undefined) {
        anchor = null;
        return;
      }
      const depth = item.size > 0 ? (top - item.start) / item.size : 0;
      anchor = { key: item.key, fraction: Math.min(1, Math.max(0, depth)), top };
    };
    /** Where the anchored row puts the viewport now, by its current measurement. */
    const targetOf = (reading: ReadingAnchor): number => {
      const item = virtualizer.measurementsCache.find(entry => entry.key === reading.key);
      if (item === undefined) return reading.top;
      return item.start + reading.fraction * item.size;
    };
    const place = (reading: ReadingAnchor): boolean => {
      const top = targetOf(reading);
      if (Math.abs(viewport.scrollTop - top) < 1) return false;
      viewport.scrollTop = top;
      store.trigger.programmaticScrollApplied({ scrollTop: viewport.scrollTop });
      return true;
    };
    const hold = (reading: ReadingAnchor) => {
      release();
      holding = true;
      const mode = threadScrollMode(store.getSnapshot().context);
      let frames = 0;
      let stable = 0;
      const tick = () => {
        frames += 1;
        if (!isRenderedElement(viewport)) return release();
        // A new turn or a jump to the end owns the viewport from here on.
        if (threadScrollMode(store.getSnapshot().context) !== mode) return release();
        stable = place(reading) ? 0 : stable + 1;
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
      if (mode === "anchoring-new-turn" && placeTurn.current(viewport)) return;
      const reading = anchor;
      if (reading === null) return;
      place(reading);
      hold(reading);
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
