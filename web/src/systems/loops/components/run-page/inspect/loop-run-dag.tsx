import { createElement, useEffect, useRef } from "react";
import { useReducedMotionConfig } from "motion/react";

import { cn, Empty, PillDot } from "@compozy/ui";

import { withOccurrenceKeys } from "@/lib/occurrence-keys";

import type { LoopDagEdgeState } from "../../../lib/loop-run-dag-edges";
import type { LoopDagColumn, LoopDagNode, LoopRunDagModel } from "../../../lib/loop-run-dag-view";
import type { LoopNodeSelection } from "../../../lib/loop-run-registers-view";
import { LOOP_PROGRESS_SEGMENT_CLASS } from "../loop-progress-segment-class";
import { LoopNodeStateChip } from "../loop-node-state-chip";

interface LoopRunDagProps {
  dag: LoopRunDagModel;
  selectedNodeId: string | null;
  /** Node, item and round together — the identity the roster reads by. */
  onSelect: (selection: LoopNodeSelection) => void;
}

/**
 * The run's shape, live.
 *
 * A read-only observability surface — no drag, no palette, no editing, and no
 * import from the builder canvas. The topology is drawn as a topological row
 * because that is what the graph is on a run page: a sequence you scan for the
 * one node that needs you.
 *
 * Liveness rides on the edges rather than the nodes, so the eye is pulled toward
 * the front of the run. Under `prefers-reduced-motion` the travelling dot is
 * removed from the render entirely — not paused mid-frame — while the live
 * edge keeps its accent fill, so liveness stays readable without animation.
 */
const EDGE_CLASS: Record<LoopDagEdgeState, string> = {
  live: "bg-accent",
  taken: "bg-success/60",
  idle: "bg-line",
  // Route evidence: the run went elsewhere. Dimmed, never dashed into invisibility.
  not_taken: "bg-line-soft",
};

/**
 * The gutter between two columns, carrying one line per authored edge that
 * crosses it. A column boundary with three edges shows three lines, so a
 * fan-out reads as a fan rather than as a single thread.
 */
function LoopDagGutter({ column, reduced }: { column: LoopDagColumn; reduced: boolean }) {
  return (
    <span
      aria-hidden="true"
      className="relative flex w-7 shrink-0 flex-col justify-center gap-1 self-stretch py-6"
      data-state={column.gutterState}
      data-testid={`loop-dag-edge-${column.gutterState}`}
    >
      {column.gutter.map(edge => (
        <span
          className={cn("h-px w-full", EDGE_CLASS[edge.state])}
          data-edge={edge.key}
          data-state={edge.state}
          key={edge.key}
        />
      ))}
      {column.gutterState === "live" && !reduced ? (
        // Unmounted, never paused: a frozen dot mid-edge reads as a stall, which
        // is the exact misreading this page exists to prevent. The accent fill
        // above still carries liveness when motion is off.
        <span className="absolute inset-0 flex items-center justify-center">
          <PillDot data-testid="loop-dag-edge-pulse" pulse tone="accent" />
        </span>
      ) : null}
    </span>
  );
}

function LoopDagCard({
  node,
  selected,
  onSelect,
}: {
  node: LoopDagNode;
  selected: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      aria-pressed={selected}
      className={cn(
        "flex w-40 shrink-0 flex-col gap-1.5 rounded-md border bg-canvas-tint px-3 py-2.5 text-left",
        "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent",
        node.chip.state === "pending" ? "border-dashed border-line" : "border-line",
        selected && "border-accent"
      )}
      data-node-id={node.nodeId}
      data-state={node.chip.state}
      data-testid={`loop-dag-node-${node.nodeId}`}
      onClick={onSelect}
      type="button"
    >
      <span className="flex min-w-0 items-center gap-1.5">
        {node.kindIcon
          ? createElement(node.kindIcon, {
              "aria-hidden": true,
              // The kind glyph stays neutral: a failed agent is still an agent.
              className: "size-3 shrink-0 text-muted",
            })
          : null}
        <span className="min-w-0 truncate text-small-body font-medium text-fg-strong">
          {node.nodeId}
        </span>
      </span>
      {node.fanOut ? (
        <span
          aria-label={node.fanOut.summary}
          className="flex h-1 gap-0.5"
          data-testid={`loop-dag-fanout-${node.nodeId}`}
          role="img"
        >
          {withOccurrenceKeys(node.fanOut.segments, segment => segment).map(
            ({ item: segment, key }) => (
              <span
                className={`flex-1 rounded-pill ${LOOP_PROGRESS_SEGMENT_CLASS[segment]}`}
                key={key}
              />
            )
          )}
        </span>
      ) : null}
      <span className="flex flex-wrap items-center gap-1.5">
        <LoopNodeStateChip chip={node.chip} />
        {node.fanOut ? (
          <span className="font-mono text-mono-id text-subtle">{node.fanOut.countLabel}</span>
        ) : null}
        {node.attemptLabel ? (
          <span className="font-mono text-mono-id text-faint">{node.attemptLabel}</span>
        ) : null}
      </span>
      {node.note ? (
        <span className="text-form-hint leading-snug text-faint">{node.note}</span>
      ) : null}
    </button>
  );
}

export function LoopRunDag({ dag, selectedNodeId, onSelect }: LoopRunDagProps) {
  const reduced = useReducedMotionConfig() === true;
  const laneRef = useRef<HTMLDivElement>(null);

  // Auto-centre on whatever needs a person. This is a scroll adjustment on an
  // external DOM node, which is what an effect is actually for.
  useEffect(() => {
    if (!dag.focusNodeId) return;
    const lane = laneRef.current;
    const target = lane?.querySelector<HTMLElement>(`[data-node-id="${dag.focusNodeId}"]`);
    if (!lane || !target) return;
    lane.scrollTo({
      left: target.offsetLeft - lane.clientWidth / 2 + target.clientWidth / 2,
      behavior: reduced ? "auto" : "smooth",
    });
  }, [dag.focusNodeId, reduced]);

  if (dag.empty) {
    return (
      <Empty
        data-testid="loop-dag-empty"
        description="This run's definition could not be read, so its shape cannot be drawn. The Nodes lane still lists every step."
        title="No readable graph"
      />
    );
  }

  return (
    <div
      className="flex items-stretch gap-0 overflow-x-auto px-4 py-4"
      data-testid="loop-run-dag"
      ref={laneRef}
    >
      {dag.columns.map((column, index) => (
        <div className="flex items-stretch" key={column.rank}>
          {/* Nodes sharing a column ran in parallel; stacking them says so. */}
          <div className="flex flex-col justify-center gap-2">
            {column.nodes.map(node => (
              <LoopDagCard
                key={node.key}
                node={node}
                onSelect={() =>
                  onSelect({ nodeId: node.nodeId, itemIndex: 0, generation: dag.round })
                }
                selected={selectedNodeId === node.nodeId}
              />
            ))}
          </div>
          {index < dag.columns.length - 1 ? (
            <LoopDagGutter column={column} reduced={reduced} />
          ) : null}
        </div>
      ))}
    </div>
  );
}
