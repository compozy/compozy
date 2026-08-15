import { useEffect, useRef, useState } from "react";

export type AttachmentRailOverflow = "none" | "start" | "end" | "start end";

function prefersReducedMotion(): boolean {
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

function fadePad(rail: HTMLElement): number {
  const raw = getComputedStyle(rail).getPropertyValue("--att-fade");
  const value = Number.parseFloat(raw);
  return Number.isFinite(value) ? value : 56;
}

function measureOverflow(track: HTMLElement): AttachmentRailOverflow {
  const max = track.scrollWidth - track.clientWidth;
  if (max <= 2) return "none";
  const bits: string[] = [];
  if (track.scrollLeft > 2) bits.push("start");
  if (track.scrollLeft < max - 2) bits.push("end");
  return bits.length > 0 ? (bits.join(" ") as AttachmentRailOverflow) : "none";
}

function keepInView(rail: HTMLElement, track: HTMLElement, item: HTMLElement) {
  const pad = fadePad(rail);
  const itemBox = item.getBoundingClientRect();
  const trackBox = track.getBoundingClientRect();
  let delta = 0;
  if (itemBox.left < trackBox.left + pad) delta = itemBox.left - (trackBox.left + pad);
  else if (itemBox.right > trackBox.right - pad) delta = itemBox.right - (trackBox.right - pad);
  if (Math.abs(delta) < 1) return;
  track.scrollTo({
    left: track.scrollLeft + delta,
    behavior: prefersReducedMotion() ? "auto" : "smooth",
  });
}

export function useAttachmentRail(itemCount: number) {
  const railRef = useRef<HTMLDivElement>(null);
  const trackRef = useRef<HTMLUListElement>(null);
  const [overflow, setOverflow] = useState<AttachmentRailOverflow>("none");

  useEffect(() => {
    const rail = railRef.current;
    const track = trackRef.current;
    const controller = new AbortController();
    const { signal } = controller;
    let frame = 0;
    const observer = new ResizeObserver(() => {
      if (track) setOverflow(measureOverflow(track));
    });

    if (rail && track) {
      const paint = () => {
        setOverflow(measureOverflow(track));
      };
      const handleWheel = (event: WheelEvent) => {
        if (track.scrollWidth <= track.clientWidth + 2) return;
        if (Math.abs(event.deltaY) <= Math.abs(event.deltaX)) return;
        event.preventDefault();
        track.scrollLeft += event.deltaY;
      };
      const handleFocusIn = (event: FocusEvent) => {
        const target = event.target;
        if (!(target instanceof HTMLElement)) return;
        const item = target.closest(
          "[data-testid='composer-attachment-tile'], [data-attachment-rail-item]"
        );
        if (item instanceof HTMLElement && rail.contains(item)) {
          keepInView(rail, track, item);
          paint();
        }
      };
      paint();
      const images = Array.from(track.querySelectorAll("img"));
      const incomplete = images.filter(image => !image.complete);
      if (incomplete.length === 0) {
        frame = requestAnimationFrame(paint);
      } else {
        for (const image of incomplete) {
          image.addEventListener("load", paint, { signal });
          image.addEventListener("error", paint, { signal });
        }
      }
      track.addEventListener("scroll", paint, { passive: true, signal });
      track.addEventListener("wheel", handleWheel, { passive: false, signal });
      rail.addEventListener("focusin", handleFocusIn, { signal });
      window.addEventListener("resize", paint, { signal });
      observer.observe(track);
    }

    return () => {
      cancelAnimationFrame(frame);
      controller.abort();
      observer.disconnect();
    };
  }, [itemCount]);

  return { overflow, railRef, trackRef };
}
