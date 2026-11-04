/** S2's containers: the pause report resumes in place and settles into a record once it runs. */
import { useAuiState } from "@assistant-ui/react";

import {
  AgentResumeError,
  agentInstancePausedUnattended,
  agentUnattendedSteps,
  useResumeAgentInstances,
  type AgentInstancePayload,
} from "@/systems/agent";

import { newestUnattendedReportId } from "../../../lib/session-report";
import type { ReportAgent } from "../../../lib/session-report-identity";
import { PauseNotice, PauseRecord, ResumeButton, ResumedReceipt } from "./pause-reports";

/** Resume with its pending and failed states; the failure reads in place, not as a toast. */
export function ResumeAction({ instances }: { instances: readonly AgentInstancePayload[] }) {
  const resume = useResumeAgentInstances({ inline: true });
  return (
    <ResumeButton
      disabled={instances.length === 0}
      failed={resume.isError}
      reason={
        resume.error instanceof AgentResumeError
          ? resume.error.failures.map(failure => failure.message).join(" ")
          : undefined
      }
      onResume={() => resume.mutate(instances)}
      pending={resume.isPending}
    />
  );
}

/**
 * S2: one brake pause. Only the newest pause of a still-paused agent offers Resume. Once the
 * agent is active the report reads as resumed; paused for another reason, or superseded by a
 * later pause, it stays a plain record.
 */
export function UnattendedPauseReport({
  agent,
  instance,
  instanceId,
  reportId,
  when,
}: {
  agent: ReportAgent;
  instance: AgentInstancePayload | undefined;
  instanceId: string;
  reportId: string;
  when: string;
}) {
  // A string from the thread's shared pause index: a streamed token re-renders nothing here.
  const newestId = useAuiState(state =>
    newestUnattendedReportId(state.thread.messages, instanceId)
  );
  const newest = newestId === undefined || newestId === reportId;
  const steps = instance ? agentUnattendedSteps(instance) : 0;
  if (instance?.state === "active") return <ResumedReceipt agent={agent} />;
  if (instance && (!agentInstancePausedUnattended(instance) || !newest)) {
    return <PauseRecord agent={agent} steps={steps} when={when} />;
  }
  return (
    <PauseNotice
      action={instance ? <ResumeAction instances={[instance]} /> : null}
      agent={agent}
      held={instance?.held_wakes ?? 0}
      steps={steps}
      when={when}
    />
  );
}
