import { ChevronDown } from "lucide-react";
import type { ReactNode } from "react";

import { cn, Collapsible, CollapsibleContent, CollapsibleTrigger, Pill } from "@compozy/ui";

interface MarketplaceCatalogSectionProps {
  /** Source name, the section title. */
  name: string;
  /** Matching rows in this section (the count chip). */
  count: number;
  /** "n of m matches 'q'" under a query, or the source gist. */
  gist?: string | null;
  children: ReactNode;
}

/**
 * One collapsible section per catalog source, open by default: chevron · title · count · gist.
 * Rendered only when the catalog has two or more sources; a single source is a flat grid.
 */
function MarketplaceCatalogSection({
  name,
  count,
  gist,
  children,
}: MarketplaceCatalogSectionProps) {
  return (
    <Collapsible
      className="flex min-w-0 flex-col"
      data-testid={`marketplace-section-${name}`}
      defaultOpen
      render={<section aria-label={name} />}
    >
      <CollapsibleTrigger
        className={cn(
          "group/marketplace-section -mx-1 flex min-h-9 w-full items-center gap-2 rounded-md px-1 text-left",
          "transition-colors duration-fast hover:bg-row-hover",
          "focus-visible:shadow-focus-inset focus-visible:outline-none"
        )}
        type="button"
      >
        <ChevronDown
          aria-hidden="true"
          className="size-3 shrink-0 -rotate-90 text-faint transition-transform duration-base group-data-panel-open/marketplace-section:rotate-0"
        />
        <span className="text-small-body font-semibold tracking-tight text-fg-strong">{name}</span>
        <Pill mono size="xs">
          {count}
        </Pill>
        {gist ? <span className="min-w-0 truncate text-eyebrow text-subtle">{gist}</span> : null}
      </CollapsibleTrigger>
      <CollapsibleContent className="pt-1.5 pb-1">{children}</CollapsibleContent>
    </Collapsible>
  );
}

export { MarketplaceCatalogSection };
export type { MarketplaceCatalogSectionProps };
