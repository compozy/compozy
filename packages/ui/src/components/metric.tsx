import type { HTMLAttributes, ReactElement, ReactNode } from "react";

import { cn } from "../lib/utils";

export interface MetricProps extends HTMLAttributes<HTMLDivElement> {
  label: ReactNode;
  value: ReactNode;
  hint?: ReactNode;
  trailing?: ReactNode;
}

export function Metric({
  className,
  hint,
  label,
  trailing,
  value,
  ...props
}: MetricProps): ReactElement {
  return (
    <div
      className={cn(
        "flex min-w-0 flex-col justify-between gap-3 rounded-[var(--radius-lg)] border border-border bg-card px-5 py-4 shadow-[var(--shadow-xs)]",
        className
      )}
      {...props}
    >
      <div className="flex items-start justify-between gap-3">
        <p className="eyebrow text-muted-foreground">{label}</p>
        {trailing ? <div className="flex shrink-0 items-center gap-2">{trailing}</div> : null}
      </div>
      <div className="min-w-0 space-y-1">
        <p className="font-display text-4xl leading-none tracking-[-0.02em] text-foreground">
          {value}
        </p>
        {hint ? <p className="truncate text-xs text-muted-foreground">{hint}</p> : null}
      </div>
    </div>
  );
}
