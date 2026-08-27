/**
 * Consecutive settled calls in one turn, stacked rather than folded away.
 */
import { AgentCallTurnCard } from "./agent-call-turn-card";
import type { CallPayload } from "../types";

export interface AgentCallTurnStackProps {
  calls: readonly CallPayload[];
  onOpenCall: (callId: string) => void;
  onOpenCallsPanel?: () => void;
  "data-testid"?: string;
}

export function AgentCallTurnStack({
  calls,
  onOpenCall,
  onOpenCallsPanel,
  "data-testid": testId,
}: AgentCallTurnStackProps) {
  if (calls.length === 0) return null;
  return (
    <div className="flex min-w-0 flex-col gap-0.5" data-testid={testId}>
      {calls.map(call => (
        <AgentCallTurnCard call={call} key={call.call_id} onOpenCall={onOpenCall} />
      ))}
      {onOpenCallsPanel ? (
        <p className="px-1 text-transcript-meta text-muted">
          {calls.length} settled calls ·{" "}
          <button
            className="underline-offset-4 hover:underline"
            onClick={onOpenCallsPanel}
            type="button"
          >
            see all in the Calls panel
          </button>
        </p>
      ) : null}
    </div>
  );
}
