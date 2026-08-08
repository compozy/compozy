import { ChevronDown, type LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { cn, Collapsible, CollapsibleContent, CollapsibleTrigger, Eyebrow } from "@compozy/ui";

/**
 * Marketplace detail anatomy (OpenDesign marketplace contract): the kind-specific
 * content is the body — collapsible sections over a shared panelbox — and the
 * 320px rail holds short collapsible property cards.
 */

interface MarketplaceDetailColumnsProps {
  main: ReactNode;
  rail: ReactNode;
}

function MarketplaceDetailColumns({ main, rail }: MarketplaceDetailColumnsProps) {
  return (
    <div className="grid grid-cols-1 items-start gap-8 lg:grid-cols-[minmax(0,1fr)_var(--width-detail-inspector-inline)]">
      <main className="flex min-w-0 flex-col gap-5.5">{main}</main>
      <aside className="flex min-w-0 flex-col gap-3">{rail}</aside>
    </div>
  );
}

interface MarketplaceDetailSectionProps {
  icon: LucideIcon;
  title: string;
  summary?: ReactNode;
  defaultOpen?: boolean;
  children: ReactNode;
  "data-testid"?: string;
}

/** Main-column collapsible section: quiet summary row above a framed panel. */
function MarketplaceDetailSection({
  icon: Icon,
  title,
  summary,
  defaultOpen = true,
  children,
  "data-testid": testId,
}: MarketplaceDetailSectionProps) {
  return (
    <Collapsible
      className="flex min-w-0 flex-col"
      data-testid={testId}
      defaultOpen={defaultOpen}
      render={<section aria-label={title} />}
    >
      <CollapsibleTrigger
        className={cn(
          "group/detail-section flex w-full items-center gap-2 pb-2.5 text-left",
          "rounded-sm focus-visible:shadow-focus-ring focus-visible:outline-none"
        )}
        type="button"
      >
        <Icon aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />
        <Eyebrow className="text-subtle">{title}</Eyebrow>
        {summary !== undefined && summary !== null ? (
          <span className="min-w-0 truncate text-transcript-meta text-faint">{summary}</span>
        ) : null}
        <span aria-hidden="true" className="flex-1" />
        <ChevronDown
          aria-hidden="true"
          className={cn(
            "size-3.5 shrink-0 -rotate-90 text-faint",
            "transition-transform duration-base group-data-panel-open/detail-section:rotate-0"
          )}
        />
      </CollapsibleTrigger>
      <CollapsibleContent>
        <MarketplaceDetailPanel>{children}</MarketplaceDetailPanel>
      </CollapsibleContent>
    </Collapsible>
  );
}

interface MarketplaceDetailPanelProps {
  children: ReactNode;
  className?: string;
}

/** The shared panelbox frame for main-column section bodies. */
function MarketplaceDetailPanel({ children, className }: MarketplaceDetailPanelProps) {
  return (
    <div className={cn("overflow-hidden rounded-lg border border-line bg-canvas-soft", className)}>
      {children}
    </div>
  );
}

interface MarketplaceDetailKvRowProps {
  label: string;
  sub?: ReactNode;
  mono?: boolean;
  children: ReactNode;
}

/** Panel key/value row: eyebrow key over a plain-language value plus optional note. */
function MarketplaceDetailKvRow({
  label,
  sub,
  mono = false,
  children,
}: MarketplaceDetailKvRowProps) {
  return (
    <div className="border-t border-line-soft px-4 py-3 first:border-t-0">
      <Eyebrow className="mb-1.5 block text-faint">{label}</Eyebrow>
      <div
        className={cn(
          "min-w-0 text-small-body text-fg",
          mono && "font-mono text-form-hint text-muted"
        )}
      >
        {children}
      </div>
      {sub !== undefined && sub !== null ? (
        <p className="mt-1 text-form-label leading-relaxed text-subtle">{sub}</p>
      ) : null}
    </div>
  );
}

interface MarketplaceDetailRailCardProps {
  icon: LucideIcon;
  title: string;
  summary?: ReactNode;
  defaultOpen?: boolean;
  children: ReactNode;
  "data-testid"?: string;
}

/** Rail collapsible property card: framed, hover-lit summary, short body. */
function MarketplaceDetailRailCard({
  icon: Icon,
  title,
  summary,
  defaultOpen = true,
  children,
  "data-testid": testId,
}: MarketplaceDetailRailCardProps) {
  return (
    <Collapsible
      className="overflow-hidden rounded-lg border border-line bg-canvas-soft"
      data-testid={testId}
      defaultOpen={defaultOpen}
      render={<section aria-label={title} />}
    >
      <CollapsibleTrigger
        className={cn(
          "group/detail-rail flex w-full items-center gap-2 px-3.5 py-2.75 text-left",
          "transition-colors duration-base hover:bg-row-hover",
          "focus-visible:shadow-focus-inset focus-visible:outline-none"
        )}
        type="button"
      >
        <Icon aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />
        <span className="shrink-0 text-form-label font-semibold text-fg-strong">{title}</span>
        <span className="min-w-0 flex-1 truncate text-right text-transcript-caption text-faint">
          {summary}
        </span>
        <ChevronDown
          aria-hidden="true"
          className={cn(
            "size-3 shrink-0 -rotate-90 text-faint",
            "transition-transform duration-base group-data-panel-open/detail-rail:rotate-0"
          )}
        />
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className="border-t border-line-soft pt-1 pb-2">{children}</div>
      </CollapsibleContent>
    </Collapsible>
  );
}

/** Quiet explanatory note inside a rail card body. */
function MarketplaceDetailRailNote({ children }: { children: ReactNode }) {
  return (
    <p className="px-3.5 pt-1.5 pb-1 text-transcript-caption leading-relaxed text-faint">
      {children}
    </p>
  );
}

export {
  MarketplaceDetailColumns,
  MarketplaceDetailKvRow,
  MarketplaceDetailPanel,
  MarketplaceDetailRailCard,
  MarketplaceDetailRailNote,
  MarketplaceDetailSection,
};
export type { MarketplaceDetailRailCardProps, MarketplaceDetailSectionProps };
