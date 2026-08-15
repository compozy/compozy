import type { ComponentProps, ComponentType, ReactNode } from "react";

import { cn } from "@/lib/utils";
import { Empty, Section } from "@compozy/ui";

export interface CatalogEmptyStateProps extends Omit<ComponentProps<"div">, "title"> {
  /** Neutral secondary action under the intro (e.g. "Create from scratch"). */
  action?: ReactNode;
  /** Intro glyph — rendered muted inside the bordered square, never accent. */
  icon: ComponentType<{ className?: string }>;
  /** Optional suggestions/templates panel; compose it with CatalogEmptyPanel. */
  panel?: ReactNode;
  /** One-line object definition under the title. */
  support: string;
  title: ReactNode;
}

/**
 * Shared zero-inventory composition for the Jobs, Triggers, and Tasks
 * catalogs: a quiet centered intro above an optional panel, in one centered
 * 720px column. Pages vary only icon, title, support line, action, and panel
 * content — the structure stays identical.
 */
export function CatalogEmptyState({
  action,
  className,
  icon: Icon,
  panel,
  support,
  title,
  ...props
}: CatalogEmptyStateProps) {
  return (
    <div className={cn("flex min-h-0 flex-1 flex-col overflow-y-auto", className)} {...props}>
      <div className="m-auto flex w-full max-w-180 flex-col gap-5.5 px-6 py-7">
        <Empty
          action={action}
          description={support}
          fill={false}
          icon={
            <span
              aria-hidden="true"
              className="flex size-full items-center justify-center rounded-lg border border-line bg-canvas text-muted"
            >
              <Icon className="size-4" />
            </span>
          }
          title={title}
        />
        {panel}
      </div>
    </div>
  );
}

export interface CatalogEmptyPanelProps extends ComponentProps<"section"> {
  count?: number | string;
  /** Optional strip under the rows (e.g. the CLI alternative). */
  footer?: ReactNode;
  label: ReactNode;
  /** At most one short muted line under the panel eyebrow. */
  note?: ReactNode;
}

/**
 * The one panel treatment of the zero-inventory state: hairline border,
 * canvas-soft fill, eyebrow header with count plus at most one muted note,
 * and rule-separated disclosure rows.
 */
export function CatalogEmptyPanel({
  children,
  className,
  count,
  footer,
  label,
  note,
  ...props
}: CatalogEmptyPanelProps) {
  return (
    <Section
      bodyClassName="min-h-0"
      bordered
      className={cn(
        "gap-0 overflow-hidden rounded-lg border border-line bg-canvas-soft",
        className
      )}
      count={count}
      headClassName="px-4 pt-3.5 pb-3"
      label={label}
      note={note}
      {...props}
    >
      {children}
      {footer ? (
        <footer className="flex flex-wrap items-center gap-2 border-t border-line px-4 py-2.5 text-form-hint text-subtle">
          {footer}
        </footer>
      ) : null}
    </Section>
  );
}
