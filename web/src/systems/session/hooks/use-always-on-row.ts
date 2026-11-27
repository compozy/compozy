import { useNavigate } from "@tanstack/react-router";

import { isRemoteComputerId, normalizeComputerId, type ComputerId } from "@/lib/computer-scope";
import { useComputers } from "@/systems/computers";

import {
  agentInstanceBinding,
  useAgentInstanceActions,
  type AgentInstancePayload,
} from "@/systems/agent";

import { parseSessionRowKey } from "../lib/session-row-ref";
import { alwaysOnOwnerOf } from "../lib/always-on-owner";
import type { SessionListTarget } from "../types";
import { useSessionLineage } from "./use-sessions";

/**
 * Which of these agents the open session belongs to: its main session, or a side chat, child, or
 * subagent under it, read from the open session's lineage. A main session needs no read.
 */
export function useAlwaysOnCurrentId(
  instances: readonly AgentInstancePayload[],
  currentSessionId: string | null | undefined,
  computerId?: ComputerId
): string | undefined {
  const current = parseSessionRowKey(currentSessionId?.trim() ?? "");
  const id = current.computerId === normalizeComputerId(computerId) ? current.sessionId : "";
  const main = instances.find(instance => id !== "" && instance.main_session_id === id);
  const session = useSessionLineage(id, id !== "" && main === undefined, computerId);
  if (main) return main.id;
  if (id === "" || session?.id !== id) return undefined;
  return alwaysOnOwnerOf(instances, session)?.id;
}

/** The verbs an avatar offers. Its one click opens the main session, created again if it was lost. */
export function useAlwaysOnRowActions(
  onOpenSession: (session: SessionListTarget) => void,
  computerId?: ComputerId
) {
  const navigate = useNavigate();
  const remote = isRemoteComputerId(computerId);
  const { computers } = useComputers({ enabled: remote, passive: true });
  const computer = computers.find(row => row.id === computerId);
  const canManage =
    !remote || (computer?.status === "connected" && computer.scopes.includes("settings:write"));
  const canOpen =
    !remote || (computer?.status === "connected" && computer.scopes.includes("sessions:operate"));
  const actions = useAgentInstanceActions(remote ? computerId : undefined);
  const open = async (instance: AgentInstancePayload) => {
    if (!canOpen) return;
    const current = instance.main_session_id
      ? instance
      : canManage
        ? await actions.ensureMain(instance).catch(() => null)
        : null;
    if (!current?.main_session_id) return;
    onOpenSession({
      id: current.main_session_id,
      agent_name: current.agent_name,
      workspace_id: current.workspace_id,
      ...(remote ? { computerId } : {}),
    });
  };
  return {
    open,
    canOpen,
    canManage,
    canNavigateToAgent: !remote,
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
