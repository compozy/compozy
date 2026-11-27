import { AlertCircle, RefreshCw } from "@compozy/ui/icons";

import { Button, Skeleton, cn } from "@compozy/ui";

export interface ScopedComputerSkeletonProps {
  computerName: string;
  computerId?: string;
  rows?: number;
  className?: string;
}

/**
 * Per-computer loading placeholder (`_uiux.md` U5). Keeps local and other
 * connected computers interactive while one computer is still loading.
 */
export function ScopedComputerSkeleton({
  computerName,
  computerId,
  rows = 2,
  className,
}: ScopedComputerSkeletonProps) {
  return (
    <div
      className={cn(
        "flex flex-col gap-1.5 rounded-md border border-line-soft bg-surface-2/40 px-2.5 py-2",
        className
      )}
      data-computer-id={computerId}
      data-testid={
        computerId ? `computer-loading-skeleton-${computerId}` : "computer-loading-skeleton"
      }
      role="status"
    >
      <span className="text-micro text-muted-foreground">
        Loading sessions from {computerName}…
      </span>
      {Array.from({ length: rows }, (_, index) => (
        <Skeleton className="h-5 w-full rounded" key={index} />
      ))}
    </div>
  );
}

export interface ScopedComputerErrorProps {
  computerName: string;
  computerId?: string;
  onRetry: () => void;
  className?: string;
}

/**
 * Per-computer inline failure row (`_uiux.md` U5). Other computers stay usable
 * while the failed computer offers an explicit Retry button.
 */
export function ScopedComputerError({
  computerName,
  computerId,
  onRetry,
  className,
}: ScopedComputerErrorProps) {
  return (
    <div
      className={cn(
        "flex items-center justify-between gap-2 rounded-md border border-line bg-surface-2/70 px-2.5 py-1.5 text-eyebrow text-muted-foreground",
        className
      )}
      data-computer-id={computerId}
      data-testid={computerId ? `computer-load-error-${computerId}` : "computer-load-error"}
      role="alert"
    >
      <span className="flex min-w-0 items-center gap-1.5">
        <AlertCircle aria-hidden="true" className="size-3.5 shrink-0 text-warning" />
        <span className="truncate">Couldn&apos;t load sessions from {computerName}</span>
      </span>
      <Button onClick={onRetry} size="xs" type="button" variant="secondary">
        <RefreshCw aria-hidden="true" data-icon="inline-start" />
        Retry
      </Button>
    </div>
  );
}

export interface ScopedComputerFailureItem {
  id: string;
  name: string;
  retry: () => void;
}

/** Compact strip above Kanban columns when one or more remote computers failed to load. */
export function ScopedComputerBoardFailureStrip({
  failures,
  className,
}: {
  failures: readonly ScopedComputerFailureItem[];
  className?: string;
}) {
  if (failures.length === 0) return null;
  return (
    <div
      className={cn("flex flex-wrap items-center gap-2 px-4 pt-2", className)}
      data-testid="board-computer-failures"
    >
      {failures.map(failure => (
        <ScopedComputerError
          computerId={failure.id}
          computerName={failure.name}
          key={failure.id}
          onRetry={failure.retry}
        />
      ))}
    </div>
  );
}
