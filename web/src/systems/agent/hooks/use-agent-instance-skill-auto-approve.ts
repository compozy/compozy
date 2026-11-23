import { useMutation, useQueryClient } from "@tanstack/react-query";

import { notifyUser } from "@/lib/user-feedback";

import {
  isAgentInstanceRouteMissing,
  setAgentInstanceSkillAutoApprove,
  type AgentInstanceRef,
} from "../adapters/agent-instance-api";
import { applyAgentInstanceUpdate } from "./use-agent-instances";

/** The per-place "Auto-approve skills" switch; only an operator can turn it on. */
export function useSetAgentInstanceSkillAutoApprove() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ instance, enabled }: { instance: AgentInstanceRef; enabled: boolean }) =>
      setAgentInstanceSkillAutoApprove(instance, enabled),
    onSuccess: instance => applyAgentInstanceUpdate(queryClient, instance),
    onError: error =>
      notifyUser({
        message: isAgentInstanceRouteMissing(error)
          ? "Skill auto-approve can't be changed here yet."
          : error.message,
        tone: "error",
      }),
  });
}
