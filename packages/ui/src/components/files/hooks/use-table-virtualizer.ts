import { useVirtualizer } from "@tanstack/react-virtual";
import type { RefObject } from "react";

/** Fixed-height row virtualizer for `FileTablePreview`, over its scrolling container. */
export function useTableVirtualizer(
  count: number,
  scrollRef: RefObject<HTMLDivElement | null>,
  rowHeight: number
) {
  "use no memo";

  // react-doctor-disable-next-line react-hooks-js/incompatible-library -- `use no memo` isolates TanStack Virtual's mutable measurement state in this hook.
  return useVirtualizer<HTMLDivElement, HTMLTableRowElement>({
    count,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => rowHeight,
    overscan: 12,
  });
}
