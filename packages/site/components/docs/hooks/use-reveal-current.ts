"use client";

import { useEffect, useRef } from "react";

/**
 * Brings the row of the page being read into its rail. The rail mounts with each page, so a deep
 * page would otherwise open with its row far below the fold. Only the rail scrolls, never the page.
 */
export function useRevealCurrent<T extends HTMLElement>(activeUrl: string) {
  const ref = useRef<T>(null);
  useEffect(() => {
    const row = ref.current?.querySelector<HTMLElement>('[aria-current="page"]');
    const rail = row?.closest<HTMLElement>(".docs-rail");
    if (!row || !rail || rail.scrollHeight <= rail.clientHeight) return;
    const railBox = rail.getBoundingClientRect();
    const rowBox = row.getBoundingClientRect();
    if (rowBox.top >= railBox.top && rowBox.bottom <= railBox.bottom) return;
    rail.scrollTop += rowBox.top - railBox.top - (rail.clientHeight - rowBox.height) / 2;
  }, [activeUrl]);
  return ref;
}
