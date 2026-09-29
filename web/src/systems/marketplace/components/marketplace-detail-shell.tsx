import { ChevronDown, type LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import {
  cn,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Eyebrow,
  Panel,
  PropertyRow,
} from "@compozy/ui";

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
        <Panel bodyClassName="p-0" className="overflow-hidden">
          {children}
        </Panel>
      </CollapsibleContent>
    </Collapsible>
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

/** Rail collapsible property card: a flat panel with a hover-lit summary over a short body. */
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
      data-testid={testId}
      defaultOpen={defaultOpen}
      render={<Panel aria-label={title} bodyClassName="p-0" className="overflow-hidden" />}
    >
      <CollapsibleTrigger
        className={cn(
          "group/detail-rail flex w-full items-center gap-2 px-3.5 py-2.5 text-left",
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

/** Rail property row linking to the entry's repository as an owner/repo slug. */
function MarketplaceRepositoryRow({ repository }: { repository: string | undefined }) {
  const url = repository?.trim();
  if (!url) return null;
  return (
    <PropertyRow label="Repository" valueTitle={url}>
      <a
        className="min-w-0 truncate font-mono text-eyebrow text-muted transition-colors duration-base hover:text-fg-strong"
        href={url}
        rel="noreferrer"
        target="_blank"
      >
        {formatRepositorySlug(url)} ↗
      </a>
    </PropertyRow>
  );
}

function formatRepositorySlug(url: string): string {
  try {
    const parsed = new URL(url);
    const path = parsed.pathname.replace(/^\/+|\/+$/g, "");
    return path || parsed.host;
  } catch {
    return url;
  }
}

export {
  MarketplaceDetailColumns,
  MarketplaceDetailRailCard,
  MarketplaceDetailRailNote,
  MarketplaceDetailSection,
  MarketplaceRepositoryRow,
};
export type { MarketplaceDetailRailCardProps, MarketplaceDetailSectionProps };
