import { useVirtualizer } from "@tanstack/react-virtual";
import type { RefObject } from "react";

/**
 * Fixed-height row virtualizer for `FileTablePreview`, over its scrolling container. It returns
 * the rows in view and the total height read on every render: the virtualizer instance is
 * stable, so a compiled caller reading it directly would keep its first, unmeasured output.
 */
export function useTableVirtualizer(
  count: number,
  scrollRef: RefObject<HTMLDivElement | null>,
  rowHeight: number
) {
  "use no memo";

  // react-doctor-disable-next-line react-hooks-js/incompatible-library -- `use no memo` isolates TanStack Virtual's mutable measurement state in this hook.
  const virtualizer = useVirtualizer<HTMLDivElement, HTMLTableRowElement>({
    count,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => rowHeight,
    overscan: 12,
  });
  return { items: virtualizer.getVirtualItems(), totalSize: virtualizer.getTotalSize() };
}
