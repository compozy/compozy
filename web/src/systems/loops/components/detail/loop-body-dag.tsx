import { ArrowRight } from "lucide-react";

import { fanOutSummary, nodeClassLabel } from "../../lib/loop-graph";
import type { LoopGraph, LoopGraphNode } from "../../lib/loop-graph";
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
        className="rounded-lg border border-line bg-canvas-soft px-4 py-6 text-center text-small-body text-subtle"
        data-testid="loop-dag-empty"
      >
        This Loop exposes no readable body graph.
      </div>
    );
  }
  return (
    <div className="rounded-lg border border-line bg-canvas-soft" data-testid="loop-dag">
      <div className="flex items-stretch gap-0 overflow-x-auto p-4">
        {graph.nodes.map((node, index) => (
          <div key={node.id} className="flex items-stretch">
            <DagNode node={node} />
            {index < graph.nodes.length - 1 ? (
              <span aria-hidden="true" className="flex items-center self-center px-1 text-faint">
                <ArrowRight className="size-4.5" />
              </span>
            ) : null}
          </div>
        ))}
      </div>
      <p className="border-t border-line-soft px-4 py-3 text-form-hint leading-relaxed text-subtle">
        Read-only view. Open the builder to fork and edit this graph.
      </p>
    </div>
  );
}

function DagNode({ node }: { node: LoopGraphNode }) {
  const summary = fanOutSummary(node);
  const kindLabel = summary ?? node.kind;
  const ClassIcon = node.nodeClass
    ? loopNodeClassIcon({
        nodeClass: node.nodeClass,
        isFanOut: node.kind === "fan-out" || summary !== null,
        isGate: node.isGate,
      })
    : undefined;
  return (
    <div
      className="flex min-w-[124px] shrink-0 flex-col gap-1 overflow-hidden rounded-md border border-line bg-canvas-tint px-3 py-2.5"
      data-testid="loop-dag-node"
      data-node-id={node.id}
    >
      <span className="flex min-w-0 items-center gap-1.5">
        {ClassIcon ? (
          <ClassIcon aria-hidden="true" className="size-3.5 shrink-0 text-muted" />
        ) : null}
        <span
          className="min-w-0 truncate text-small-body font-medium text-fg-strong"
          title={node.id}
        >
          {node.id}
        </span>
      </span>
      <span className="min-w-0 truncate font-mono text-mono-id text-faint">
        {nodeClassLabel(node)}
      </span>
      <span
        className="min-w-0 truncate font-mono text-mono-id text-subtle"
        title={kindLabel || undefined}
      >
        {kindLabel}
      </span>
    </div>
  );
}
