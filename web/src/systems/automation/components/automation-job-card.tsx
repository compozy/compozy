import { Clock3, Play } from "lucide-react";
import { Link } from "@tanstack/react-router";

import { Button, CatalogCard, Pill } from "@agh/ui";

import {
  automationSourceLabel,
  automationSourceTone,
  automationStatusTone,
  describeSchedule,
  formatRelativeTime,
} from "../lib/automation-formatters";
import type { AutomationJob } from "../types";

export interface AutomationJobCardProps {
  job: AutomationJob;
  isRunPending: boolean;
  onRun: (id: string) => void;
  runDisabled: boolean;
}

/** Card presentation for a job in the Jobs catalog (cards view). */
function AutomationJobCard({ job, isRunPending, onRun, runDisabled }: AutomationJobCardProps) {
  const enabledTone = automationStatusTone(job.enabled ? "enabled" : "disabled");
  const nextRun = job.scheduler?.next_run_at ?? job.next_run;

  return (
    <CatalogCard actionable data-automation-id={job.id} data-testid={`automation-card-${job.id}`}>
      <Link
        aria-label={`Open ${job.name}`}
        className="flex min-w-0 flex-col gap-3"
        params={{ jobId: job.id }}
        to="/jobs/$jobId"
      >
        <div className="flex items-start gap-3">
          <CatalogCard.Logo>
            <Clock3 className="size-4" />
          </CatalogCard.Logo>
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <CatalogCard.Title>{job.name}</CatalogCard.Title>
            <CatalogCard.Meta>
              <span>{describeSchedule(job.schedule)}</span>
              <span>{`Next ${formatRelativeTime(nextRun)}`}</span>
            </CatalogCard.Meta>
          </div>
        </div>
      </Link>
      <CatalogCard.Actions className="justify-between">
        <span className="flex items-center gap-1.5">
          <Pill.Dot tone={enabledTone} />
          <Pill mono size="sm" tone={automationSourceTone(job.source)}>
            {automationSourceLabel(job.source)}
          </Pill>
        </span>
        <Button
          aria-label={`Run ${job.name} now`}
          data-testid={`automation-run-now-${job.id}`}
          disabled={runDisabled || isRunPending}
          onClick={() => onRun(job.id)}
          size="sm"
          type="button"
          variant="outline"
        >
          <Play aria-hidden="true" className="size-3" />
          {isRunPending ? "Queuing..." : "Run now"}
        </Button>
      </CatalogCard.Actions>
    </CatalogCard>
  );
}

export { AutomationJobCard };
