import type { ComponentProps, ReactNode } from "react";

import { cn } from "@compozy/ui";

export interface TaskRailSectionProps extends ComponentProps<"section"> {
  label: string;
  action?: ReactNode;
  children: ReactNode;
}

/**
 * One band of the 320px task properties rail: eyebrow label, optional trailing
 * action, then property rows. Sections stack inside a single card separated by
 * hairlines, so the first one drops its top border.
 */
export function TaskRailSection({
  label,
  action,
  children,
  className,
  ...props
}: TaskRailSectionProps) {
  return (
    <section
      {...props}
      className={cn("border-t border-line-soft px-4 py-3.5 first:border-t-0", className)}
    >
      <header className="mb-2 flex items-center justify-between gap-2">
        <h3 className="eyebrow text-subtle">{label}</h3>
        {action}
      </header>
      {children}
    </section>
  );
}
