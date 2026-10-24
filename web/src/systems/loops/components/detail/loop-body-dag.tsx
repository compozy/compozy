import { ArrowRight } from "lucide-react";
import { createElement } from "react";

import { SkeletonText } from "@compozy/ui";

import { placeholderSlots } from "../../lib/placeholder-slots";
import { fanOutSummary, nodeClassLabel, routeSummary } from "../../lib/loop-graph";
import type { LoopGraph, LoopGraphNode } from "../../lib/loop-graph";
import { humanizeLoopNodeId } from "../../lib/loop-node-labels";
import { loopNodeClassIcon } from "../../lib/loop-node-kind-icons";

// Shared by the real graph and its skeleton so a step card is the same size either way.
const DAG_LANE_CLASS = "flex items-stretch gap-0 overflow-x-auto p-4";
const DAG_CARD_CLASS =
  "flex w-31 shrink-0 flex-col gap-1 overflow-hidden rounded-md bg-card px-3 py-2.5 shadow-card";
const DAG_ARROW_CLASS = "flex items-center self-center px-1 text-faint";

interface LoopBodyDagProps {
  graph: LoopGraph;
}

/**
 * Read-only body graph: a horizontal spine of neutral node cards in topological
 * order. The class glyph carries identity; labels stay faint. Color is state-only.
 */
export function LoopBodyDag({ graph }: LoopBodyDagProps) {
  if (graph.nodes.length === 0) {
    return (
      <div
        className="rounded-lg bg-sunken px-4 py-6 text-center text-small-body text-subtle"
        data-testid="loop-dag-empty"
      >
        This Loop has no steps to show.
      </div>
    );
  }
  return (
    <div className="rounded-lg bg-sunken" data-testid="loop-dag">
      <div className={DAG_LANE_CLASS}>
        {graph.nodes.map((node, index) => (
          <div key={node.id} className="flex items-stretch">
            <DagNode node={node} />
            {index < graph.nodes.length - 1 ? (
              <span aria-hidden="true" className={DAG_ARROW_CLASS}>
                <ArrowRight className="size-4" />
              </span>
            ) : null}
          </div>
        ))}
      </div>
    </div>
  );
}

function DagNode({ node }: { node: LoopGraphNode }) {
  const isRoute = node.kind === "route";
  const isAsk = node.kind === "ask";

  const summary = isRoute ? routeSummary(node) : fanOutSummary(node);
  const kindLabel = summary ?? node.kind;
  const classLabel = nodeClassLabel(node);
  const detail = [...new Set([classLabel, kindLabel].filter(Boolean))].join(" · ");
  const classIcon = node.nodeClass
    ? loopNodeClassIcon({
        nodeClass: node.nodeClass,
        isFanOut: node.kind === "fan-out",
        isGate: node.isGate,
        isRoute,
        isAsk,
      })
    : undefined;
  return (
    <div className={DAG_CARD_CLASS} data-testid="loop-dag-node" data-node-id={node.id}>
      <span className="flex min-w-0 items-center gap-1.5">
        {classIcon
          ? createElement(classIcon, {
              "aria-hidden": true,
              className: "size-3.5 shrink-0 text-muted",
            })
          : null}
        <span
          className="min-w-0 truncate text-small-body font-medium text-fg-strong"
          title={node.id}
        >
          {humanizeLoopNodeId(node.id)}
        </span>
      </span>
      <span className="min-w-0 truncate text-form-hint text-faint" title={detail || undefined}>
        {detail}
      </span>
    </div>
  );
}

/**
 * The steps graph while the Loop is being read: a spine of `count` step cards in the graph's own
 * recessed lane. Presentational, so the caller wraps the area in a `SkeletonRegion`.
 */
export function LoopBodyDagSkeleton({ count = 4 }: { count?: number }) {
  const slots = placeholderSlots("dag-step", count);
  return (
    <div aria-hidden="true" className="rounded-lg bg-sunken" data-testid="loop-dag-loading">
      <div className={DAG_LANE_CLASS}>
        {slots.map(slot => (
          <div className="flex items-stretch" key={slot.id}>
            <div className={DAG_CARD_CLASS}>
              <SkeletonText className="w-20 text-small-body" />
              <SkeletonText className="w-16 text-form-hint" />
            </div>
            {slot.position < slots.length - 1 ? (
              <span aria-hidden="true" className={DAG_ARROW_CLASS}>
                <ArrowRight className="size-4" />
              </span>
            ) : null}
          </div>
        ))}
      </div>
    </div>
  );
}
