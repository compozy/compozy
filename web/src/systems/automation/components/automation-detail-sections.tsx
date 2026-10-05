import { CodeBlock, Disclosure, PropertyRow, Section } from "@compozy/ui";

import {
  catchUpPolicyLabel,
  describeFireLimit,
  describeRetry,
  formatDateTime,
} from "../lib/automation-formatters";
import type { LoopTargetProjection } from "../lib/automation-target";
import type { JobRunDigest } from "../lib/job-preview";
import type { AutomationJob } from "../types";
import { AutomationTargetDetails } from "./automation-target-details";

export function AutomationTargetSection({ target }: { target: LoopTargetProjection }) {
  return (
    <Section label="Runs a loop">
      <div className="rounded-md bg-sunken px-4 py-3">
        <AutomationTargetDetails showInputMapping={false} target={target} />
      </div>
    </Section>
  );
}

export function PromptSection({ prompt }: { prompt: string }) {
  return (
    <Section label="Prompt">
      <CodeBlock code={prompt} copyable={false} />
    </Section>
  );
}

export function JobTaskSection({ task }: { task: NonNullable<JobRunDigest["task"]> }) {
  return (
    <Section label="Creates a task">
      <div className="rounded-md bg-sunken px-4 py-3" data-testid="automation-task-details">
        <PropertyRow label="Title">{task.title}</PropertyRow>
        <PropertyRow label="Owner" mono>
          {task.owner}
        </PropertyRow>
        <p className="mt-3 text-small-body leading-relaxed whitespace-pre-wrap text-fg">
          {task.description}
        </p>
      </div>
    </Section>
  );
}

function scheduleExpression(job: AutomationJob): string | null {
  if (job.schedule?.mode === "cron") return job.schedule.expr ?? null;
  if (job.schedule?.mode === "every") return job.schedule.interval ?? null;
  return null;
}

/**
 * Operator-facing scheduler internals and limits, folded closed by default so
 * the page leads with what the job does and how it has been going.
 */
export function JobAdvancedDetails({ job }: { job: AutomationJob }) {
  const scheduler = job.scheduler;
  const expression = scheduleExpression(job);

  return (
    <Disclosure
      className="border-t border-line-soft pt-1"
      data-testid="automation-job-advanced"
      label="Advanced details"
      size="md"
      triggerProps={{
        "data-testid": "automation-job-advanced-toggle",
        className: "w-full py-2.5 text-fg",
      }}
      contentProps={{ className: "flex flex-col pt-0 pb-1 pl-6" }}
    >
      {expression ? (
        <PropertyRow label="Schedule expression" mono>
          {expression}
        </PropertyRow>
      ) : null}
      <PropertyRow label="Retries">{describeRetry(job.retry)}</PropertyRow>
      <PropertyRow label="Run limit">{describeFireLimit(job.fire_limit)}</PropertyRow>
      {scheduler ? (
        <div data-testid="automation-job-scheduler">
          <PropertyRow label="Scheduler">
            {scheduler.registered ? "Registered" : "Idle"}
          </PropertyRow>
          <PropertyRow label="Last scheduled">
            {formatDateTime(scheduler.last_scheduled_at)}
          </PropertyRow>
          <PropertyRow label="Missed runs">
            {catchUpPolicyLabel(scheduler.catch_up_policy)}
          </PropertyRow>
          <PropertyRow label="Times missed">{scheduler.misfire_count ?? 0}</PropertyRow>
          {scheduler.last_fire_id ? (
            <PropertyRow label="Last fire ID" mono>
              {scheduler.last_fire_id}
            </PropertyRow>
          ) : null}
        </div>
      ) : null}
    </Disclosure>
  );
}
