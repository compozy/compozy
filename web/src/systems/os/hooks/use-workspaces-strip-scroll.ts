import { useEffect, useEffectEvent, useRef, useState } from "react";

/** Which strip edges currently hide overflowed tiles. */
export type WorkspacesStripOverflow = "none" | "start" | "end" | "start end";

/** Scroll deltas under this read as "at the edge" (sub-pixel jitter guard). */
const EDGE_EPSILON = 2;
/** Pointer travel before a press becomes a drag flick. */
const DRAG_THRESHOLD_PX = 6;
/** Free scroll settles after this pause, then focus snaps to the nearest tile. */
const SCROLL_SETTLE_MS = 90;
/** Programmatic smooth scrolls suppress settle-snapping until they finish. */
const FOCUS_SCROLL_LOCK_MS = 320;
const FOCUS_SCROLL_LOCK_INSTANT_MS = 40;

export interface UseWorkspacesStripScrollInput {
  /** Wheel scrubbing binds to the whole overlay, not just the track. */
  overlayRef: React.RefObject<HTMLElement | null>;
  reducedMotion: boolean;
  /** Overflow repaints when the tile population changes. */
  itemCount: number;
  /** Free scroll or a drag flick settled — focus the nearest tile. */
  onSettleFocus: (index: number) => void;
}

export interface WorkspacesStripScroll {
  overflow: WorkspacesStripOverflow;
  dragging: boolean;
  trackRef: React.RefObject<HTMLDivElement | null>;
  trackProps: {
    onScroll: () => void;
    onPointerDown: (event: React.PointerEvent<HTMLDivElement>) => void;
    onPointerMove: (event: React.PointerEvent<HTMLDivElement>) => void;
    onPointerUp: (event: React.PointerEvent<HTMLDivElement>) => void;
    onPointerCancel: (event: React.PointerEvent<HTMLDivElement>) => void;
  };
  /** Scrolls a tile clear of the edge fades; instant on wrap jumps. */
  keepInView: (element: HTMLElement, options?: { instant?: boolean }) => void;
  /** True exactly once after a drag flick, so the trailing click is swallowed. */
  consumeDragClick: () => boolean;
  /** Live drag flag for hover-focus suppression (reads outside render). */
  isTrackDragging: () => boolean;
}

interface DragState {
  pointerId: number;
  startX: number;
  startScrollLeft: number;
  moved: boolean;
}

function tileElements(track: HTMLElement): HTMLElement[] {
  return Array.from(track.querySelectorAll<HTMLElement>('[data-slot="os-workspace-tile"]'));
}

/**
 * The strip's physical model: overflow edge state, wheel scrubbing over the
 * whole overlay, drag flicks with pointer capture, settle-snapping, and
 * keep-in-view padding — the workspaces.css track contract.
 */
export function useWorkspacesStripScroll({
  overlayRef,
  reducedMotion,
  itemCount,
  onSettleFocus,
}: UseWorkspacesStripScrollInput): WorkspacesStripScroll {
  const trackRef = useRef<HTMLDivElement | null>(null);
  const [overflow, setOverflow] = useState<WorkspacesStripOverflow>("none");
  const [dragging, setDragging] = useState(false);

  const overflowRef = useRef<WorkspacesStripOverflow>("none");
  const dragRef = useRef<DragState | null>(null);
  const draggingRef = useRef(false);
  const dragClickRef = useRef(false);
  const scrollingFromFocusRef = useRef(false);
  const settleTimerRef = useRef(0);
  const focusLockTimerRef = useRef(0);
  const onSettleFocusRef = useRef(onSettleFocus);

  useEffect(() => {
    onSettleFocusRef.current = onSettleFocus;
  }, [onSettleFocus]);

  const paintOverflow = () => {
    const track = trackRef.current;
    if (!track) return;
    const max = track.scrollWidth - track.clientWidth;
    let next: WorkspacesStripOverflow = "none";
    if (max > EDGE_EPSILON) {
      const start = track.scrollLeft > EDGE_EPSILON;
      const end = track.scrollLeft < max - EDGE_EPSILON;
      next = start && end ? "start end" : start ? "start" : end ? "end" : "none";
    }
    if (overflowRef.current !== next) {
      overflowRef.current = next;
      setOverflow(next);
    }
  };
  const paintOverflowEvent = useEffectEvent(paintOverflow);

  const nearestIndex = (): number => {
    const track = trackRef.current;
    if (!track) return 0;
    const box = track.getBoundingClientRect();
    const mid = box.left + box.width / 2;
    let best = 0;
    let bestDistance = Number.POSITIVE_INFINITY;
    tileElements(track).forEach((element, index) => {
      const rect = element.getBoundingClientRect();
      const distance = Math.abs(rect.left + rect.width / 2 - mid);
      if (distance < bestDistance) {
        bestDistance = distance;
        best = index;
      }
    });
    return best;
  };

  const settleFocus = () => onSettleFocusRef.current(nearestIndex());
  const scheduleSettleFocus = () => {
    window.clearTimeout(settleTimerRef.current);
    settleTimerRef.current = window.setTimeout(() => settleFocus(), SCROLL_SETTLE_MS);
  };

  const keepInView = (element: HTMLElement, options: { instant?: boolean } = {}) => {
    const track = trackRef.current;
    if (!track) return;
    const rect = element.getBoundingClientRect();
    const trackRect = track.getBoundingClientRect();
    const edge = track.parentElement?.querySelector<HTMLElement>(
      '[data-slot="os-workspaces-strip-edge"]'
    );
    const keepInViewPad = edge?.getBoundingClientRect().width ?? 0;
    let delta = 0;
    if (rect.left < trackRect.left + keepInViewPad) {
      delta = rect.left - (trackRect.left + keepInViewPad);
    } else if (rect.right > trackRect.right - keepInViewPad) {
      delta = rect.right - (trackRect.right - keepInViewPad);
    }
    if (Math.abs(delta) < 1) {
      paintOverflow();
      return;
    }
    const instant = Boolean(options.instant) || reducedMotion;
    scrollingFromFocusRef.current = true;
    track.scrollTo({ left: track.scrollLeft + delta, behavior: instant ? "auto" : "smooth" });
    window.clearTimeout(focusLockTimerRef.current);
    focusLockTimerRef.current = window.setTimeout(
      () => {
        scrollingFromFocusRef.current = false;
        paintOverflow();
      },
      instant ? FOCUS_SCROLL_LOCK_INSTANT_MS : FOCUS_SCROLL_LOCK_MS
    );
  };

  const onScroll = () => {
    paintOverflow();
    if (scrollingFromFocusRef.current) return;
    scheduleSettleFocus();
  };

  const onPointerDown = (event: React.PointerEvent<HTMLDivElement>) => {
    if (event.button !== 0) return;
    const track = trackRef.current;
    if (!track) return;
    dragRef.current = {
      pointerId: event.pointerId,
      startX: event.clientX,
      startScrollLeft: track.scrollLeft,
      moved: false,
    };
  };

  const onPointerMove = (event: React.PointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current;
    const track = trackRef.current;
    if (!drag || !track || event.pointerId !== drag.pointerId) return;
    const dx = event.clientX - drag.startX;
    if (!drag.moved && Math.abs(dx) < DRAG_THRESHOLD_PX) return;
    if (!drag.moved) {
      drag.moved = true;
      draggingRef.current = true;
      setDragging(true);
      try {
        track.setPointerCapture(event.pointerId);
      } catch (error) {
        // Capture can fail when the pointer already ended (e.g. synthetic
        // events); the drag still works from the move stream we do receive.
        console.warn("workspaces strip pointer capture skipped", error);
      }
    }
    track.scrollLeft = drag.startScrollLeft - dx;
  };

  const endDrag = (event: React.PointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current;
    if (!drag || event.pointerId !== drag.pointerId) return;
    const moved = drag.moved;
    dragRef.current = null;
    draggingRef.current = false;
    setDragging(false);
    if (!moved) return;
    window.clearTimeout(settleTimerRef.current);
    dragClickRef.current = true;
    requestAnimationFrame(() => {
      dragClickRef.current = false;
    });
    settleFocus();
  };

  // Wheel scrubbing must preventDefault to keep the page still, and React
  // registers wheel listeners passively — so the overlay listener binds here.
  useEffect(() => {
    const overlay = overlayRef.current;
    const track = trackRef.current;
    if (!overlay || !track) return undefined;
    const onWheel = (event: WheelEvent) => {
      if (overflowRef.current === "none") return;
      if (!(event.target instanceof Element)) return;
      if (event.target.closest('[data-slot="os-workspaces-below"]')) return;
      const dominant =
        Math.abs(event.deltaX) > Math.abs(event.deltaY) ? event.deltaX : event.deltaY;
      if (dominant === 0) return;
      event.preventDefault();
      window.clearTimeout(focusLockTimerRef.current);
      scrollingFromFocusRef.current = false;
      track.scrollLeft += dominant;
      paintOverflow();
      scheduleSettleFocus();
    };
    overlay.addEventListener("wheel", onWheel, { passive: false });
    return () => overlay.removeEventListener("wheel", onWheel);
  }, [itemCount, overlayRef]);

  // Overflow tracks viewport resizes and tile population changes; the double
  // rAF waits out the open animation's first layout.
  useEffect(() => {
    const raf = requestAnimationFrame(() => requestAnimationFrame(() => paintOverflowEvent()));
    const onResize = () => paintOverflowEvent();
    window.addEventListener("resize", onResize);
    return () => {
      cancelAnimationFrame(raf);
      window.removeEventListener("resize", onResize);
    };
  }, [itemCount]);

  // A live row can change width without changing the number of tiles.
  useEffect(() => {
    paintOverflowEvent();
  });

  useEffect(() => {
    return () => {
      window.clearTimeout(settleTimerRef.current);
      window.clearTimeout(focusLockTimerRef.current);
    };
  }, []);

  return {
    overflow,
    dragging,
    trackRef,
    trackProps: {
      onScroll,
      onPointerDown,
      onPointerMove,
      onPointerUp: endDrag,
      onPointerCancel: endDrag,
    },
    keepInView,
    consumeDragClick: () => {
      const consumed = dragClickRef.current;
      dragClickRef.current = false;
      return consumed;
    },
    isTrackDragging: () => draggingRef.current,
  };
}
