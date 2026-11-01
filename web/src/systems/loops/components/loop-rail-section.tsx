import type { ComponentProps, ReactNode } from "react";
import { ChevronRight } from "@compozy/ui/icons";

import { cn, Collapsible, CollapsibleContent, CollapsibleTrigger } from "@compozy/ui";

const RAIL_TRIGGER_CLASS = "flex w-full items-center gap-2 px-4 py-3 text-left";
const RAIL_TITLE_CLASS = "text-form-label font-medium text-fg-strong";
const RAIL_GIST_CLASS = "min-w-0 flex-1 truncate text-right text-form-hint text-subtle";

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
 * answer (`50 rounds · no budgets`) and opening only adds detail. At most
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
    <Collapsible className={cn("rounded-lg bg-card shadow-card", className)} {...props}>
      <CollapsibleTrigger className={cn("group/rail-trigger", RAIL_TRIGGER_CLASS)}>
        {icon ? <span className="shrink-0 text-muted-foreground">{icon}</span> : null}
        <span className={RAIL_TITLE_CLASS}>{title}</span>
        <span className={RAIL_GIST_CLASS}>{gist}</span>
        <ChevronRight
          aria-hidden="true"
          className="size-3.5 shrink-0 text-faint transition-transform group-data-panel-open/rail-trigger:rotate-90"
        />
      </CollapsibleTrigger>
      <CollapsibleContent className="border-t border-line-soft">{children}</CollapsibleContent>
    </Collapsible>
  );
}

interface LoopRailSectionSkeletonProps extends Omit<ComponentProps<"div">, "title"> {
  title: string;
  icon?: ReactNode;
  /** Placeholder for the gist the loaded section prints beside its title. */
  gist?: ReactNode;
  /** Shows the body under the header, as a section that opens by default does. */
  open?: boolean;
}

/**
 * A `LoopRailSection` while its content is being read: the real header (static icon and title)
 * without the fold behaviour, and a body only when the loaded section opens by default.
 */
export function LoopRailSectionSkeleton({
  title,
  icon,
  gist,
  open = false,
  className,
  children,
  ...props
}: LoopRailSectionSkeletonProps) {
  return (
    <div className={cn("rounded-lg bg-card shadow-card", className)} {...props}>
      <div className={RAIL_TRIGGER_CLASS}>
        {icon ? <span className="shrink-0 text-muted-foreground">{icon}</span> : null}
        <span className={RAIL_TITLE_CLASS}>{title}</span>
        <span className={RAIL_GIST_CLASS}>{gist}</span>
        <ChevronRight
          aria-hidden="true"
          className={cn("size-3.5 shrink-0 text-faint", open && "rotate-90")}
        />
      </div>
      {open ? <div className="border-t border-line-soft">{children}</div> : null}
    </div>
  );
}
