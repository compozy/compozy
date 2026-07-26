import type { ReactNode } from "react";

import { cn } from "@agh/ui";

export interface SettingsGroupProps {
  title?: ReactNode;
  /** One sentence about what the group controls, in user language. */
  description?: ReactNode;
  /** Optional header action (quiet, right-aligned). */
  action?: ReactNode;
  /** Rendered inside the panelbox frame; usually `SettingRow`s. */
  children: ReactNode;
  /** Drop the panelbox frame (tiles and choice grids frame themselves). */
  bare?: boolean;
  className?: string;
  "data-testid"?: string;
}

/**
 * A titled cluster of setting rows (design system §04): sentence-case title,
 * quiet description, rows inside one panelbox.
 */
export function SettingsGroup({
  title,
  description,
  action,
  children,
  bare = false,
  className,
  "data-testid": testId,
}: SettingsGroupProps) {
  return (
    <section className={cn("flex flex-col gap-2.5", className)} data-testid={testId}>
      {title || description || action ? (
        <header className="flex items-start justify-between gap-3">
          <div className="flex min-w-0 flex-col gap-0.5">
            {title ? (
              <h2 className="text-ws-name font-semibold tracking-tight text-fg-strong">{title}</h2>
            ) : null}
            {description ? (
              <p className="max-w-settings-page-description text-form-label text-muted">
                {description}
              </p>
            ) : null}
          </div>
          {action ? <div className="flex shrink-0 items-center gap-2">{action}</div> : null}
        </header>
      ) : null}
      {bare ? (
        children
      ) : (
        <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
          {children}
        </div>
      )}
    </section>
  );
}
