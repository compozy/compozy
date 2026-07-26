import type { RefObject } from "react";

import { cn } from "@agh/ui";

import type { ProjectedSeam } from "@/systems/os";

import { useLayoutDraftSeam } from "../../hooks/use-layout-draft-seam";
import { LAYOUT_CANVAS_REFERENCE } from "../../lib/window-manager-layout-reference";
import type { WindowManagerLayoutDesktop } from "../../lib/window-manager-layout-types";

interface LayoutCanvasSeamProps {
  seam: ProjectedSeam;
  desktop: WindowManagerLayoutDesktop;
  canvasRef: RefObject<HTMLElement | null>;
  onChange: (next: WindowManagerLayoutDesktop) => void;
}

function percent(value: number, span: number): string {
  return `${(value / span) * 100}%`;
}

/**
 * The divider between two tiles, dragged where it sits. It is also a slider:
 * arrow keys move it, so a layout can be balanced without a pointer.
 */
export function LayoutCanvasSeam({ seam, desktop, canvasRef, onChange }: LayoutCanvasSeamProps) {
  const model = useLayoutDraftSeam(seam, desktop, canvasRef, onChange);
  const vertical = seam.orientation === "vertical";
  const centreX = seam.rect.x + seam.rect.w / 2;
  const centreY = seam.rect.y + seam.rect.h / 2;

  return (
    <div
      aria-label={`Balance between ${seam.leadingWindowIds.length} and ${seam.trailingWindowIds.length} window${seam.trailingWindowIds.length === 1 ? "" : "s"}`}
      aria-orientation={vertical ? "vertical" : "horizontal"}
      aria-valuemax={100}
      aria-valuemin={0}
      aria-valuenow={Math.round(model.ratio * 100)}
      aria-valuetext={`${model.leadingLabel} and ${model.trailingLabel}`}
      className={cn(
        "group absolute z-2 flex items-center justify-center",
        vertical ? "cursor-col-resize" : "cursor-row-resize",
        "focus-visible:outline-none focus-visible:shadow-focus-ring"
      )}
      data-dragging={model.dragging ? "true" : undefined}
      data-testid={`layout-canvas-seam-${seam.id}`}
      role="slider"
      style={
        vertical
          ? {
              left: percent(centreX, LAYOUT_CANVAS_REFERENCE.w),
              top: percent(seam.rect.y, LAYOUT_CANVAS_REFERENCE.h),
              height: percent(seam.rect.h, LAYOUT_CANVAS_REFERENCE.h),
              width: "14px",
              marginLeft: "-7px",
            }
          : {
              top: percent(centreY, LAYOUT_CANVAS_REFERENCE.h),
              left: percent(seam.rect.x, LAYOUT_CANVAS_REFERENCE.w),
              width: percent(seam.rect.w, LAYOUT_CANVAS_REFERENCE.w),
              height: "14px",
              marginTop: "-7px",
            }
      }
      tabIndex={0}
      onKeyDown={model.onKeyDown}
      onPointerDown={model.onPointerDown}
    >
      <span
        aria-hidden="true"
        className={cn(
          "rounded-pill bg-line-strong transition-colors duration-base ease-out",
          "group-hover:bg-accent group-focus-visible:bg-accent",
          vertical ? "h-[34%] w-0.5" : "h-0.5 w-[34%]",
          model.dragging && "bg-accent"
        )}
      />
      {model.dragging ? (
        <span
          className={cn(
            "pointer-events-none absolute z-6 rounded-xs border px-1.5 py-0.5",
            "bg-elevated font-mono text-badge whitespace-nowrap shadow-overlay",
            model.snapped ? "border-accent-dim text-accent-strong" : "border-line-strong text-fg"
          )}
          role="status"
        >
          {model.leadingLabel} · {model.trailingLabel}
        </span>
      ) : null}
    </div>
  );
}
