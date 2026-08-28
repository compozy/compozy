/**
 * The inspector's Calls tab — this session's delegations, both directions.
 *
 * A thin connected edge: the queries and the counting live in
 * `useSessionCallsPanel`, so this file only supplies navigation and hands the
 * panel a finished view model.
 */
import { useNavigate } from "@tanstack/react-router";

import { AgentCallsInspectorPanel } from "@/systems/agent-comms";

import { useSessionCallsPanel } from "../hooks/use-session-calls-panel";

export interface SessionCallsSectionProps {
  sessionId?: string;
  liveDataEnabled?: boolean;
}

export function SessionCallsSection({
  sessionId,
  liveDataEnabled = true,
}: SessionCallsSectionProps) {
  const navigate = useNavigate();
  const panel = useSessionCallsPanel(sessionId ?? "", liveDataEnabled);

  return (
    <AgentCallsInspectorPanel
      data-testid="session-inspector-calls"
      callerNames={panel.callerNames}
      made={panel.made}
      onOpenCall={callId => {
        void navigate({ to: "/agents/calls/$callId", params: { callId } });
      }}
      prunedSessionIds={panel.prunedSessionIds}
      received={panel.received}
    />
  );
}
