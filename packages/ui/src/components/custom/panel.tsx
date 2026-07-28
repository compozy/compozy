import type * as React from "react";

import { cn } from "../../lib/utils";

export interface PanelProps extends Omit<React.ComponentProps<"section">, "title"> {
  /** Optional head title; the head row renders only when title, meta, or right is present. */
  title?: React.ReactNode;
  /** Optional mono-faint meta slot rendered next to the title (counts, ids, durations). */
  meta?: React.ReactNode;
  /** Optional action slot right-aligned in the head (pills, buttons, window switches). */
  right?: React.ReactNode;
  /** Optional foot row pinned to the panel bottom (trailing links, summaries). */
  foot?: React.ReactNode;
  /** Optional override for the inner body padding (`p-0` for full-bleed rows). */
  bodyClassName?: string;
}

function hasSlot(value: React.ReactNode): boolean {
  return value !== undefined && value !== null && value !== false;
}

/**
 * Flat panel container — the warm `--canvas-soft` card behind dashboard zones
 * and grouped rows. Head and foot are optional hairline-separated slots; the
 * foot pins to the bottom (`mt-auto`) so paired panels in a grid close flush.
 * Identity H1s never live here; panel titles render an h3 styled with the
 * `text-section-head` token so pages keep a single h1/h2 outline.
 */
export function Panel({
  title,
  meta,
  right,
  foot,
  className,
  bodyClassName,
  children,
  ...props
}: PanelProps) {
  const withHead = hasSlot(title) || hasSlot(meta) || hasSlot(right);
  return (
    <section
      className={cn("flex min-w-0 flex-col rounded-lg bg-canvas-soft", className)}
      data-slot="panel"
      {...props}
    >
      {withHead ? (
        <header
          className="flex items-center justify-between gap-3 border-b border-line-soft px-5 py-3"
          data-slot="panel-head"
        >
          <div className="flex min-w-0 items-baseline gap-2">
            {hasSlot(title) ? (
              <h3
                className="truncate text-section-head font-medium tracking-section-head text-fg-strong"
                data-slot="panel-title"
              >
                {title}
              </h3>
            ) : null}
            {hasSlot(meta) ? (
              <span
                className="shrink-0 font-mono text-mono-id tabular-nums text-faint"
                data-slot="panel-meta"
              >
                {meta}
              </span>
            ) : null}
          </div>
          {hasSlot(right) ? (
            <div className="flex shrink-0 items-center gap-2" data-slot="panel-right">
              {right}
            </div>
          ) : null}
        </header>
      ) : null}
      <div className={cn("min-w-0 p-5", bodyClassName)} data-slot="panel-body">
        {children}
      </div>
      {hasSlot(foot) ? (
        <div
          className="mt-auto flex items-center justify-end gap-2 border-t border-line-soft px-3 py-2"
          data-slot="panel-foot"
        >
          {foot}
        </div>
      ) : null}
    </section>
  );
}
