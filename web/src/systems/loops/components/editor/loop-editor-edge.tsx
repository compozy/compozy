import { X } from "lucide-react";
import {
  BaseEdge,
  EdgeLabelRenderer,
  getBezierPath,
  type Edge,
  type EdgeProps,
} from "@xyflow/react";

import { Button, cn } from "@compozy/ui";

const EDGE_INTERACTION_WIDTH = 28;

export interface LoopEditorEdgeData extends Record<string, unknown> {
  routeLabel?: string;
  readOnly?: boolean;
  onDelete?: (edgeId: string) => void;
}

export function LoopEditorEdge({
  id,
  sourceX,
  sourceY,
  sourcePosition,
  targetX,
  targetY,
  targetPosition,
  data,
  markerEnd,
  style,
  selected,
}: EdgeProps<Edge<LoopEditorEdgeData>>) {
  const [edgePath, labelX, labelY] = getBezierPath({
    sourceX,
    sourceY,
    sourcePosition,
    targetX,
    targetY,
    targetPosition,
  });
  const routeLabel = data?.routeLabel ?? "";
  const onDelete = data?.onDelete;
  const deletable = selected === true && data?.readOnly !== true && onDelete !== undefined;
  const hasOverlay = routeLabel !== "" || deletable;
  return (
    <>
      <BaseEdge
        id={id}
        interactionWidth={EDGE_INTERACTION_WIDTH}
        markerEnd={markerEnd}
        path={edgePath}

        style={selected === true ? { ...style, stroke: "var(--color-accent-dim)" } : style}
      />
      {hasOverlay ? (
        <EdgeLabelRenderer>
          <div
            className="nodrag nopan pointer-events-auto absolute flex items-center gap-1"
            data-edge-id={id}
            style={{ transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)` }}
          >
            {routeLabel === "" ? null : (
              <span
                className={cn(
                  "max-w-36 truncate rounded-xs border border-line bg-elevated px-1 py-px",
                  "font-mono text-pill-group-badge",
                  selected === true ? "text-accent-strong" : "text-subtle"
                )}
                data-testid="loop-editor-edge-route-label"
                title={routeLabel}
              >
                {routeLabel}
              </span>
            )}
            {deletable ? (
              <Button
                aria-label="Delete connection"
                className="shrink-0 rounded-full border border-line-strong bg-elevated text-muted hover:border-transparent hover:bg-danger-tint hover:text-danger"
                data-testid="loop-editor-edge-delete"
                onClick={() => onDelete(id)}
                size="icon-xs"
                type="button"
                variant="ghost"
              >
                <X aria-hidden="true" className="size-3" />
              </Button>
            ) : null}
          </div>
        </EdgeLabelRenderer>
      ) : null}
    </>
  );
}
