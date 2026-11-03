/** S2's containers: the pause report resumes in place and settles into a record once it runs. */
import { useState } from "react";

import {
  agentInstancePausedUnattended,
  agentUnattendedSteps,
  useResumeAgentInstances,
  type AgentInstancePayload,
} from "@/systems/agent";

import type { ReportAgent } from "../../../lib/session-report-identity";
import { PauseNotice, ResumeButton, ResumedReceipt } from "./pause-reports";

/** Resume with its pending and failed states, for every place a paused report sits. */
export function ResumeAction({
  instances,
  onResumed,
}: {
  instances: readonly AgentInstancePayload[];
  onResumed?: () => void;
}) {
  const resume = useResumeAgentInstances();
  return (
    <ResumeButton
      disabled={instances.length === 0}
      failed={resume.isError}
      onResume={() => resume.mutate(instances, { onSuccess: onResumed })}
      pending={resume.isPending}
    />
  );
}

/** S2: the pause report, resumed in place. */
export function UnattendedPauseReport({
  agent,
  instance,
  when,
}: {
  agent: ReportAgent;
  instance: AgentInstancePayload | undefined;
  when: string;
}) {
  // What was waiting when you resumed here, so the record can say what it delivered.
  const [delivered, setDelivered] = useState(0);
  if (instance && !agentInstancePausedUnattended(instance)) {
    return <ResumedReceipt agent={agent} delivered={delivered} />;
  }
  return (
    <PauseNotice
      action={
        instance ? (
          <ResumeAction
            instances={[instance]}
            onResumed={() => setDelivered(instance.held_wakes ?? 0)}
          />
        ) : null
      }
      agent={agent}
      held={instance?.held_wakes ?? 0}
      steps={instance ? agentUnattendedSteps(instance) : 0}
      when={when}
    />
  );
}
