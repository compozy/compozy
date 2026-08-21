import { useLayoutEffect, useRef } from "react";

import { CommandItem, CommandList } from "@compozy/ui";

import type { PaletteViewRow } from "../lib/palette-view-registry";
import { paletteItemClass, paletteRowEstimate } from "../lib/palette-view-inset";

export const PALETTE_VIEW_VIRTUAL_THRESHOLD = 150;

export function OsPaletteVirtualRows({
  className,
  note,
  rows,
}: {
  className: string;
  note: React.ReactNode;
  rows: readonly PaletteViewRow[];
}) {
  const viewportRef = useRef<HTMLDivElement | null>(null);
  useLayoutEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) return;
    const root =
      viewport.closest("[data-testid='os-command-palette']") ??
      viewport.closest("[data-slot='command']");
    if (!root) return;
    const sync = () => {
      root
        .querySelector<HTMLElement>("[data-slot='command-item'][data-selected='true']")
        ?.scrollIntoView({ block: "nearest" });
    };
    const observer = new MutationObserver(sync);
    observer.observe(root, {
      subtree: true,
      attributes: true,
      attributeFilter: ["data-selected"],
    });
    sync();
    return () => observer.disconnect();
  }, [rows]);
  return (
    <CommandList ref={viewportRef} className={className} data-virtualized="true">
      {rows.map(row => (
        <div
          key={row.value}
          style={{
            contentVisibility: "auto",
            containIntrinsicSize: `0 ${paletteRowEstimate(row.twoLine)}px`,
          }}
        >
          <CommandItem
            className={paletteItemClass(row.twoLine)}
            forceMount
            value={row.value}
            data-testid={row.testId}
            disabled={row.disabled}
            onSelect={row.onSelect}
          >
            {row.node}
          </CommandItem>
        </div>
      ))}
      {note}
    </CommandList>
  );
}
