import { ChevronRight } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/lib/utils";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@compozy/ui";

export type CatalogEmptyTone = "accent" | "info" | "warning" | "neutral";

const TONE_CLASS: Record<CatalogEmptyTone, string> = {
  accent: "bg-accent-tint text-accent",
  info: "bg-info-tint text-info",
  warning: "bg-warning-tint text-warning",
  neutral: "bg-canvas-tint text-muted",
};

export interface CatalogEmptyDisclosureRowProps extends Omit<React.ComponentProps<"li">, "title"> {
  actions: ReactNode;
  busy?: boolean;
  description: string;
  detail: ReactNode;
  error?: ReactNode;
  icon: ReactNode;
  pill?: ReactNode;
  title: string;
  tone?: CatalogEmptyTone;
}

/**
 * Shared zero-inventory disclosure row for task templates and job suggestions.
 * Collapsed by default; the opener owns review, and actions stay outside it.
 */
export function CatalogEmptyDisclosureRow({
  actions,
  busy = false,
  className,
  description,
  detail,
  error,
  icon,
  pill,
  ref,
  title,
  tone = "neutral",
  ...props
}: CatalogEmptyDisclosureRowProps) {
  return (
    <li
      aria-busy={busy || undefined}
      className={cn("border-t border-line-soft first:border-t-0", className)}
      data-tone={tone}
      ref={ref}
      {...props}
    >
      <Collapsible>
        <div
          className={cn(
            "grid grid-cols-1 items-center gap-3.5 px-4",
            "hover:bg-row-hover",
            "lg:grid-cols-[minmax(0,1fr)_auto]"
          )}
        >
          <CollapsibleTrigger
            className={cn(
              "group/collapsible-trigger grid w-full min-w-0",
              "grid-cols-[12px_28px_minmax(0,1fr)] items-start gap-2.5 py-3 text-left",
              "rounded-sm focus-visible:outline-none focus-visible:shadow-focus-inset"
            )}
          >
            <ChevronRight
              aria-hidden="true"
              className={cn(
                "mt-1 size-3 text-muted transition-transform duration-base ease-out",
                "motion-reduce:transition-none",
                "group-data-panel-open/collapsible-trigger:rotate-90"
              )}
            />
            <span
              aria-hidden="true"
              className={cn(
                "mt-px inline-flex size-7 items-center justify-center rounded-md",
                TONE_CLASS[tone]
              )}
            >
              {icon}
            </span>
            <span className="min-w-0">
              <span className="flex min-w-0 flex-wrap items-center gap-2">
                <b className="truncate text-card-title font-medium tracking-row-title text-fg-strong">
                  {title}
                </b>
                {pill}
              </span>
              <span className="mt-0.5 line-clamp-1 text-small-body text-muted">{description}</span>
            </span>
          </CollapsibleTrigger>
          <div className="flex shrink-0 items-center gap-1.5 pb-3 pl-16 lg:pb-0 lg:pl-0">
            {actions}
          </div>
        </div>
        <CollapsibleContent
          className={cn(
            "h-(--collapsible-panel-height) overflow-hidden",
            "transition-[height] duration-slow ease-out motion-reduce:transition-none",
            "data-ending-style:h-0 data-starting-style:h-0"
          )}
        >
          <div className="flex flex-col gap-2 px-4 pt-0.5 pb-4 lg:pl-20">{detail}</div>
        </CollapsibleContent>
      </Collapsible>
      {error ? <div className="px-4 pb-3 lg:pl-20">{error}</div> : null}
    </li>
  );
}
