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
        "flex min-w-0 flex-col justify-between gap-4 rounded-[var(--radius-xl)] border border-border-subtle bg-card px-5 py-4 shadow-[var(--shadow-sm)]",
        "transition-[border-color,background-color,box-shadow,transform] duration-200 ease-out hover:-translate-y-px hover:border-border-strong hover:shadow-[var(--shadow-md)]",
        className
      )}
      {...props}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="eyebrow text-muted-foreground">{label}</div>
        {trailing ? <div className="flex shrink-0 items-center gap-2">{trailing}</div> : null}
      </div>
      <div className="min-w-0 space-y-1">
        <div className="font-mono text-3xl leading-none text-foreground tabular-nums">{value}</div>
        {hint ? <div className="truncate text-xs text-muted-foreground">{hint}</div> : null}
      </div>
    </div>
  );
}
