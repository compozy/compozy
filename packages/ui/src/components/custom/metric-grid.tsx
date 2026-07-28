"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

type MetricGridColumns = 1 | 2 | 3 | 4;

interface MetricGridProps extends React.ComponentProps<"div"> {
  columns?: MetricGridColumns;
}

const GRID_COLUMNS: Record<MetricGridColumns, string> = {
  1: "grid-cols-1",
  2: "grid-cols-1 sm:grid-cols-2",
  3: "grid-cols-1 sm:grid-cols-2 xl:grid-cols-3",
  4: "grid-cols-1 sm:grid-cols-2 xl:grid-cols-4",
};

function MetricGrid({ columns = 4, className, ...props }: MetricGridProps) {
  return (
    <div
      data-slot="metric-grid"
      data-columns={columns}
      className={cn("grid gap-3", GRID_COLUMNS[columns], className)}
      {...props}
    />
  );
}

export { MetricGrid };
export type { MetricGridColumns, MetricGridProps };
