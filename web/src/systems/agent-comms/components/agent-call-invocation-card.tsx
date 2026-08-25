/**
 * The caller's side of a delegation, in the turn that made it.
 *
 * The transcript already holds the `compozy__agent_call` tool part, ordered by
 * the daemon among the turn's other work. This renders that part as a call card
 * instead of a generic tool row, and reads the call records live by id — because
 * the tool's own result was written at acceptance and says `running` forever.
 *
 * One invocation is one card, whether it started one call or twelve. A batch
 * that rendered as twelve sibling cards would turn a single "ask twelve helpers"
 * into twelve transcript entries and bury the conversation around it.
 */
import { AgentCallTurnCard } from "./agent-call-turn-card";
import { AgentCallTurnFanout } from "./agent-call-turn-fanout";
import { useCallsById } from "../hooks/use-calls-by-id";
import type { AgentCallToolInvocation } from "../lib/agent-call-tool-parts";

export interface AgentCallInvocationCardProps {
  invocation: AgentCallToolInvocation;
  live?: boolean;
  onOpenCall: (callId: string) => void;
  onOpenCallsPanel?: () => void;
  "data-testid"?: string;
}

export function AgentCallInvocationCard({
  invocation,
  live = false,
  onOpenCall,
  onOpenCallsPanel,
  "data-testid": testId,
}: AgentCallInvocationCardProps) {
  const { calls } = useCallsById(invocation.callIds, live);

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

  if (calls.length === 0) return null;

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
