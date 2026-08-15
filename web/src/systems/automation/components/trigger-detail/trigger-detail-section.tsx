import type { LucideIcon } from "lucide-react";
import { ChevronDown } from "lucide-react";
import type { ReactNode } from "react";

import { Collapsible, CollapsibleContent, CollapsibleTrigger, Eyebrow, cn } from "@compozy/ui";

interface TriggerDetailSectionProps {
  icon: LucideIcon;
  label: string;
  /** Omitted at zero — a zero count is not a fact worth a chip. */
  count?: number;
  gist?: string;
  defaultOpen?: boolean;
  children: ReactNode;
  "data-testid"?: string;
}

/**
 * Main-column collapsible section: quiet uppercase label, optional count, and a
 * one-line gist so the section still says something while collapsed.
 */
export function TriggerDetailSection({
  icon: Icon,
  label,
  count,
  gist,
  defaultOpen = true,
  children,
  "data-testid": testId,
}: TriggerDetailSectionProps) {
  return (
    <Collapsible data-testid={testId} defaultOpen={defaultOpen}>
      <CollapsibleTrigger
        className={cn(
          "group/section flex w-full items-center gap-2 rounded-xs pb-2.5 text-left",
          "focus-visible:shadow-focus-ring focus-visible:outline-none"
        )}
      >
        <Icon aria-hidden="true" className="size-3.5 shrink-0 text-subtle" strokeWidth={1.75} />
        <Eyebrow className="shrink-0 text-subtle">{label}</Eyebrow>
        {count !== undefined && count > 0 ? (
          <span className="inline-flex h-count-chip min-w-count-chip items-center justify-center rounded-mono-badge bg-canvas-soft px-1.5 font-mono text-mono-id font-medium tabular-nums text-muted">
            {count}
          </span>
        ) : null}
        {gist ? <span className="min-w-0 truncate text-form-hint text-faint">{gist}</span> : null}
        <ChevronDown
          aria-hidden="true"
          className="ml-auto size-3.5 shrink-0 -rotate-90 text-subtle transition-transform duration-base ease-out group-data-panel-open/section:rotate-0"
          strokeWidth={1.75}
        />
      </CollapsibleTrigger>
      <CollapsibleContent>{children}</CollapsibleContent>
    </Collapsible>
  );
}
