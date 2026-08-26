import type { RefObject } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";

const GRID_ROW_ESTIMATE = 156;

export function usePaletteGridVirtualizer(
  rows: readonly { key: string }[],
  viewportRef: RefObject<HTMLDivElement | null>
) {
  "use no memo";

  // react-doctor-disable-next-line react-hooks-js/incompatible-library -- `use no memo` isolates TanStack Virtual's mutable measurement state in this hook.
  return useVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: rows.length,
    getScrollElement: () => viewportRef.current,
    getItemKey: index => rows[index]?.key ?? index,
    estimateSize: () => GRID_ROW_ESTIMATE,
    overscan: 4,
    useFlushSync: false,
  });
}
