import type { HTMLAttributes, ReactElement } from "react";

import { cn } from "../lib/utils";

export interface SkeletonProps extends HTMLAttributes<HTMLDivElement> {}

export function Skeleton({ className, ...props }: SkeletonProps): ReactElement {
  return (
    <div
      aria-hidden="true"
      className={cn(
        "animate-pulse rounded-[var(--radius-sm)] bg-[color:var(--tone-neutral-bg)]",
        className
      )}
      {...props}
    />
  );
}

export function SkeletonText({
  className,
  lines = 3,
  ...props
}: HTMLAttributes<HTMLDivElement> & { lines?: number }): ReactElement {
  return (
    <div className={cn("space-y-2", className)} aria-hidden="true" {...props}>
      {Array.from({ length: Math.max(1, lines) }).map((_, index) => (
        <Skeleton className={cn("h-3", index === lines - 1 ? "w-2/3" : "w-full")} key={index} />
      ))}
    </div>
  );
}

export function SkeletonRow({ className, ...props }: HTMLAttributes<HTMLDivElement>): ReactElement {
  return (
    <div
      aria-hidden="true"
      className={cn(
        "flex items-center justify-between gap-3 rounded-[var(--radius-md)] border border-border bg-black/10 px-3 py-2",
        className
      )}
      {...props}
    >
      <div className="min-w-0 flex-1 space-y-2">
        <Skeleton className="h-3 w-2/5" />
        <Skeleton className="h-2.5 w-3/5" />
      </div>
      <Skeleton className="h-5 w-16 rounded-full" />
    </div>
  );
}
