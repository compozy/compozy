import { ChevronRight } from "lucide-react";
import type { ReactNode } from "react";

import { cn, Collapsible, CollapsibleContent, CollapsibleTrigger } from "@compozy/ui";

interface KnowledgeFoldProps {
  label: ReactNode;
  children: ReactNode;
  defaultOpen?: boolean;
  className?: string;
  "data-testid"?: string;
  toggleTestId?: string;
}

/**
 * Closed-by-default disclosure for secondary knowledge detail (facts, history).
 * The chevron toggle keeps the reading view calm while leaving every field one
 * click away.
 */
function KnowledgeFold({
  label,
  children,
  defaultOpen = false,
  className,
  "data-testid": testId,
  toggleTestId,
}: KnowledgeFoldProps) {
  return (
    <Collapsible
      className={cn("flex min-w-0 flex-col", className)}
      data-testid={testId}
      defaultOpen={defaultOpen}
    >
      <CollapsibleTrigger
        className={cn(
          "group/knowledge-fold flex w-full items-center gap-2 rounded-sm py-1 text-left text-small-body font-medium text-muted",
          "transition-colors duration-base hover:text-fg",
          "focus-visible:outline-none focus-visible:shadow-focus-ring"
        )}
        data-testid={toggleTestId}
        type="button"
      >
        <ChevronRight
          aria-hidden="true"
          className="size-3.5 text-faint transition-transform duration-base group-data-panel-open/knowledge-fold:rotate-90 motion-reduce:transition-none"
        />
        {label}
      </CollapsibleTrigger>
      <CollapsibleContent className="pt-3" keepMounted>
        {children}
      </CollapsibleContent>
    </Collapsible>
  );
}

export { KnowledgeFold };
export type { KnowledgeFoldProps };
