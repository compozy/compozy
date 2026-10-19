import { useQuery } from "@tanstack/react-query";

import { agentsListOptions } from "@/systems/agent";

import type { SessionPayload } from "../types";
import { useSessionRuntimeLabel } from "./use-session-runtime-label";

export function useSessionRowRuntimeLabel(session: SessionPayload) {
  const recorded = session.runtime?.effective ?? session.runtime?.last_known;
  const selected = session.runtime?.selected;
  const defaults = useQuery({
    ...agentsListOptions(session.workspace_id, session.profile_name),
    enabled: !recorded && !selected && !!session.profile_name,
    select: agents => agents.find(agent => agent.name === session.agent_name)?.effective_runtime,
  });
  const runtime = useSessionRuntimeLabel(recorded ?? selected ?? defaults.data);
  return runtime ?? { provider: "unknown", label: "Runtime not selected" };
}
