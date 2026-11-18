import { useEffect, useRef, useState } from "react";

/** Which rail edges currently hide chips; drives the fades and paging affordances. */
export type RailOverflow = "none" | "start" | "end" | "start end";

/** Sub-pixel scroll jitter is not real overflow. */
const RAIL_EDGE_EPSILON = 2;

function measureRailOverflow(track: HTMLElement): RailOverflow {
  const max = track.scrollWidth - track.clientWidth;
  if (max <= RAIL_EDGE_EPSILON) return "none";
  const start = track.scrollLeft > RAIL_EDGE_EPSILON;
  const end = track.scrollLeft < max - RAIL_EDGE_EPSILON;
  if (start && end) return "start end";
  if (start) return "start";
  if (end) return "end";
  return "none";
}

/**
 * Owns the chip track's overflow state. The native scrollbar is hidden, so the
 * edge fades and chevrons are the only cue that more providers exist.
 */
export function useRailOverflow(chipCount: number) {
  const trackRef = useRef<HTMLDivElement>(null);
  const [overflow, setOverflow] = useState<RailOverflow>("none");

  useEffect(() => {
    const track = trackRef.current;
    if (!track) return;
    const controller = new AbortController();
    const { signal } = controller;
    const paint = () => setOverflow(measureRailOverflow(track));

    paint();
    const frame = requestAnimationFrame(paint);
    track.addEventListener("scroll", paint, { passive: true, signal });
    // Arrow-key navigation focuses a chip; the browser scrolls it into view.
    track.addEventListener("focusin", paint, { signal });
    window.addEventListener("resize", paint, { signal });
    const observer = typeof ResizeObserver === "undefined" ? null : new ResizeObserver(paint);
    observer?.observe(track);

    return () => {
      cancelAnimationFrame(frame);
      controller.abort();
      observer?.disconnect();
    };
  }, [chipCount]);

  return { trackRef, overflow };
}
