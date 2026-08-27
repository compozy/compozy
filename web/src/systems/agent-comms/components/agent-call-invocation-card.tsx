/**
 * The caller's side of a delegation, in the turn that made it.
 *
 * One invocation is one card, whether it started one call or twelve. While the
 * tool is still open, the ask is already known from the tool args — that is
 * enough to render an honest asked row.
 */
import { OwnerAvatar, ToolCallRow } from "@compozy/ui";

import { AgentCallLiveness, AgentCallStatePill } from "./agent-call-state-pill";
import { AgentCallTurnCard } from "./agent-call-turn-card";
import { AgentCallTurnFanout } from "./agent-call-turn-fanout";
import { agentCallArgsFromTool, type AgentCallToolInvocation } from "../lib/agent-call-tool-parts";
import type { CallPayload } from "../types";

export interface AgentCallInvocationCardProps {
  invocation: AgentCallToolInvocation;
  calls: readonly CallPayload[];
  loading: boolean;
  args?: Record<string, unknown>;
  onOpenCall: (callId: string) => void;
  onOpenCallsPanel?: () => void;
  "data-testid"?: string;
}

function AgentCallPendingRow({
  agent,
  prompt,
  testId,
}: {
  agent: string | null;
  prompt: string | null;
  testId?: string;
}) {
  const name = agent ?? "an agent";
  return (
    <ToolCallRow
      data-testid={testId}
      icon={<OwnerAvatar ownerId={name} ownerKind="agent" size="sm" />}
      preview={prompt}
      status="running"
      statusSlot={
        <span className="flex items-center gap-1">
          <AgentCallStatePill state="running" />
          <AgentCallLiveness state="running" />
        </span>
      }
      toolName={
        <>
          Asked <span className="font-medium text-fg">{name}</span>
        </>
      }
    />
  );
}

export function AgentCallInvocationCard({
  invocation,
  calls,
  loading,
  args,
  onOpenCall,
  onOpenCallsPanel,
  "data-testid": testId,
}: AgentCallInvocationCardProps) {
  const parsed = agentCallArgsFromTool(args);

  if (invocation.callIds.length === 0 || (loading && calls.length === 0)) {
    return <AgentCallPendingRow agent={parsed.agent} prompt={parsed.prompt} testId={testId} />;
  }

  if (calls.length === 1) {
    return (
      <AgentCallTurnCard
        call={calls[0]!}
        data-testid={testId}
        onOpenCall={onOpenCall}
        prompt={calls[0]!.prompt_preview ?? parsed.prompt}
      />
    );
  }

  if (calls.length > 1) {
    return (
      <AgentCallTurnFanout
        calls={calls}
        data-testid={testId}
        onOpenCall={onOpenCall}
        {...(onOpenCallsPanel ? { onOpenCallsPanel } : {})}
      />
    );
  }

  return <AgentCallPendingRow agent={parsed.agent} prompt={parsed.prompt} testId={testId} />;
}
