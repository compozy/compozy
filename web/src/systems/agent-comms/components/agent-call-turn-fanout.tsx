/**
 * Many calls at once, as one row.
 *
 * A parallel fan-out is one act. The worst state escalates to the head using
 * the same chip dictionary every other row uses. Expanded, a capped list;
 * the Calls panel owns the rest.
 */
import { OwnerAvatar, Pill, Time, ToolCallRow } from "@compozy/ui";

import { AgentCallStatePill } from "./agent-call-state-pill";
import { escalateCallPayloads } from "../lib/agent-comms-tree";
import { toCallState } from "../lib/call-state";
import type { CallPayload } from "../types";

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

function headAge(calls: readonly CallPayload[]): string | undefined {
  let earliest: string | undefined;
  for (const call of calls) {
    const iso = call.started_at ?? call.updated_at;
    if (iso && (earliest === undefined || iso < earliest)) earliest = iso;
  }
  return earliest;
}

function fanoutStatus(
  escalation: ReturnType<typeof escalateCallPayloads>
): "running" | "success" | "failed" | "empty" {
  if (escalation === "running" || escalation === "queued") return "running";
  if (escalation === "completed") return "success";
  if (
    escalation === "invalid-result" ||
    escalation === "completed-without-result" ||
    escalation === "failed" ||
    escalation === "expired"
  ) {
    return "failed";
  }
  return "empty";
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
  const lead = calls[0];
  const overflow = calls.length - 1;
  const shown = onOpenCallsPanel ? calls.slice(0, MAX_EXPANDED_ROWS) : calls;
  const hidden = calls.length - shown.length;
  const age = headAge(calls);
  const counts = tally(calls);

  return (
    <ToolCallRow
      data-testid={testId}
      defaultExpanded={defaultOpen}
      icon={
        <span className="relative z-10">
          <OwnerAvatar
            ownerId={lead?.agent ?? lead?.call_id ?? "agent"}
            ownerKind="agent"
            size="sm"
          />
        </span>
      }
      preview={counts}
      stat={age ? <Time iso={age} mode="compact" /> : undefined}
      status={fanoutStatus(escalation)}
      statusSlot={
        <span className="flex items-center gap-1">
          {overflow > 0 ? (
            <Pill mono size="xs" tone="neutral">
              +{overflow}
            </Pill>
          ) : null}
          {escalation !== null ? <AgentCallStatePill state={escalation} /> : null}
        </span>
      }
      toolName={`Asked ${String(calls.length)} agents`}
    >
      <ul className="flex flex-col">
        {shown.map(call => {
          const agentName = call.agent ?? call.call_id;
          const rowAge = call.started_at ?? call.updated_at;
          return (
            <li className="flex items-center gap-2 py-0.5" key={call.call_id}>
              <OwnerAvatar ownerId={agentName} ownerKind="agent" size="sm" />
              <button
                className="min-w-0 flex-1 truncate text-left text-transcript-meta text-fg hover:underline"
                onClick={() => onOpenCall(call.call_id)}
                type="button"
              >
                {agentName}
              </button>
              <AgentCallStatePill fallbackLabel={call.state} state={toCallState(call.state)} />
              {rowAge ? <Time className="shrink-0 text-muted" iso={rowAge} mode="compact" /> : null}
            </li>
          );
        })}
      </ul>
      {onOpenCallsPanel ? (
        <button
          className="text-transcript-meta text-muted underline-offset-4 hover:underline"
          onClick={onOpenCallsPanel}
          type="button"
        >
          {hidden > 0
            ? `${String(hidden)} more · see all in the Calls panel`
            : "see all in the Calls panel"}
        </button>
      ) : null}
    </ToolCallRow>
  );
}
