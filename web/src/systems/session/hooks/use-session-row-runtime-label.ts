import { useQuery } from "@tanstack/react-query";

import { agentsListOptions } from "@/systems/agent";

import type { SessionPayload } from "../types";
import { useSessionRuntimeLabel } from "./use-session-runtime-label";

export function useSessionRowRuntimeLabel(session: SessionPayload) {
  const snapshot = session.runtime;
  const wasBound = !!snapshot?.generation || !!snapshot?.acp_session_id;
  const recorded = snapshot?.effective ?? (wasBound ? snapshot?.last_known : undefined);
  const selection = recorded ?? snapshot?.selected ?? snapshot?.last_known;
  const defaults = useQuery({
    ...agentsListOptions(session.workspace_id, session.profile_name),
    enabled: !selection && !!session.profile_name,
    select: agents => agents.find(agent => agent.name === session.agent_name)?.effective_runtime,
  });
  const runtime = useSessionRuntimeLabel(selection ?? defaults.data);
  return runtime ?? { provider: "unknown", label: "Runtime not selected" };
}
