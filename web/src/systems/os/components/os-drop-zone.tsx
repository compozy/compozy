import { cn } from "@/lib/utils";

import type { DropEdge } from "../lib/drop-edge";
import type { PixelRect } from "../lib/window-manager-types";

const ZONE_BOX: Record<DropEdge, string> = {
  left: "inset-y-0 left-0 w-1/2 border-l-2",
  right: "inset-y-0 right-0 w-1/2 border-r-2",
  top: "inset-x-0 top-0 h-1/2 border-t-2",
  bottom: "inset-x-0 bottom-0 h-1/2 border-b-2",
};

const ROOT_BAND: Record<DropEdge, string> = {
  left: "inset-y-0 left-0 w-1/3 border-l-2",
  right: "inset-y-0 right-0 w-1/3 border-r-2",
  top: "inset-x-0 top-0 h-1/3 border-t-2",
  bottom: "inset-x-0 bottom-0 h-1/3 border-b-2",
};

export interface OsDropZoneProps {
  /** The target window's box in the content area. */
  rect: PixelRect;
  edge: DropEdge;
  mode?: "split" | "swap";
}

/**
 * Where a dragged tab or window head would land: the tinted half of the target window with an
 * edge line, or the whole window under the swap modifier. Tint plus line, never color alone.
 */
export function OsDropZone({ rect, edge, mode = "split" }: OsDropZoneProps) {
  return (
    <span
      aria-hidden="true"
      className="pointer-events-none absolute z-20"
      style={{ left: rect.x, top: rect.y, width: rect.w, height: rect.h }}
    >
      {mode === "swap" ? (
        <span
          className="absolute inset-0 grid place-items-center border-2 border-brand bg-brand-tint-strong"
          data-mode="swap"
          data-slot="os-drop-zone"
        >
          <span className="rounded-pill bg-elevated px-3 py-1 text-meta font-medium text-fg shadow-pop">
            Swap
          </span>
        </span>
      ) : (
        <span
          className={cn("absolute border-brand bg-brand-tint-strong", ZONE_BOX[edge])}
          data-edge={edge}
          data-slot="os-drop-zone"
        />
      )}
    </span>
  );
}

/** The root-edge drop: a full-height or full-width member at the tab's outer edge. */
export function OsRootDropZone({ edge }: { edge: DropEdge }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "pointer-events-none absolute z-30 border-brand bg-brand-tint-strong",
        ROOT_BAND[edge]
      )}
      data-edge={edge}
      data-slot="os-drop-zone-root"
    />
  );
}
