/**
 * Transcript adapters for a delegation tool part.
 *
 * Navigation and inspector-tab requests stay here so the agent-comms cards
 * stay presentational.
 */
import { useNavigate } from "@tanstack/react-router";

import {
  AgentCallInvocationCard,
  AgentCallReturnTurn,
  AgentCallTurnStack,
  callIdsFromToolResult,
  useCallsById,
  verdictFromReturnResult,
} from "@/systems/agent-comms";
import { useOpenSessionCallsPanel } from "./hooks/use-open-session-calls-panel";
import type { SessionTimelineToolPart } from "./session-timeline.logic";

export function SessionCallInvocation({
  tool,
  turnSettled,
}: {
  tool: SessionTimelineToolPart;
  turnSettled: boolean;
}) {
  const navigate = useNavigate();
  const onOpenCallsPanel = useOpenSessionCallsPanel();
  const callIds = callIdsFromToolResult(tool.result);
  const { calls, loading } = useCallsById(callIds, !turnSettled);
  return (
    <AgentCallInvocationCard
      args={tool.args}
      calls={calls}
      data-testid="session-call-invocation"
      invocation={{
        toolCallId: tool.toolCallId,
        callIds,
        pending: tool.status === "running",
      }}
      loading={loading}
      onOpenCall={callId => {
        void navigate({ to: "/agents/calls/$callId", params: { callId } });
      }}
      onOpenCallsPanel={onOpenCallsPanel}
    />
  );
}

export function SessionSettledCallStack({ tools }: { tools: readonly SessionTimelineToolPart[] }) {
  const navigate = useNavigate();
  const onOpenCallsPanel = useOpenSessionCallsPanel();
  const callIds = tools.flatMap(tool => callIdsFromToolResult(tool.result));
  const { calls, loading } = useCallsById(callIds, false);
  if (loading && calls.length === 0) {
    return (
      <div className="flex min-w-0 flex-col gap-0.5" data-testid="session-call-stack">
        {tools.map(tool => (
          <SessionCallInvocation key={tool.id} tool={tool} turnSettled />
        ))}
      </div>
    );
  }
  return (
    <AgentCallTurnStack
      calls={calls}
      data-testid="session-call-stack"
      onOpenCall={callId => {
        void navigate({ to: "/agents/calls/$callId", params: { callId } });
      }}
      onOpenCallsPanel={onOpenCallsPanel}
    />
  );
}

export function SessionCallReturnRow({ tool }: { tool: SessionTimelineToolPart }) {
  return (
    <AgentCallReturnTurn
      callerName="the caller"
      data-testid="session-call-return"
      verdict={verdictFromReturnResult(tool.result)}
    />
  );
}
