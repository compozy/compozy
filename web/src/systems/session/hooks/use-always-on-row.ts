import { useNavigate } from "@tanstack/react-router";

import {
  agentInstanceBinding,
  useAgentInstanceActions,
  type AgentInstancePayload,
} from "@/systems/agent";

import { alwaysOnOwnerOf } from "../lib/always-on-owner";
import type { SessionListTarget } from "../types";
import { useSessionLineage } from "./use-sessions";

/**
 * Which of these agents the open session belongs to: its main session, or a side chat, child, or
 * subagent under it, read from the open session's lineage. A main session needs no read.
 */
export function useAlwaysOnCurrentId(
  instances: readonly AgentInstancePayload[],
  currentSessionId: string | null | undefined
): string | undefined {
  const id = currentSessionId?.trim() ?? "";
  const main = instances.find(instance => id !== "" && instance.main_session_id === id);
  const session = useSessionLineage(id, id !== "" && main === undefined);
  if (main) return main.id;
  if (id === "" || session?.id !== id) return undefined;
  return alwaysOnOwnerOf(instances, session)?.id;
}

/** The verbs an avatar offers. Its one click opens the main session, created again if it was lost. */
export function useAlwaysOnRowActions(onOpenSession: (session: SessionListTarget) => void) {
  const navigate = useNavigate();
  const actions = useAgentInstanceActions();
  const open = async (instance: AgentInstancePayload) => {
    const current = instance.main_session_id
      ? instance
      : await actions.ensureMain(instance).catch(() => null);
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
      instance.state === "paused" ? actions.resume(instance) : actions.pause(instance),
    archive: (instance: AgentInstancePayload) => actions.archive(instance),
    openAgent: (instance: AgentInstancePayload) =>
      navigate({
        to: "/agents/$name",
        params: { name: instance.agent_name },
        search: agentInstanceBinding(instance),
      }),
    openCheckIns: (instance: AgentInstancePayload) =>
      navigate({
        to: "/agents/$name/$page",
        params: { name: instance.agent_name, page: "check-ins" },
        search: agentInstanceBinding(instance),
      }),
  };
}

export type AlwaysOnRowActions = ReturnType<typeof useAlwaysOnRowActions>;
