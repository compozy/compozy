/**
 * A flow edge drawn on the ELK route (orthogonal sections, rounded at the bends) with its trigger
 * label at the position ELK reserved for it. The kind is the line pattern; the selected node's
 * edges switch to the accent color.
 */
import { BaseEdge, EdgeLabelRenderer, type Edge, type EdgeProps } from "@xyflow/react";

import { cn } from "@compozy/ui";

import type { ResolvedEdge } from "#gallery/flow";
import {
  edgeInk,
  edgeInkHighlighted,
  edgeKindStyle,
  edgeLabelClass,
  roundedPath,
} from "#gallery/shell/flows/edge-style";
import type { EdgeRoute } from "#gallery/shell/flows/layout-state";

export type FlowEdgeData = {
  edge: ResolvedEdge;
  route: EdgeRoute | null;
  highlighted: boolean;
  dimmed: boolean;
};
export type FlowEdgeType = Edge<FlowEdgeData, "flow">;

const arrow = "flow-arrow";
const arrowHighlighted = "flow-arrow-highlighted";

export function FlowEdgeView({ id, data }: EdgeProps<FlowEdgeType>) {
  if (!data?.route) return null;
  const { edge, route, highlighted, dimmed } = data;
  const style = edgeKindStyle[edge.kind];
  const label = route.label;
  return (
    <>
      <BaseEdge
        id={id}
        interactionWidth={0}
        markerEnd={`url(#${highlighted ? arrowHighlighted : arrow})`}
        path={roundedPath(route.points)}
        style={{
          stroke: highlighted ? edgeInkHighlighted : edgeInk,
          strokeWidth: highlighted ? 2 : 1.4,
          strokeDasharray: style.dash,
          strokeLinecap: style.round ? "round" : undefined,
          opacity: dimmed ? 0.35 : 1,
        }}
      />
      {label ? (
        <EdgeLabelRenderer>
          <div
            className={cn(
              edgeLabelClass,
              "nodrag nopan pointer-events-none absolute top-0 left-0 bg-surface-2 text-center",
              highlighted ? "text-accent" : "text-muted",
              dimmed && "opacity-40"
            )}
            data-testid="flow-edge-label"
            style={{
              transform: `translate(${label.x}px, ${label.y}px)`,
              width: label.width,
              height: label.height,
            }}
          >
            {edge.on}
          </div>
        </EdgeLabelRenderer>
      ) : null}
    </>
  );
}

/** Arrowheads for both edge states; rendered once per canvas. */
export function FlowMarkers() {
  return (
    <svg aria-hidden="true" className="absolute size-0">
      <defs>
        {[
          { id: arrow, fill: edgeInk },
          { id: arrowHighlighted, fill: edgeInkHighlighted },
        ].map(marker => (
          <marker
            id={marker.id}
            key={marker.id}
            markerHeight="7"
            markerWidth="7"
            orient="auto-start-reverse"
            refX="9"
            refY="5"
            viewBox="0 0 10 10"
          >
            <path d="M0 0L10 5L0 10z" style={{ fill: marker.fill }} />
          </marker>
        ))}
      </defs>
    </svg>
  );
}
