/**
 * One call in the Activity tree.
 *
 * A row keeps to five facts — who, what state, how the answer arrived, how big
 * it was, how long ago — and everything else lives one click deeper. That bound
 * is deliberate: ULIDs, contract digests, and child lifecycle belong on the
 * call record, not on the default tree.
 */
import { OwnerAvatar, Time, cn } from "@compozy/ui";

import {
  AgentCallStatePill,
  AgentCallVerdictChip,
  AgentChildStatePill,
} from "./agent-call-state-pill";
import { toCallVerdict } from "../lib/call-state";
import type { CallTreeRow } from "../lib/agent-comms-tree";
import { formatAgentCallBytes } from "../lib/format-bytes";
import type { ChildState } from "../types";

/** Visual indent follows the daemon depth and stops at the designed third step. */
const MAX_TREE_INDENT = 3;

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
    return `${issues} ${issues === 1 ? "issue" : "issues"}`;
  }
  if (row.state === "canceled" && call.failure_detail) return call.failure_detail;
  if (typeof call.result_bytes === "number") return formatAgentCallBytes(call.result_bytes);
  return "—";
}

export interface AgentCallTreeRowProps {
  row: CallTreeRow;
  /** Indent is the daemon's own delegation depth, not a count of loaded parents. */
  depth: number;
  selected?: boolean;
  childState?: ChildState;
  "data-testid"?: string;
}

export function AgentCallTreeRow({
  row,
  depth,
  selected = false,
  childState,
  "data-testid": testId,
}: AgentCallTreeRowProps) {
  const { call } = row;
  const agentName = call.agent ?? call.call_id;
  const stat = trailingStat(row);
  const indentDepth = Math.min(depth, MAX_TREE_INDENT);
  return (
    <span
      data-testid={testId}
      data-depth={depth}
      data-call-id={call.call_id}
      data-orphaned={row.orphaned || undefined}
      className={cn(
        "flex min-h-8 w-full items-center gap-2 rounded-sm px-2 text-small-body",
        selected && "bg-row-selected text-fg-strong"
      )}
      style={{ paddingInlineStart: `calc(${indentDepth} * var(--spacing) * 5)` }}
    >
      <OwnerAvatar ownerKind="agent" ownerId={agentName} size="sm" />
      <span className="truncate text-fg">{agentName}</span>
      <AgentCallStatePill state={row.state} fallbackLabel={call.state} />
      <AgentCallVerdictChip verdict={toCallVerdict(call.verdict)} />
      {childState ? (
        <AgentChildStatePill state={childState} data-testid="agent-call-tree-child-state" />
      ) : null}
      <span className="flex-1" />
      <span className="max-w-48 shrink-0 truncate font-mono text-form text-muted" title={stat}>
        {stat}
      </span>
      <Time iso={call.updated_at} mode="compact" className="shrink-0 text-form text-muted" />
    </span>
  );
}
