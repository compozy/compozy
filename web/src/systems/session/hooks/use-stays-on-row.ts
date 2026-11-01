import { useNavigate } from "@tanstack/react-router";

import { useAgentInstanceActions, type AgentInstancePayload } from "@/systems/agent";

import type { SessionListTarget } from "../types";

/** The verbs an avatar offers. Its one click opens the main session, created again if it was lost. */
export function useStaysOnRowActions(onOpenSession: (session: SessionListTarget) => void) {
  const navigate = useNavigate();
  const actions = useAgentInstanceActions();
  const open = async (instance: AgentInstancePayload) => {
    const current = instance.main_session_id
      ? instance
      : await actions.ensureMain(instance.id).catch(() => null);
    if (!current?.main_session_id) return;
    onOpenSession({
      id: current.main_session_id,
      agent_name: current.agent_name,
      workspace_id: current.workspace_id,
    });
  };
  return {
    open,
    togglePause: (instance: AgentInstancePayload) =>
      instance.state === "paused" ? actions.resume(instance.id) : actions.pause(instance.id),
    archive: (instance: AgentInstancePayload) => actions.archive(instance.id),
    openAgent: (instance: AgentInstancePayload) =>
      navigate({ to: "/agents/$name", params: { name: instance.agent_name } }),
    // The agent's page hosts Check-ins until its focused editor ships.
    openCheckIns: (instance: AgentInstancePayload) =>
      navigate({ to: "/agents/$name", params: { name: instance.agent_name } }),
  };
}

export type StaysOnRowActions = ReturnType<typeof useStaysOnRowActions>;
