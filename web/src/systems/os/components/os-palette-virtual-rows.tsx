import { useRef } from "react";

import { CommandItem, CommandList } from "@compozy/ui";
import { useVirtualizer } from "@tanstack/react-virtual";

import type { PaletteViewRow } from "../lib/palette-view-registry";
import { paletteViewItemClass } from "../lib/palette-view-inset";

const ROW_ESTIMATE = 48;
const ROW_OVERSCAN = 8;

export function OsPaletteVirtualRows({
  className,
  note,
  rows,
}: {
  className: string;
  note: React.ReactNode;
  rows: readonly PaletteViewRow[];
}) {
  "use no memo";

  const viewportRef = useRef<HTMLDivElement | null>(null);
  // oxlint-disable-next-line react/incompatible-library -- virtualizer state is isolated inside this compiler boundary.
  const virtualizer = useVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: rows.length,
    getScrollElement: () => viewportRef.current,
    getItemKey: index => rows[index]?.value ?? index,
    estimateSize: () => ROW_ESTIMATE,
    overscan: ROW_OVERSCAN,
    useFlushSync: false,
  });
  return (
    <CommandList ref={viewportRef} className={className}>
      <div className="relative w-full" style={{ height: virtualizer.getTotalSize() }}>
        {virtualizer.getVirtualItems().map(item => {
          const row = rows[item.index];
          if (!row) return null;
          return (
            <div
              key={row.value}
              ref={virtualizer.measureElement}
              className="absolute top-0 left-0 w-full"
              data-index={item.index}
              style={{ transform: `translateY(${item.start}px)` }}
            >
              <CommandItem
                className={paletteViewItemClass}
                forceMount
                value={row.value}
                data-testid={row.testId}
                disabled={row.disabled}
                onSelect={row.onSelect}
              >
                {row.node}
              </CommandItem>
            </div>
          );
        })}
      </div>
      {note}
    </CommandList>
  );
}
