import { cn } from "@compozy/ui";

import { useLayoutDraftSeam } from "../../hooks/use-layout-draft-seam";
import type {
  WindowManagerLayoutNode,
  WindowManagerLayoutSplit,
} from "../../lib/window-manager-layout-types";

interface LayoutCanvasSeamProps {
  root: WindowManagerLayoutNode;
  split: WindowManagerLayoutSplit;
  /** The seam sits after member `index`. */
  index: number;
  onChange: (next: WindowManagerLayoutNode) => void;
}

/**
 * The line between two members, drawn like the shell's split seam
 * (`os-split-view.tsx`): a hairline with a wider hit area, accent on hover and
 * drag. It is also a slider — arrow keys move it, double-click balances the
 * pair — so a split can be tuned without a pointer.
 */
export function LayoutCanvasSeam({ root, split, index, onChange }: LayoutCanvasSeamProps) {
  const model = useLayoutDraftSeam(root, split, index, onChange);
  const row = split.axis === "horizontal";

  return (
    <div
      aria-label="Resize windows"
      aria-orientation={row ? "vertical" : "horizontal"}
      aria-valuemax={100}
      aria-valuemin={0}
      aria-valuenow={Math.round(model.ratio * 100)}
      aria-valuetext={`${model.leadingLabel} and ${model.trailingLabel}`}
      className={cn(
        "group/seam relative z-10 shrink-0 bg-line outline-none",
        row ? "w-px cursor-col-resize" : "h-px cursor-row-resize"
      )}
      data-dragging={model.dragging ? "true" : undefined}
      data-testid={`layout-canvas-seam-${split.id}-${index}`}
      role="slider"
      tabIndex={0}
      onClick={event => event.stopPropagation()}
      onDoubleClick={model.onDoubleClick}
      onKeyDown={model.onKeyDown}
      onPointerDown={model.onPointerDown}
    >
      <span className={cn("absolute", row ? "inset-y-0 -inset-x-1.5" : "inset-x-0 -inset-y-1.5")} />
      <span
        aria-hidden="true"
        className={cn(
          "pointer-events-none absolute rounded-pill transition-colors duration-fast",
          row ? "inset-y-0 -left-px w-0.75" : "inset-x-0 -top-px h-0.75",
          "group-hover/seam:bg-brand group-focus-visible/seam:bg-brand",
          "group-focus-visible/seam:shadow-focus-ring",
          model.dragging && "bg-brand"
        )}
      />
      {model.dragging ? (
        <span
          className={cn(
            "pointer-events-none absolute z-10 rounded-pill bg-elevated px-2 py-0.5 font-mono text-badge whitespace-nowrap shadow-pop",
            row ? "top-1/2 left-2 -translate-y-1/2" : "top-2 left-1/2 -translate-x-1/2",
            model.snapped ? "text-brand-strong" : "text-fg"
          )}
          role="status"
        >
          {model.leadingLabel} · {model.trailingLabel}
        </span>
      ) : null}
    </div>
  );
}
