import type { HTMLAttributes, ReactElement, ReactNode } from "react";

import { cn } from "../lib/utils";

export interface EmptyStateProps extends Omit<HTMLAttributes<HTMLElement>, "title"> {
  action?: ReactNode;
  description?: ReactNode;
  eyebrow?: ReactNode;
  icon?: ReactNode;
  title: ReactNode;
}

export function EmptyState({
  action,
  className,
  description,
  eyebrow = "Empty",
  icon,
  title,
  ...props
}: EmptyStateProps): ReactElement {
  return (
    <section
      className={cn(
        "relative overflow-hidden rounded-[var(--radius-xl)] border border-dashed border-border bg-[color:var(--surface-inset)] px-5 py-8",
        "text-center shadow-[var(--shadow-xs)]",
        className
      )}
      {...props}
    >
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-x-8 top-0 h-px bg-linear-to-r from-transparent via-primary/40 to-transparent"
      />
      <div className="mx-auto flex max-w-xl flex-col items-center gap-4">
        {icon ? (
          <div className="grid size-10 place-items-center rounded-full border border-border bg-card text-primary shadow-[var(--shadow-xs)]">
            {icon}
          </div>
        ) : null}
        <div className="space-y-2">
          {eyebrow ? <p className="eyebrow text-muted-foreground">{eyebrow}</p> : null}
          <h2 className="text-base font-semibold text-foreground">{title}</h2>
          {description ? (
            <p className="text-sm leading-6 text-muted-foreground">{description}</p>
          ) : null}
        </div>
        {action ? (
          <div className="flex flex-wrap items-center justify-center gap-2">{action}</div>
        ) : null}
      </div>
    </section>
  );
}
