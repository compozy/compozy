/**
 * A call as it appears inside the conversation that made it.
 *
 * Same row as every other tool: `ToolCallRow` + status chips + compact age.
 * The ask and the answer wait behind the row. Open call is muted and only
 * appears once the call is terminal.
 */
import { FileX } from "lucide-react";

import { OwnerAvatar, Time, ToolCallRow, type ToolCallStatus } from "@compozy/ui";

import {
  AgentCallLiveness,
  AgentCallStatePill,
  AgentCallVerdictChip,
} from "./agent-call-state-pill";
import {
  CALL_VERDICT_SIGNAL,
  isTerminalCallState,
  toCallState,
  toCallVerdict,
} from "../lib/call-state";
import type { CallPayload, CallState } from "../types";

function toolCallStatus(state: CallState): ToolCallStatus {
  switch (state) {
    case "queued":
    case "running":
      return "running";
    case "completed":
      return "success";
    case "invalid-result":
    case "completed-without-result":
    case "failed":
    case "expired":
      return "failed";
    case "canceled":
    case "timeout":
      return "empty";
  }
}

/** A one-line gist of the answer, or the daemon's own failure detail. */
function resultGist(call: CallPayload): string | null {
  if (call.result_preview !== undefined) {
    const encoded = JSON.stringify(call.result_preview);
    if (encoded !== undefined) {
      return encoded.length > 120 ? `${encoded.slice(0, 117)}…` : encoded;
    }
  }
  if (call.failure_detail) return call.failure_detail;
  return null;
}

export interface AgentCallTurnCardProps {
  call: CallPayload;
  /** The ask, once the projection or the tool args carry it. */
  prompt?: string | null;
  onOpenCall: (callId: string) => void;
  /** Stage the card open — used by stories and by a deep link into one call. */
  defaultOpen?: boolean;
  "data-testid"?: string;
}

export function AgentCallTurnCard({
  call,
  prompt = null,
  onOpenCall,
  defaultOpen = false,
  "data-testid": testId,
}: AgentCallTurnCardProps) {
  const state = toCallState(call.state);
  const verdict = toCallVerdict(call.verdict);
  const agentName = call.agent ?? call.call_id;
  const ask = prompt ?? call.prompt_preview ?? null;
  const terminal = state !== null && isTerminalCallState(state);
  const gist = terminal ? resultGist(call) : null;
  const age = call.started_at ?? call.updated_at;
  const extracted = verdict === "extracted";
  const invalid = state === "invalid-result";
  const hasBody =
    Boolean(ask) || gist !== null || invalid || extracted || (terminal && Boolean(onOpenCall));

  return (
    <ToolCallRow
      data-call-id={call.call_id}
      data-testid={testId}
      defaultExpanded={defaultOpen}
      icon={<OwnerAvatar ownerId={agentName} ownerKind="agent" size="sm" />}
      stat={age ? <Time iso={age} mode="compact" /> : undefined}
      status={state === null ? "empty" : toolCallStatus(state)}
      statusSlot={
        <span className="flex items-center gap-1">
          <AgentCallStatePill fallbackLabel={call.state} state={state} />
          <AgentCallVerdictChip verdict={verdict} />
          <AgentCallLiveness state={state} />
        </span>
      }
      toolName={
        <>
          Asked <span className="font-medium text-fg">{agentName}</span>
        </>
      }
    >
      {hasBody ? (
        <>
          {ask ? <p className="text-small-body text-fg">{ask}</p> : null}
          {gist !== null ? (
            <p className="truncate font-mono text-transcript-meta text-muted">{gist}</p>
          ) : null}
          {invalid ? (
            <div className="flex items-start gap-1.5 text-danger">
              <FileX aria-hidden="true" className="size-3 shrink-0" />
              <div className="min-w-0">
                <p className="text-small-body">The answer didn't match what was asked…</p>
                {call.failure_code ? (
                  <p className="font-mono text-transcript-caption">{call.failure_code}</p>
                ) : null}
              </div>
            </div>
          ) : null}
          {extracted ? (
            <p className="text-transcript-meta text-muted">
              {CALL_VERDICT_SIGNAL.extracted.description}
            </p>
          ) : null}
          {terminal ? (
            <button
              className="text-transcript-meta text-muted underline-offset-4 hover:underline"
              onClick={() => onOpenCall(call.call_id)}
              type="button"
            >
              Open call
            </button>
          ) : null}
        </>
      ) : null}
    </ToolCallRow>
  );
}
