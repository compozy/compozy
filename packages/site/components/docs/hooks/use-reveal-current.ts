"use client";

import { useEffect, useRef } from "react";

/**
 * Brings the current row (`selector`: the page being read in the sidebar, the section being read
 * in "On this page") into its `.docs-rail` when it is out of view; a row already in view leaves the
 * rail where the reader scrolled it. The sidebar outlives navigations, so when the tree it shows
 * changes (`scope`, another section) it starts again from the top first. Only the rail scrolls,
 * never the page.
 */
export function useRevealCurrent<T extends HTMLElement>(
  current: string,
  scope: string,
  selector = '[aria-current="page"]'
) {
  const ref = useRef<T>(null);
  const shown = useRef(scope);
  useEffect(() => {
    const rail = ref.current?.closest<HTMLElement>(".docs-rail");
    if (!rail) return;
    if (shown.current !== scope) {
      shown.current = scope;
      rail.scrollTop = 0;
    }
    const row = ref.current?.querySelector<HTMLElement>(selector);
    if (!row || rail.scrollHeight <= rail.clientHeight) return;
    const railBox = rail.getBoundingClientRect();
    const rowBox = row.getBoundingClientRect();
    if (rowBox.top >= railBox.top && rowBox.bottom <= railBox.bottom) return;
    rail.scrollTop += rowBox.top - railBox.top - (rail.clientHeight - rowBox.height) / 2;
  }, [current, scope, selector]);
  return ref;
}
