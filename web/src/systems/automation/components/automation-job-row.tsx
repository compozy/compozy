import { Clock3, Play } from "lucide-react";
import { Link } from "@tanstack/react-router";

import { Button, ListingRow, Pill } from "@agh/ui";

import {
  automationScopeLabel,
  automationSourceLabel,
  automationSourceTone,
  automationStatusTone,
  describeSchedule,
  formatRelativeTime,
} from "../lib/automation-formatters";
import type { AutomationJob } from "../types";

export interface AutomationJobRowProps {
  job: AutomationJob;
  isRunPending: boolean;
  onRun: (id: string) => void;
  runDisabled: boolean;
}

/** Row presentation for a job in the Jobs catalog (rows view). */
function AutomationJobRow({ job, isRunPending, onRun, runDisabled }: AutomationJobRowProps) {
  const enabledTone = automationStatusTone(job.enabled ? "enabled" : "disabled");
  const nextRun = job.scheduler?.next_run_at ?? job.next_run;

  return (
    <ListingRow data-automation-id={job.id} data-testid={`automation-item-${job.id}`}>
      <ListingRow.Link
        render={
          <Link aria-label={`Open ${job.name}`} params={{ jobId: job.id }} to="/jobs/$jobId" />
        }
      >
        <ListingRow.Icon>
          <Clock3 aria-hidden="true" className="size-4" />
        </ListingRow.Icon>
        <ListingRow.Main>
          <ListingRow.Name>
            <ListingRow.Title>{job.name}</ListingRow.Title>
            <span className="flex shrink-0 items-center gap-1.5">
              <Pill.Dot tone={enabledTone} />
              <Pill mono size="xs" tone={enabledTone}>
                {job.enabled ? "ENABLED" : "DISABLED"}
              </Pill>
            </span>
            <Pill mono size="xs" tone={automationSourceTone(job.source)}>
              {automationSourceLabel(job.source)}
            </Pill>
          </ListingRow.Name>
          <ListingRow.Meta>
            <span>{describeSchedule(job.schedule)}</span>
            <ListingRow.MetaDot />
            <span>{automationScopeLabel(job.scope)}</span>
          </ListingRow.Meta>
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail>
        <ListingRow.Stat>
          <ListingRow.Stat.Value>{formatRelativeTime(nextRun)}</ListingRow.Stat.Value>
          <ListingRow.Stat.Label>next run</ListingRow.Stat.Label>
        </ListingRow.Stat>
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
      </ListingRow.Trail>
    </ListingRow>
  );
}

export { AutomationJobRow };
