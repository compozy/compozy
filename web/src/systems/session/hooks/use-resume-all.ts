import {
  agentInstancePausedUnattended,
  useResumeAgentInstances,
  type AgentInstancePayload,
} from "@/systems/agent";

/** Resume all appears once the brake paused at least this many. */
const RESUME_ALL_FROM = 2;

/** S4: resume every agent the brake paused, one by one; who didn't resume reads in place. */
export function useResumeAll(instances: readonly AgentInstancePayload[]) {
  const resume = useResumeAgentInstances({ inline: true });
  const paused = instances.filter(agentInstancePausedUnattended);
  return {
    offered: paused.length >= RESUME_ALL_FROM,
    pending: resume.isPending,
    error: resume.error?.message,
    run: () => resume.mutate(paused),
  };
}
