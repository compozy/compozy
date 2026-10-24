import { Time } from "@compozy/ui";

import type { TaskRunDetailView } from "../types";
import { TASK_SUBHEAD_CLASS } from "./task-surface-classes";

function MetaDot() {
  return (
    <span aria-hidden="true" className="text-faint">
      ·
    </span>
  );
}

/** Demoted run meta line: freshness and live-ticking elapsed; ids live behind Inspect. */
export function TaskRunSubhead({ run, duration }: { run: TaskRunDetailView; duration?: string }) {
  const record = run.run;

  return (
    <div className={TASK_SUBHEAD_CLASS} data-testid="tasks-run-subhead">
      {record.ended_at ? (
        <span className="inline-flex items-center gap-1">
          Ended <Time iso={record.ended_at} mode="relative" />
        </span>
      ) : record.started_at ? (
        <span className="inline-flex items-center gap-1">
          Started <Time iso={record.started_at} mode="relative" />
        </span>
      ) : (
        <span className="inline-flex items-center gap-1">
          Queued <Time iso={record.queued_at} mode="relative" />
        </span>
      )}
      {duration ? (
        <>
          <MetaDot />
          <span className="tabular-nums">{duration}</span>
        </>
      ) : null}
    </div>
  );
}
