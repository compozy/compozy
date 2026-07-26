import type { RefObject } from "react";

import { cn } from "@agh/ui";

import {
  useLayoutGroupFrameDrag,
  type LayoutGroupEdge,
} from "../../hooks/use-layout-group-frame-drag";
import type {
  WindowManagerLayoutGroup,
  WindowManagerNormalizedRect,
} from "../../lib/window-manager-layout-types";

const EDGE_LABEL: Record<LayoutGroupEdge, string> = {
  top: "top",
  right: "right",
  bottom: "bottom",
  left: "left",
};

const EDGE_BOX: Record<LayoutGroupEdge, string> = {
  top: "-top-1 right-2 left-2 h-2 cursor-ns-resize",
  bottom: "-bottom-1 right-2 left-2 h-2 cursor-ns-resize",
  left: "-left-1 top-2 bottom-2 w-2 cursor-ew-resize",
  right: "-right-1 top-2 bottom-2 w-2 cursor-ew-resize",
};

const EDGE_GRIP: Record<LayoutGroupEdge, string> = {
  top: "h-0.5 w-6",
  bottom: "h-0.5 w-6",
  left: "h-6 w-0.5",
  right: "h-6 w-0.5",
};

interface LayoutCanvasGroupEdgeProps {
  group: WindowManagerLayoutGroup;
  edge: LayoutGroupEdge;
  canvasRef: RefObject<HTMLElement | null>;
  onChange: (frame: WindowManagerNormalizedRect) => void;
}

/** One edge of a group's share of the desktop — a divider, and a slider. */
export function LayoutCanvasGroupEdge({
  group,
  edge,
  canvasRef,
  onChange,
}: LayoutCanvasGroupEdgeProps) {
  const model = useLayoutGroupFrameDrag(group, edge, canvasRef, onChange);

  return (
    <div
      aria-label={`Group ${EDGE_LABEL[edge]} edge`}
      aria-orientation={edge === "left" || edge === "right" ? "vertical" : "horizontal"}
      aria-valuemax={100}
      aria-valuemin={0}
      aria-valuenow={Math.round(model.position * 100)}
      aria-valuetext={model.label}
      className={cn(
        "group/edge absolute z-3 flex items-center justify-center",
        EDGE_BOX[edge],
        "focus-visible:outline-none focus-visible:shadow-focus-ring"
      )}
      data-dragging={model.dragging ? "true" : undefined}
      data-testid={`layout-canvas-group-edge-${group.id}-${edge}`}
      role="slider"
      tabIndex={0}
      onKeyDown={model.onKeyDown}
      onPointerDown={model.onPointerDown}
    >
      <span
        aria-hidden="true"
        className={cn(
          "rounded-xxs bg-transparent transition-colors duration-base ease-out",
          EDGE_GRIP[edge],
          "group-hover/edge:bg-accent group-focus-visible/edge:bg-accent",
          model.dragging && "bg-accent"
        )}
      />
    </div>
  );
}
