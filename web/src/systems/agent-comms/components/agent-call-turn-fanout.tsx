/**
 * Many calls at once, as one card.
 *
 * A parallel fan-out is one act, not N. Rendering a sibling card per call would
 * turn a single "ask twelve helpers" into twelve transcript entries and bury the
 * conversation. So the batch collapses into one card: overlapping identity
 * chips, a live tally, and — the part that matters — **the worst state escalated
 * to the head as an ordinary state chip**, using the same dictionary every other
 * row uses. No new vocabulary for batches.
 *
 * Expanded, it shows a capped list and hands off to the Calls panel, which owns
 * the full enumeration. The transcript never becomes the list.
 */
import { ChevronRight } from "lucide-react";

import { OwnerAvatar, Time } from "@compozy/ui";

import { AgentCallStatePill } from "./agent-call-state-pill";
import { escalateCallPayloads } from "../lib/agent-comms-tree";
import { toCallState } from "../lib/call-state";
import type { CallPayload } from "../types";

/** Identity chips shown before the "+N" overflow. */
const MAX_IDENTITY_CHIPS = 3;

/** Rows shown when expanded. The Calls panel owns the rest. */
const MAX_EXPANDED_ROWS = 6;

/** "9 done · 2 running · 1 needs a look", with zero clauses omitted. */
function tally(calls: readonly CallPayload[]): string {
  let done = 0;
  let running = 0;
  let needsYou = 0;
  for (const call of calls) {
    const state = toCallState(call.state);
    if (state === "completed") done += 1;
    else if (state === "running" || state === "queued") running += 1;
    else if (state === "invalid-result" || state === "completed-without-result") needsYou += 1;
  }
  const parts: string[] = [];
  if (done > 0) parts.push(`${done} done`);
  if (running > 0) parts.push(`${running} running`);
  if (needsYou > 0) parts.push(`${needsYou} needs a look`);
  return parts.join(" · ");
}

export interface AgentCallTurnFanoutProps {
  calls: readonly CallPayload[];
  onOpenCall: (callId: string) => void;
  /** Opens the Calls panel, which owns the full list. */
  onOpenCallsPanel?: () => void;
  defaultOpen?: boolean;
  "data-testid"?: string;
}

export function AgentCallTurnFanout({
  calls,
  onOpenCall,
  onOpenCallsPanel,
  defaultOpen = false,
  "data-testid": testId,
}: AgentCallTurnFanoutProps) {
  if (calls.length === 0) return null;

  const escalation = escalateCallPayloads(calls);
  const chips = calls.slice(0, MAX_IDENTITY_CHIPS);
  const overflow = calls.length - chips.length;
  // Without a Calls-panel destination, every accepted record stays reachable
  // here. Capping the rows while rendering no working handoff would hide data.
  const shown = onOpenCallsPanel ? calls.slice(0, MAX_EXPANDED_ROWS) : calls;
  const hidden = calls.length - shown.length;

  return (
    <details
      data-testid={testId}
      open={defaultOpen || undefined}
      className="rounded-md border border-line-soft bg-canvas-soft"
    >
      <summary className="flex min-h-8.5 cursor-pointer list-none items-center gap-2 px-2.5 [&::-webkit-details-marker]:hidden">
        <ChevronRight
          className="size-3 shrink-0 text-muted transition-transform in-[details[open]]:rotate-90"
          aria-hidden="true"
        />
        <span className="flex -space-x-1.5">
          {chips.map(call => (
            <OwnerAvatar
              key={call.call_id}
              ownerKind="agent"
              ownerId={call.agent ?? call.call_id}
              size="sm"
            />
          ))}
        </span>
        {overflow > 0 ? <span className="text-form text-muted">+{overflow}</span> : null}
        <span className="text-small-body text-fg">Asked {calls.length} helpers</span>
        {escalation !== null ? <AgentCallStatePill state={escalation} /> : null}
        <span className="flex-1" />
        <span className="shrink-0 text-form text-muted">{tally(calls)}</span>
      </summary>

      <ul className="flex flex-col border-t border-line-soft">
        {shown.map(call => {
          const agentName = call.agent ?? call.call_id;
          return (
            <li key={call.call_id} className="flex items-center gap-2 px-2.5 py-1">
              <OwnerAvatar ownerKind="agent" ownerId={agentName} size="sm" />
              <button
                type="button"
                className="min-w-0 flex-1 truncate text-left text-form text-fg hover:underline"
                onClick={() => onOpenCall(call.call_id)}
              >
                {agentName}
              </button>
              <AgentCallStatePill state={toCallState(call.state)} fallbackLabel={call.state} />
              <Time iso={call.updated_at} className="shrink-0 text-form text-muted" />
            </li>
          );
        })}
      </ul>

      {hidden > 0 && onOpenCallsPanel ? (
        <div className="border-t border-line-soft px-2.5 py-1.5">
          <button
            type="button"
            className="text-form text-accent underline-offset-4 hover:underline"
            onClick={onOpenCallsPanel}
          >
            {hidden} more in the Calls panel
          </button>
        </div>
      ) : null}
    </details>
  );
}
