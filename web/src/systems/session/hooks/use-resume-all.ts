import {
  AgentResumeError,
  agentInstancePausedUnattended,
  agentResumeFailureCopy,
  useResumeAgentInstances,
  type AgentInstancePayload,
} from "@/systems/agent";

/** Resume all appears once the brake paused at least this many. */
const RESUME_ALL_FROM = 2;

/**
 * S4: resume every agent the brake paused, one by one; who didn't resume reads in place. The
 * error names only agents the brake still holds, so it clears once they resume from anywhere.
 */
export function useResumeAll(instances: readonly AgentInstancePayload[]) {
  const resume = useResumeAgentInstances({ inline: true });
  const paused = instances.filter(agentInstancePausedUnattended);
  const stillPaused = new Set(paused.map(instance => instance.id));
  const failures =
    resume.error instanceof AgentResumeError
      ? resume.error.failures.filter(failure => stillPaused.has(failure.instance.id))
      : [];
  const error =
    resume.error instanceof AgentResumeError
      ? failures.length > 0
        ? agentResumeFailureCopy(failures)
        : undefined
      : resume.error?.message;
  return {
    offered: paused.length >= RESUME_ALL_FROM,
    pending: resume.isPending,
    error,
    run: () => resume.mutate(paused),
  };
}
