import { MonoId, Time } from "@agh/ui";

import type { TaskRunDetailView } from "../types";

function MetaDot() {
  return (
    <span aria-hidden="true" className="text-faint">
      ·
    </span>
  );
}

/** Demoted run meta line: mono ids, claimant, freshness, live-ticking elapsed. */
export function TaskRunSubhead({ run, duration }: { run: TaskRunDetailView; duration?: string }) {
  const record = run.run;
  const sessionId = record.session_id ?? run.session?.session_id ?? null;

  return (
    <div
      className="mb-5 flex min-w-0 flex-wrap items-center gap-2 border-b border-line pb-3.5 text-form-label text-subtle"
      data-testid="tasks-run-subhead"
    >
      <MonoId value={record.id} />
      {sessionId ? (
        <>
          <MetaDot />
          <MonoId value={sessionId} />
        </>
      ) : null}
      {record.claimed_by?.ref ? (
        <>
          <MetaDot />
          <span>
            Claimed by <span className="font-medium text-muted">{record.claimed_by.ref}</span>
          </span>
        </>
      ) : null}
      <MetaDot />
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
          <span className="font-mono text-eyebrow tabular-nums">{duration}</span>
        </>
      ) : null}
    </div>
  );
}
