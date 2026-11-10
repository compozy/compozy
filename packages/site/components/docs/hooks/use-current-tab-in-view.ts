"use client";

import { useEffect, useRef } from "react";

/**
 * Keeps the current tab inside the strip when the strip scrolls (390 and 768 show part of it):
 * only the strip scrolls, and only when the tab is not already fully in view.
 */
export function useCurrentTabInView(current: string | null) {
  const ref = useRef<HTMLElement>(null);
  useEffect(() => {
    const strip = ref.current;
    const tab = strip?.querySelector<HTMLElement>('[aria-current="page"]');
    if (!strip || !tab || strip.scrollWidth <= strip.clientWidth) return;
    const stripBox = strip.getBoundingClientRect();
    const tabBox = tab.getBoundingClientRect();
    if (tabBox.left >= stripBox.left && tabBox.right <= stripBox.right) return;
    strip.scrollLeft += tabBox.left - stripBox.left - (stripBox.width - tabBox.width) / 2;
  }, [current]);
  return ref;
}
