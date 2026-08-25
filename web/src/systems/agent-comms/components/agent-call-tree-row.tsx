/**
 * One call in the Activity tree.
 *
 * A row keeps to five facts — who, what state, how the answer arrived, how big
 * it was, how long ago — and everything else lives one click deeper. That bound
 * is deliberate: ULIDs and contract digests are machine truth, and putting them
 * on every row turns a scannable list into a wall of hex. They belong on the
 * call detail, where the operator has asked for the record.
 */
import { OwnerAvatar, Time, cn } from "@compozy/ui";

import {
  AgentCallLiveness,
  AgentCallStatePill,
  AgentCallVerdictChip,
  AgentChildStatePill,
} from "./agent-call-state-pill";
import { toCallVerdict } from "../lib/call-state";
import type { CallTreeRow } from "../lib/agent-comms-tree";
import type { ChildState } from "../types";

/** Bytes as the daemon counted them — a record field, shown verbatim. */
function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const kib = bytes / 1024;
  if (kib < 1024) return `${kib.toFixed(kib < 10 ? 1 : 0)} KiB`;
  return `${(kib / 1024).toFixed(1)} MiB`;
}

/**
 * The trailing stat, or an em dash.
 *
 * Only two things earn this slot: the size of an answer that arrived, and the
 * count of issues on one that did not. A running call has neither, and an em
 * dash says so more honestly than a zero.
 */
function trailingStat(row: CallTreeRow): string {
  const { call } = row;
  if (row.state === "invalid-result") {
    const issues = call.repair_attempts + 1;
    return `${issues} ${issues === 1 ? "try" : "tries"}`;
  }
  if (row.state === "canceled" && call.failure_detail) return call.failure_detail;
  if (typeof call.result_bytes === "number") return formatBytes(call.result_bytes);
  return "—";
}

export interface AgentCallTreeRowProps {
  row: CallTreeRow;
  /** Indent is the daemon's own delegation depth, not a count of loaded parents. */
  depth: number;
  /**
   * What became of the helper this call was handed to.
   *
   * Absent until the session catalog for this tree is complete — a child is
   * only reported gone against a catalog that could have listed it.
   */
  childState?: ChildState | null;
  selected?: boolean;
  "data-testid"?: string;
}

export function AgentCallTreeRow({
  row,
  depth,
  childState = null,
  selected = false,
  "data-testid": testId,
}: AgentCallTreeRowProps) {
  const { call } = row;
  const agentName = call.agent ?? call.call_id;
  return (
    <span
      data-testid={testId}
      data-depth={depth}
      data-call-id={call.call_id}
      data-orphaned={row.orphaned || undefined}
      className={cn(
        "flex min-h-8 w-full items-center gap-2 rounded-sm px-2 text-small-body",
        selected && "bg-elevated text-fg-strong"
      )}
      style={{ paddingInlineStart: `calc(${depth} * var(--spacing) * 5)` }}
    >
      <OwnerAvatar ownerKind="agent" ownerId={agentName} size="sm" />
      <span className="truncate text-fg">{agentName}</span>
      <AgentCallStatePill state={row.state} fallbackLabel={call.state} />
      <AgentCallVerdictChip verdict={toCallVerdict(call.verdict)} />
      <AgentCallLiveness state={row.state} />
      {childState ? (
        <AgentChildStatePill data-testid="agent-call-tree-child-state" state={childState} />
      ) : null}
      <span className="flex-1" />
      <span className="shrink-0 font-mono text-form text-muted">{trailingStat(row)}</span>
      <Time iso={call.updated_at} className="shrink-0 text-form text-muted" />
    </span>
  );
}
