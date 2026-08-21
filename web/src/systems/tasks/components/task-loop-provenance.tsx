import { Link } from "@tanstack/react-router";
import { ArrowUpRight } from "lucide-react";

import { MonoId, PropertyRow } from "@compozy/ui";

import {
  RUN_GONE_LABEL,
  taskLoopRoleLabel,
  taskLoopRunLink,
  type TaskLoopProvenance as TaskLoopProvenanceData,
} from "../lib/task-loop-identity";
import { TaskRailSection } from "./task-rail-section";

export interface TaskLoopProvenanceProps {
  loop: TaskLoopProvenanceData;
}

/**
 * Loop provenance for a revealed execution record's detail page (S3, US-015.AC-2).
 *
 * This page is the mechanism view; the run page is where the work is acted on,
 * so the link back is mandatory and never depends on having arrived from a list.
 * Every field is projected provenance — nothing here is recovered from the task
 * id (ADR-004).
 */
export function TaskLoopProvenance({ loop }: TaskLoopProvenanceProps) {
  const runLink = taskLoopRunLink(loop);
  const nodeId = loop.node_id?.trim();

  // The heading names what the record is, so no row repeats it back.
  return (
    <TaskRailSection data-testid="task-loop-provenance" label={taskLoopRoleLabel(loop)}>
      {loop.loop_name ? <PropertyRow label="Loop">{loop.loop_name}</PropertyRow> : null}
      <PropertyRow
        editor={
          runLink ? (
            <Link
              className="inline-flex min-h-6 min-w-0 items-center gap-1 rounded-sm px-1.5 py-0.5 text-small-body font-medium text-fg hover:bg-row-hover focus-visible:outline-none focus-visible:shadow-focus-ring"
              data-testid="task-loop-provenance-open-run"
              params={runLink.params}
              to={runLink.to}
            >
              <span className="truncate">Open run</span>
              <ArrowUpRight aria-hidden="true" className="size-3" />
            </Link>
          ) : (
            <span className="text-muted" data-testid="task-loop-provenance-run-gone">
              {RUN_GONE_LABEL}
            </span>
          )
        }
        label="Run"
      />
      {typeof loop.generation === "number" ? (
        <PropertyRow label="Round">{loop.generation}</PropertyRow>
      ) : null}
      {nodeId ? <PropertyRow label="Step">{nodeId}</PropertyRow> : null}
      {typeof loop.item_index === "number" ? (
        <PropertyRow label="Item">{loop.item_index}</PropertyRow>
      ) : null}
      <PropertyRow label="Run id" mono>
        <MonoId value={loop.run_id} />
      </PropertyRow>
    </TaskRailSection>
  );
}
