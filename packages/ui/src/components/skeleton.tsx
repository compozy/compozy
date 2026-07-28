import * as React from "react";

import { cn } from "../lib/utils";

function Skeleton({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="skeleton"
      className={cn(
        "animate-shimmer rounded-md bg-canvas-soft bg-[linear-gradient(90deg,transparent_0%,var(--color-surface-glaze)_50%,transparent_100%)] bg-size-[200%_100%]",
        className
      )}
      {...props}
    />
  );
}

export interface SkeletonRowsProps extends React.ComponentProps<"div"> {
  count?: number;
  rowClassName?: string;
  children?: React.ReactNode;
}

function SkeletonRows({
  count = 3,
  rowClassName,
  className,
  children,
  ...props
}: SkeletonRowsProps) {
  const rows = Array.from({ length: count }, (_, position) => ({
    id: `skeleton-row-${position}`,
  }));

  return (
    <div data-slot="skeleton-rows" className={cn("flex flex-col", className)} {...props}>
      {rows.map(row => (
        <div
          data-slot="skeleton-row"
          className={cn("flex flex-col gap-2", rowClassName)}
          key={row.id}
        >
          {children ?? (
            <>
              <Skeleton className="h-3.5 w-2/3" />
              <Skeleton className="h-3 w-full" />
              <Skeleton className="h-3 w-3/4" />
            </>
          )}
        </div>
      ))}
    </div>
  );
}

export { Skeleton, SkeletonRows };
