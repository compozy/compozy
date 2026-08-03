import type { ComponentProps, ReactNode } from "react";
import { ChevronRight } from "lucide-react";

import { cn, Collapsible, CollapsibleContent, CollapsibleTrigger } from "@compozy/ui";

interface LoopRailSectionProps extends Omit<ComponentProps<typeof Collapsible>, "title"> {
  title: string;
  /** The takeaway a reader gets without opening — never a bare count. */
  gist?: ReactNode;
  icon?: ReactNode;
}

/**
 * A rail card that stays informative while folded.
 *
 * Collapse is the density control on these pages, so the summary line carries the
 * answer (`50 generations · no budgets set`) and opening only adds detail. At most
 * one rail section on a page opens by default.
 */
export function LoopRailSection({
  title,
  gist,
  icon,
  className,
  children,
  ...props
}: LoopRailSectionProps) {
  return (
    <Collapsible
      className={cn("rounded-lg border border-line bg-canvas-soft", className)}
      {...props}
    >
      <CollapsibleTrigger className="group/rail-trigger flex w-full items-center gap-2 px-3.5 py-2.5 text-left">
        {icon ? <span className="shrink-0 text-muted">{icon}</span> : null}
        <span className="text-form-label font-medium text-fg-strong">{title}</span>
        <span className="min-w-0 flex-1 truncate text-right text-form-hint text-subtle">
          {gist}
        </span>
        <ChevronRight
          aria-hidden="true"
          className="size-3.5 shrink-0 text-faint transition-transform group-data-panel-open/rail-trigger:rotate-90"
        />
      </CollapsibleTrigger>
      <CollapsibleContent className="border-t border-line-soft">{children}</CollapsibleContent>
    </Collapsible>
  );
}
