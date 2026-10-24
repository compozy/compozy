import { Eyebrow, MonoId, Pill, Skeleton, SkeletonRegion, SkeletonText } from "@compozy/ui";

import type { SchedulerBacklog } from "../types";

const BACKLOG_PREVIEW_LIMIT = 5;
const BACKLOG_ROW_CLASS =
  "grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 py-2 text-form-label";
const SKELETON_ROWS = ["first", "second", "third"] as const;

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
            aria-hidden="true"
            className="h-pill-sm w-8 rounded-pill"
            data-testid="scheduler-backlog-total-loading"
          />
        ) : (
          <Pill data-testid="scheduler-backlog-total" tone="neutral">
            {backlog?.total ?? 0}
          </Pill>
        )}
      </div>
      {errorMessage ? (
        <p className="mt-2 text-form-label text-danger" data-testid="scheduler-backlog-error">
          {errorMessage}
        </p>
      ) : isInitialLoading ? (
        <SkeletonRegion
          className="mt-2"
          data-testid="scheduler-backlog-loading"
          label="Loading scheduler backlog"
        >
          <div aria-hidden="true" className="divide-y divide-line-soft">
            {SKELETON_ROWS.map(row => (
              <div className={BACKLOG_ROW_CLASS} key={row}>
                <div className="flex min-w-0 items-center gap-2">
                  <SkeletonText className="w-16" />
                  <SkeletonText className="w-48" />
                </div>
                <Skeleton className="h-4 w-14 rounded-pill" />
              </div>
            ))}
          </div>
        </SkeletonRegion>
      ) : rows.length === 0 ? (
        <p className="mt-2 text-form-label text-muted" data-testid="scheduler-backlog-empty">
          No queued runs.
        </p>
      ) : (
        <div className="mt-2 divide-y divide-line-soft" data-testid="scheduler-backlog-rows">
          {rows.map(item => (
            <div
              className={BACKLOG_ROW_CLASS}
              data-testid={`scheduler-backlog-row-${item.run.id}`}
              key={item.run.id}
            >
              <div className="min-w-0">
                <div className="flex min-w-0 items-center gap-2">
                  {item.task.identifier ? <MonoId value={item.task.identifier} /> : null}
                  <span className="truncate text-fg">{item.task.title}</span>
                </div>
              </div>
              {item.task.effective_paused ? (
                <Pill form="plain">
                  <Pill.Dot tone="warning" />
                  Paused
                </Pill>
              ) : (
                <Pill form="plain">
                  <Pill.Dot tone="neutral" />
                  Queued
                </Pill>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export { SchedulerBacklogPanel };
