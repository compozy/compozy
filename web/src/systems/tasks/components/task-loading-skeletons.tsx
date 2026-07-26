import { Skeleton } from "@agh/ui";

const SKELETON_ROWS = [0, 1, 2, 3];

interface TaskRowsLoadingSkeletonProps {
  label: string;
  testId?: string;
  rows?: number;
}

export function TaskRowsLoadingSkeleton({ label, testId, rows = 3 }: TaskRowsLoadingSkeletonProps) {
  return (
    <div
      aria-label={label}
      className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border border-line bg-canvas-soft"
      data-testid={testId}
      role="status"
    >
      {SKELETON_ROWS.slice(0, rows).map(row => (
        <div
          className="flex items-center gap-3 border-b border-line-soft px-4 py-3 last:border-b-0"
          key={row}
        >
          <Skeleton className="size-7 shrink-0 rounded-md" />
          <div className="flex min-w-0 flex-1 flex-col gap-1.5">
            <Skeleton className="h-3 w-3/5 rounded-xs" />
            <Skeleton className="h-2.5 w-2/5 rounded-xs" />
          </div>
          <Skeleton className="h-5 w-16 rounded-pill" />
        </div>
      ))}
    </div>
  );
}

export function TasksDashboardLoadingSkeleton() {
  return (
    <div
      aria-label="Loading tasks dashboard"
      className="flex min-h-0 flex-1 flex-col gap-4 overflow-hidden p-5"
      data-testid="tasks-dashboard-loading"
      role="status"
    >
      <Skeleton className="h-20 w-full rounded-lg" />
      <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
        {SKELETON_ROWS.map(item => (
          <Skeleton className="h-24 rounded-lg" key={item} />
        ))}
      </div>
      <div className="grid flex-1 grid-cols-1 gap-4 xl:grid-cols-[2fr_1fr]">
        <Skeleton className="min-h-48 rounded-lg" />
        <Skeleton className="min-h-48 rounded-lg" />
      </div>
      <Skeleton className="h-32 w-full rounded-lg" />
    </div>
  );
}

export function TaskInspectLoadingSkeleton({ label }: { label: string }) {
  return (
    <div aria-label={label} className="flex flex-col gap-3" role="status">
      <div className="grid grid-cols-2 gap-2.5">
        {SKELETON_ROWS.map(item => (
          <Skeleton className="h-16 rounded-md" key={item} />
        ))}
      </div>
      <Skeleton className="h-4 w-28 rounded-xs" />
      <Skeleton className="h-24 w-full rounded-md" />
    </div>
  );
}
