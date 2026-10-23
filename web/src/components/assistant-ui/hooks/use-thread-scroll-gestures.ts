import { type RefObject, useEffect } from "react";

import {
  FOLLOW_REARM_BAND_PX,
  navigationKeyDirection,
  shouldMaintainEnd,
} from "../timeline-scroll-anchoring";
import type { ThreadScrollStore } from "./thread-scroll-context";
import { navigationKeyTarget } from "./thread-scroll-landing";

/** Pins the viewport to the live edge and records the write as ours. */
export function pinToEnd(viewport: HTMLDivElement, store: ThreadScrollStore): void {
  viewport.scrollTop = Math.max(0, viewport.scrollHeight - viewport.clientHeight);
  store.trigger.programmaticScrollApplied({ scrollTop: viewport.scrollTop });
}

/**
 * The reader's side of the scroll owner (ADR-007): wheel, touch, pointer, and
 * navigation keys are the only input that hands the viewport over or takes it
 * back. A scroll event alone never does — measurement and clamping move the
 * viewport too — so while following, a scroll off the end re-pins it. A
 * pointer held on the scroller (scrollbar drag, selection autoscroll) reports
 * the direction its scrolls move, so dragging back down can re-arm following;
 * only a press on the scrollbar gutter takes the viewport before it moves.
 */
export function useThreadScrollGestures(
  viewportRef: RefObject<HTMLDivElement | null>,
  store: ThreadScrollStore
): void {
  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) return undefined;
    const pointerEvents = viewport.ownerDocument;
    const distanceFromBottom = () =>
      viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight;
    const observeNavigation = (direction: "up" | "down") => {
      store.trigger.manualNavigationObserved({
        awayFromEnd: distanceFromBottom() > FOLLOW_REARM_BAND_PX,
        direction,
      });
    };
    let lastTouchY: number | null = null;
    let heldPointerScrollTop: number | null = null;
    const handleWheel = (event: WheelEvent) => {
      observeNavigation(event.deltaY < 0 ? "up" : "down");
    };
    const handleTouchStart = (event: TouchEvent) => {
      lastTouchY = event.touches[0]?.clientY ?? null;
    };
    const handleTouchMove = (event: TouchEvent) => {
      const y = event.touches[0]?.clientY ?? null;
      // A finger moving down drags the content down: the reader is heading up.
      const direction = lastTouchY !== null && y !== null && y > lastTouchY ? "up" : "down";
      lastTouchY = y;
      observeNavigation(direction);
    };
    // A press on the scrollbar gutter is the reader taking the viewport. Any
    // other press (selecting text, a row control, an overlay scrollbar) takes
    // it only once the scrolls it drives move the viewport.
    const handlePointerDown = (event: PointerEvent) => {
      heldPointerScrollTop = viewport.scrollTop;
      const onGutter =
        event.target === viewport &&
        event.clientX - viewport.getBoundingClientRect().left >= viewport.clientWidth;
      if (onGutter) observeNavigation("up");
    };
    const handlePointerRelease = () => {
      heldPointerScrollTop = null;
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      const direction = navigationKeyDirection({
        key: event.key,
        shiftKey: event.shiftKey,
        defaultPrevented: event.defaultPrevented,
        target: navigationKeyTarget(event.target),
      });
      if (direction) observeNavigation(direction);
    };
    const handleScroll = () => {
      if (heldPointerScrollTop !== null && viewport.scrollTop !== heldPointerScrollTop) {
        const direction = viewport.scrollTop > heldPointerScrollTop ? "down" : "up";
        heldPointerScrollTop = viewport.scrollTop;
        observeNavigation(direction);
      }
      store.trigger.scrollObserved({
        atEnd: distanceFromBottom() <= FOLLOW_REARM_BAND_PX,
        scrollTop: viewport.scrollTop,
      });
      // No gesture moved the viewport off the end (estimates replaced by real
      // heights while a long transcript opens): following holds the end.
      if (shouldMaintainEnd(store.getSnapshot().context.ownership) && distanceFromBottom() > 1) {
        pinToEnd(viewport, store);
      }
    };
    viewport.addEventListener("wheel", handleWheel, { passive: true });
    viewport.addEventListener("touchstart", handleTouchStart, { passive: true });
    viewport.addEventListener("touchmove", handleTouchMove, { passive: true });
    viewport.addEventListener("pointerdown", handlePointerDown, { passive: true });
    pointerEvents.addEventListener("pointerup", handlePointerRelease, { passive: true });
    pointerEvents.addEventListener("pointercancel", handlePointerRelease, { passive: true });
    viewport.addEventListener("keydown", handleKeyDown);
    viewport.addEventListener("scroll", handleScroll, { passive: true });
    return () => {
      viewport.removeEventListener("wheel", handleWheel);
      viewport.removeEventListener("touchstart", handleTouchStart);
      viewport.removeEventListener("touchmove", handleTouchMove);
      viewport.removeEventListener("pointerdown", handlePointerDown);
      pointerEvents.removeEventListener("pointerup", handlePointerRelease);
      pointerEvents.removeEventListener("pointercancel", handlePointerRelease);
      viewport.removeEventListener("keydown", handleKeyDown);
      viewport.removeEventListener("scroll", handleScroll);
    };
  }, [store, viewportRef]);
}
