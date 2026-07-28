import { SquareCheck, Terminal } from "lucide-react";

import type { JobRunDigest } from "../../../lib/job-preview";
import { AutomationTargetDetails } from "../../automation-target-details";
import { PreviewCard } from "../../trigger-form/preview/preview-card";

interface RunDigestCardProps {
  runDigest: JobRunDigest;
}

/** Digest of the run body: the prompt the agent receives, or the materialized task. */
export function RunDigestCard({ runDigest }: RunDigestCardProps) {
  if (runDigest.output === "loop" && runDigest.loopTarget) {
    return (
      <PreviewCard icon={Terminal} label="Loop target">
        <AutomationTargetDetails showInputMapping={false} target={runDigest.loopTarget} />
      </PreviewCard>
    );
  }
  if (runDigest.output === "task" && runDigest.task) {
    const task = runDigest.task;
    return (
      <PreviewCard icon={SquareCheck} label="Task it materializes">
        <dl className="grid grid-cols-[auto_1fr] gap-x-3.5 gap-y-1.5 text-small-body">
          <dt className="text-subtle">Title</dt>
          <dd className="text-right font-medium text-fg">{task.title}</dd>
          <dt className="text-subtle">Owner</dt>
          <dd className="text-right font-mono text-form-hint font-medium text-fg">{task.owner}</dd>
          <dt className="text-subtle">Participation</dt>
          <dd
            className="text-right font-mono text-form-hint font-medium text-fg"
            data-testid="job-preview-task-participation"
          >
            {task.participation.mode === "live" ? "Live" : "Local"}
          </dd>
          {task.participation.channelStrategy ? (
            <>
              <dt className="text-subtle">Channel strategy</dt>
              <dd className="text-right font-mono text-form-hint font-medium text-fg">
                {task.participation.channelStrategy}
              </dd>
            </>
          ) : null}
          {task.participation.channelId ? (
            <>
              <dt className="text-subtle">Channel</dt>
              <dd
                className="text-right font-mono text-form-hint font-medium text-fg"
                data-testid="job-preview-task-channel"
              >
                {task.participation.channelId}
              </dd>
            </>
          ) : null}
          <dt className="text-subtle">Run status</dt>
          <dd className="text-right font-mono text-form-hint font-medium text-info">
            {task.runStatus}
          </dd>
        </dl>
        <div className="mt-2.5 border-t border-line-soft pt-2.5 text-small-body leading-relaxed whitespace-pre-wrap text-fg">
          {task.description}
        </div>
      </PreviewCard>
    );
  }
  return (
    <PreviewCard icon={Terminal} label="Prompt the agent receives">
      <div className="text-small-body leading-relaxed whitespace-pre-wrap text-fg">
        {runDigest.prompt}
      </div>
    </PreviewCard>
  );
}
