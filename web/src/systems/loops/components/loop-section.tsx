import type { ComponentProps, ReactNode } from "react";
import { ChevronDown } from "lucide-react";

import { cn, Collapsible, CollapsibleContent, CollapsibleTrigger, Eyebrow } from "@compozy/ui";

interface LoopSectionProps extends Omit<ComponentProps<typeof Collapsible>, "title"> {
  icon: ReactNode;
  title: string;
  /** One-line takeaway kept on the header while the section is folded. */
  gist?: ReactNode;
  right?: ReactNode;
}

/**
 * Collapsible main-column section: Lucide icon + eyebrow title + optional gist
 * + right slot + chevron. The right slot is a sibling of the triggers so it can
 * hold a link or button. Rail cards stay on `LoopRailSection`.
 */
export function LoopSection({
  icon,
  title,
  gist,
  right,
  defaultOpen = true,
  className,
  children,
  ...props
}: LoopSectionProps) {
  return (
    <Collapsible className={cn("group/sec mb-6", className)} defaultOpen={defaultOpen} {...props}>
      <div className="flex min-h-6 items-center gap-2 pb-2.5">
        <CollapsibleTrigger className="flex min-w-0 flex-1 items-center gap-2 text-left">
          <span className="shrink-0 text-subtle [&_svg]:size-3.5">{icon}</span>
          <Eyebrow className="text-subtle">{title}</Eyebrow>
          {gist ? (
            <span className="min-w-0 shrink truncate text-form-hint text-faint">{gist}</span>
          ) : null}
        </CollapsibleTrigger>
        {right ? <span className="shrink-0">{right}</span> : null}
        <CollapsibleTrigger aria-hidden="true" className="shrink-0 text-faint" tabIndex={-1}>
          <ChevronDown
            aria-hidden="true"
            className={cn(
              "size-3.5 shrink-0 transition-transform -rotate-90",
              "group-data-open/sec:rotate-0 motion-reduce:transition-none"
            )}
          />
        </CollapsibleTrigger>
      </div>
      <CollapsibleContent>{children}</CollapsibleContent>
    </Collapsible>
  );
}
