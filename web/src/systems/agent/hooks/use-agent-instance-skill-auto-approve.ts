import { useMutation, useMutationState, useQueryClient } from "@tanstack/react-query";

import { notifyUser } from "@/lib/user-feedback";

import {
  isAgentInstanceRouteMissing,
  setAgentInstanceSkillAutoApprove,
  type AgentInstanceRef,
} from "../adapters/agent-instance-api";
import { applyAgentInstanceUpdate } from "./use-agent-instances";

const skillAutoApproveMutationKey = ["agent-instance-skill-auto-approve"] as const;
type SkillAutoApproveUpdate = { instance: AgentInstanceRef; enabled: boolean };

/** The per-place "Auto-approve skills" switch; only an operator can turn it on. */
export function useSetAgentInstanceSkillAutoApprove() {
  const queryClient = useQueryClient();
  const { mutate } = useMutation({
    mutationKey: skillAutoApproveMutationKey,
    mutationFn: ({ instance, enabled }: SkillAutoApproveUpdate) =>
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
  const pendingInstanceIds = useMutationState({
    filters: { mutationKey: skillAutoApproveMutationKey, status: "pending", exact: true },
    select: mutation => (mutation.state.variables as SkillAutoApproveUpdate).instance.id,
  });
  return { mutate, pendingInstanceIds };
}
