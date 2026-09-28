import { useState } from "react";
import { ChevronRight } from "lucide-react";

import {
  CodeBlock,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  PropertyRow,
  Section,
  cn,
} from "@compozy/ui";

import {
  catchUpPolicyLabel,
  describeFireLimit,
  describeRetry,
  formatDateTime,
} from "../lib/automation-formatters";
import type { LoopTargetProjection } from "../lib/automation-target";
import type { AutomationJob } from "../types";
import { AutomationTargetDetails } from "./automation-target-details";

export function AutomationTargetSection({ target }: { target: LoopTargetProjection }) {
  return (
    <Section label="Runs a loop">
      <div className="rounded-md border border-line bg-canvas-soft px-4 py-3">
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
  const [open, setOpen] = useState(false);
  const scheduler = job.scheduler;
  const expression = scheduleExpression(job);

  return (
    <Collapsible
      className="border-t border-line-soft pt-1"
      data-testid="automation-job-advanced"
      onOpenChange={setOpen}
      open={open}
    >
      <CollapsibleTrigger
        className="flex w-full items-center gap-2 rounded-sm py-2.5 text-left outline-none focus-visible:shadow-focus-ring"
        data-testid="automation-job-advanced-toggle"
        type="button"
      >
        <ChevronRight
          aria-hidden="true"
          className={cn(
            "size-4 text-muted transition-transform duration-base ease-out",
            open && "rotate-90"
          )}
        />
        <span className="text-small-body font-medium text-fg">Advanced details</span>
      </CollapsibleTrigger>
      <CollapsibleContent className="flex flex-col pb-1 pl-6">
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
      </CollapsibleContent>
    </Collapsible>
  );
}
