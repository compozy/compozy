/**
 * The header for one delegation tree.
 *
 * Two jobs, and the second is the important one:
 *
 * 1. Identify the root session and say how big the tree is.
 * 2. **Escalate.** A folded tree carries its worst state up here — tone, glyph,
 *    and the exact state word together — so folding can never hide a problem.
 *    That is the whole reason folding is safe to offer at all.
 *
 * The counts come from the daemon's `total` for this root, not from the rows
 * that happen to be loaded, so a paged tree still reports its real size.
 */
import { Square } from "lucide-react";

import { Button, MonoId, OwnerAvatar } from "@compozy/ui";

import { AgentCallStatePill } from "./agent-call-state-pill";
import type { CallState } from "../types";

export interface AgentCallTreeRootRowProps {
  rootSessionId: string;
  /** Human name for the root session when known, else the id stands alone. */
  rootLabel?: string | null;
  /** Daemon count for this tree. Undefined while the probe is in flight. */
  totalCalls: number | undefined;
  /** Daemon count of calls still working in this tree. */
  runningCalls: number | undefined;
  /** Daemon count of calls in this tree that need a look. */
  needsYouCalls: number | undefined;
  /** Worst state in the tree — rendered even while folded. */
  escalation: CallState | null;
  /** Absent when the operator cannot drain this tree. */
  onStopSubtree?: () => void;
  stopPending?: boolean;
  "data-testid"?: string;
}

/**
 * "3 calls · 2 running · 1 needs a look" — with each clause dropped when its
 * count is zero or still unknown. A zero never renders: "0 running" is noise
 * that reads like a fact.
 */
function summaryText(
  totalCalls: number | undefined,
  runningCalls: number | undefined,
  needsYouCalls: number | undefined
): string | null {
  const parts: string[] = [];
  if (totalCalls !== undefined) {
    parts.push(`${totalCalls} ${totalCalls === 1 ? "call" : "calls"}`);
  }
  if (runningCalls !== undefined && runningCalls > 0) parts.push(`${runningCalls} running`);
  if (needsYouCalls !== undefined && needsYouCalls > 0) {
    parts.push(`${needsYouCalls} ${needsYouCalls === 1 ? "needs" : "need"} a look`);
  }
  return parts.length > 0 ? parts.join(" · ") : null;
}

export function AgentCallTreeRootRow({
  rootSessionId,
  rootLabel,
  totalCalls,
  runningCalls,
  needsYouCalls,
  escalation,
  onStopSubtree,
  stopPending = false,
  "data-testid": testId,
}: AgentCallTreeRootRowProps) {
  const label = rootLabel?.trim() || rootSessionId;
  const summary = summaryText(totalCalls, runningCalls, needsYouCalls);
  return (
    <span
      data-testid={testId}
      data-root-session-id={rootSessionId}
      className="flex min-h-8 w-full items-center gap-2 text-small-body"
    >
      <OwnerAvatar ownerKind="agent" ownerId={rootSessionId} name={label} size="sm" />
      <span className="truncate font-medium text-fg-strong">{label}</span>
      <MonoId value={rootSessionId} className="shrink-0 text-muted" />
      <span className="flex-1" />
      {escalation !== null ? <AgentCallStatePill state={escalation} /> : null}
      {summary ? <span className="shrink-0 text-form text-muted">{summary}</span> : null}
      {onStopSubtree ? (
        <Button
          size="xs"
          variant="outline"
          type="button"
          disabled={stopPending}
          onClick={event => {
            // The header is a tree row; draining must not also toggle it.
            event.stopPropagation();
            onStopSubtree();
          }}
        >
          <Square aria-hidden="true" />
          Stop subtree
        </Button>
      ) : null}
    </span>
  );
}
