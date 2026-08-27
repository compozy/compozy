/**
 * The caller's side of a delegation, in the turn that made it.
 *
 * The transcript already holds the `compozy__agent_call` tool part, ordered by
 * the daemon among the turn's other work. This renders that part as a call card
 * instead of a generic tool row. Records arrive from the parent — this card
 * does not fetch.
 *
 * One invocation is one card, whether it started one call or twelve. A batch
 * that rendered as twelve sibling cards would turn a single "ask twelve helpers"
 * into twelve transcript entries and bury the conversation around it.
 */
import { AgentCallTurnCard } from "./agent-call-turn-card";
import { AgentCallTurnFanout } from "./agent-call-turn-fanout";
import type { AgentCallToolInvocation } from "../lib/agent-call-tool-parts";
import type { CallPayload } from "../types";

export interface AgentCallInvocationCardProps {
  invocation: AgentCallToolInvocation;
  calls: readonly CallPayload[];
  loading: boolean;
  onOpenCall: (callId: string) => void;
  onOpenCallsPanel?: () => void;
  "data-testid"?: string;
}

export function AgentCallInvocationCard({
  invocation,
  calls,
  loading,
  onOpenCall,
  onOpenCallsPanel,
  "data-testid": testId,
}: AgentCallInvocationCardProps) {
  // The tool has not returned yet, so there is no record to read. Saying the ask
  // is in flight is honest; rendering an empty card would not be.
  if (invocation.callIds.length === 0) {
    return (
      <p
        className="text-form text-muted"
        data-testid={testId}
        data-tool-call-id={invocation.toolCallId}
      >
        {invocation.pending ? "Asking a helper…" : "This call left no record."}
      </p>
    );
  }

  if (loading || calls.length !== invocation.callIds.length) {
    return (
      <p
        className="text-form text-muted"
        data-testid={testId}
        data-tool-call-id={invocation.toolCallId}
      >
        {loading ? "Loading call records…" : "The call records are unavailable."}
      </p>
    );
  }

  if (calls.length === 1) {
    return <AgentCallTurnCard call={calls[0]!} data-testid={testId} onOpenCall={onOpenCall} />;
  }

  return (
    <AgentCallTurnFanout
      calls={calls}
      data-testid={testId}
      onOpenCall={onOpenCall}
      {...(onOpenCallsPanel ? { onOpenCallsPanel } : {})}
    />
  );
}
