import { cn } from "@compozy/ui";

import { OsSplitSeam } from "@/systems/os";

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
 * The shell's split seam (`OsSplitSeam`) driving the draft: drag it, nudge it
 * with the arrow keys, double-click to balance the pair. While it is held it
 * shows both shares, brand-coloured when they sit on a detent.
 */
export function LayoutCanvasSeam({ root, split, index, onChange }: LayoutCanvasSeamProps) {
  const model = useLayoutDraftSeam(root, split, index, onChange);
  const row = split.axis === "horizontal";

  return (
    <OsSplitSeam
      aria-valuetext={`${model.leadingLabel} and ${model.trailingLabel}`}
      axis={split.axis}
      data-testid={`layout-canvas-seam-${split.id}-${index}`}
      state={model.dragging ? "drag" : undefined}
      valueNow={Math.round(model.ratio * 100)}
      onDoubleClick={model.onDoubleClick}
      onKeyDown={model.onKeyDown}
      onPointerDown={model.onPointerDown}
    >
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
    </OsSplitSeam>
  );
}
