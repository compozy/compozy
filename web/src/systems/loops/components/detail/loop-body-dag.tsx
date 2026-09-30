import { ArrowRight } from "lucide-react";
import { createElement } from "react";

import { fanOutSummary, nodeClassLabel, routeSummary } from "../../lib/loop-graph";
import type { LoopGraph, LoopGraphNode } from "../../lib/loop-graph";
import { humanizeLoopNodeId } from "../../lib/loop-node-labels";
import { loopNodeClassIcon } from "../../lib/loop-node-kind-icons";

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
      <div className="flex items-stretch gap-0 overflow-x-auto p-4">
        {graph.nodes.map((node, index) => (
          <div key={node.id} className="flex items-stretch">
            <DagNode node={node} />
            {index < graph.nodes.length - 1 ? (
              <span aria-hidden="true" className="flex items-center self-center px-1 text-faint">
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
    <div
      className="flex w-31 shrink-0 flex-col gap-1 overflow-hidden rounded-md bg-canvas px-3 py-2.5 shadow-card"
      data-testid="loop-dag-node"
      data-node-id={node.id}
    >
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
