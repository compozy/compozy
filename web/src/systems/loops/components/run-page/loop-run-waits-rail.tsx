import { Link } from "@tanstack/react-router";
import { Bell } from "lucide-react";

import { Eyebrow } from "@compozy/ui";

import { isOpenWait, type LoopNodeLifecycle } from "../../lib/loop-node-lifecycle";
import type { LoopNodeInventoryState } from "../../types";

interface LoopRunWaitsRailProps {
  /** Every node the daemon reports lifecycle state for, from the run detail. */
  nodes: readonly LoopNodeLifecycle[];
  runId: string;
}

/**
 * The rail's "Waits & attention" panel (§2 rail anatomy). Three counts, each a
 * direct tally of this run's own `node_controls[]` / `waits[]` — never a
 * workspace-wide figure, and never an estimate.
 *
 * A zero renders as a plain `0` rather than disappearing: unlike a badge, this
 * panel answers "is anything parked?", and the answer "nothing" is information
 * the operator came here for (DESIGN-LESSONS L5 governs badges, not readouts).
 */
export function LoopRunWaitsRail({ nodes, runId }: LoopRunWaitsRailProps) {
  let waiting = 0;
  let attention = 0;
  let quarantined = 0;
  for (const node of nodes) {
    if (node.waits.some(isOpenWait)) waiting += 1;
    if (node.attentionFlag !== "") attention += 1;
    if (node.quarantined) quarantined += 1;
  }
  const rows: {
    label: string;
    value: number;
    nodes: LoopNodeInventoryState;
    nodeId?: string;
  }[] = [
    {
      label: "Open waits",
      value: waiting,
      nodes: "waiting",
      nodeId: waiting === 1 ? nodes.find(node => node.waits.some(isOpenWait))?.nodeId : undefined,
    },
    {
      label: "Needs attention",
      value: attention,
      nodes: "attention",
      nodeId: attention === 1 ? nodes.find(node => node.attentionFlag !== "")?.nodeId : undefined,
    },
    {
      label: "Quarantined",
      value: quarantined,
      nodes: "quarantined",
      nodeId: quarantined === 1 ? nodes.find(node => node.quarantined)?.nodeId : undefined,
    },
  ];
  return (
    <div className="border-t border-line-soft px-3 py-3" data-testid="loop-run-waits-rail">
      <div className="mb-2 flex items-center gap-1.5">
        <Bell aria-hidden="true" className="size-3 text-muted" />
        <Eyebrow className="text-muted">Waits &amp; attention</Eyebrow>
      </div>
      <dl className="flex flex-col gap-1.5">
        {rows.map(row => (
          <div className="flex items-baseline justify-between gap-3" key={row.label}>
            <dt className="text-small-body text-muted">{row.label}</dt>
            <dd
              className={`font-mono text-mono-id tabular-nums ${
                row.value > 0 ? "text-warning" : "text-subtle"
              }`}
              data-testid={`loop-run-waits-${row.label.toLowerCase().replaceAll(" ", "-")}`}
            >
              {row.value > 0 ? (
                <Link
                  className="text-warning hover:text-fg-strong"
                  search={{ nodes: row.nodes, nodes_run: runId }}
                  to="/loop-runs"
                >
                  {row.value}
                </Link>
              ) : (
                row.value
              )}
              {row.nodeId ? <span className="ml-1.5 text-faint">· {row.nodeId}</span> : null}
            </dd>
          </div>
        ))}
      </dl>
    </div>
  );
}
