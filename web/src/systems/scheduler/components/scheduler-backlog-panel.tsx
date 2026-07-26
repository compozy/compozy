import { Eyebrow, MonoId, Pill, Skeleton, SkeletonRows } from "@agh/ui";

import type { SchedulerBacklog } from "../types";

const BACKLOG_PREVIEW_LIMIT = 5;

interface SchedulerBacklogPanelProps {
  backlog: SchedulerBacklog | null;
  errorMessage: string | null;
  isLoading: boolean;
}

function SchedulerBacklogPanel({ backlog, errorMessage, isLoading }: SchedulerBacklogPanelProps) {
  const rows = backlog?.runs?.slice(0, BACKLOG_PREVIEW_LIMIT) ?? [];
  const isInitialLoading = isLoading && !backlog;

  return (
    <div className="mt-4 border-t border-line-soft pt-3" data-testid="scheduler-backlog-panel">
      <div className="flex items-center justify-between gap-3">
        <Eyebrow className="text-muted">Backlog</Eyebrow>
        {isInitialLoading ? (
          <Skeleton
            aria-label="Loading backlog total"
            className="h-5 w-8 rounded-full"
            data-testid="scheduler-backlog-total-loading"
            role="status"
          />
        ) : (
          <Pill data-testid="scheduler-backlog-total" tone={backlog?.total ? "warning" : "neutral"}>
            {backlog?.total ?? 0}
          </Pill>
        )}
      </div>
      {errorMessage ? (
        <p className="mt-2 text-form-label text-danger" data-testid="scheduler-backlog-error">
          {errorMessage}
        </p>
      ) : isInitialLoading ? (
        <SkeletonRows
          aria-label="Loading scheduler backlog"
          className="mt-2"
          count={3}
          data-testid="scheduler-backlog-loading"
          role="status"
          rowClassName="border-b border-line-soft py-2"
        />
      ) : rows.length === 0 ? (
        <p className="mt-2 text-form-label text-muted" data-testid="scheduler-backlog-empty">
          No queued runs.
        </p>
      ) : (
        <div className="mt-2 divide-y divide-line-soft" data-testid="scheduler-backlog-rows">
          {rows.map(item => (
            <div
              className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 py-2 text-form-label"
              data-testid={`scheduler-backlog-row-${item.run.id}`}
              key={item.run.id}
            >
              <div className="min-w-0">
                <div className="flex min-w-0 items-center gap-2">
                  <MonoId value={item.task.identifier ?? item.task.id} />
                  <span className="truncate text-fg">{item.task.title}</span>
                </div>
                <div className="mt-1 flex flex-wrap items-center gap-2 text-muted">
                  <span>Run {item.run.id}</span>
                  <span aria-hidden="true" className="text-faint">
                    ·
                  </span>
                  <span>{item.run.status}</span>
                </div>
              </div>
              {item.task.effective_paused ? (
                <Pill tone="warning">Paused</Pill>
              ) : (
                <Pill tone="neutral">Queued</Pill>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export { SchedulerBacklogPanel };
