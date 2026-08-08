import { PackageX } from "lucide-react";

import { Button, cn, Empty, PAGE_CONTENT_GUTTER, Skeleton } from "@compozy/ui";

import type { MarketplaceEntryResponse } from "../types";
import { MarketplaceDetailExtensionView } from "./marketplace-detail-extension";
import { MarketplaceDetailLede } from "./marketplace-detail-lede";
import { MarketplaceDetailMCPView } from "./marketplace-detail-mcp";
import { MarketplaceDetailSkillView } from "./marketplace-detail-skill";

interface MarketplaceDetailProps {
  data: MarketplaceEntryResponse;
  managementScope?: "global" | "workspace";
  managementWorkspaceId?: string;
}

/**
 * Marketplace entry detail. Each kind fills the body with its own story —
 * readme, authorization, or kit — and keeps the rail to short property cards.
 * Window identity and the primary action live in the OS head.
 */
function MarketplaceDetail({
  data,
  managementScope,
  managementWorkspaceId,
}: MarketplaceDetailProps) {
  const kind = data.entry.kind;
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto" data-testid="marketplace-detail">
      <div className={cn(PAGE_CONTENT_GUTTER, "flex flex-col pt-6 pb-20")}>
        <MarketplaceDetailLede data={data} />
        {kind === "skill" ? <MarketplaceDetailSkillView data={data} /> : null}
        {kind === "mcp" ? (
          <MarketplaceDetailMCPView
            data={data}
            scope={managementScope}
            workspaceId={managementWorkspaceId}
          />
        ) : null}
        {kind === "extension" ? <MarketplaceDetailExtensionView data={data} /> : null}
      </div>
    </div>
  );
}

function MarketplaceDetailSkeleton() {
  return (
    <div className={cn(PAGE_CONTENT_GUTTER, "flex flex-col pt-6 pb-20")} role="status">
      <div className="mb-6 flex items-start gap-3.5 border-b border-line pb-5">
        <Skeleton className="size-(--size-provider-logo-well) shrink-0 rounded-lg" />
        <div className="flex min-w-0 flex-1 flex-col gap-3">
          <Skeleton className="h-5.5 w-55 max-w-full" />
          <Skeleton className="h-3 w-75 max-w-full" />
          <Skeleton className="h-3 w-95 max-w-full" />
        </div>
      </div>
      <div className="grid grid-cols-1 gap-8 lg:grid-cols-[minmax(0,1fr)_20rem]">
        <div className="flex flex-col gap-2.5 rounded-lg border border-line bg-canvas-soft p-5">
          {[78, 91, 66, 82, 94, 72].map(width => (
            <Skeleton className="h-2.5" key={width} style={{ width: `${width}%` }} />
          ))}
        </div>
        <aside className="flex flex-col gap-3">
          {[0, 1].map(card => (
            <div
              className="overflow-hidden rounded-lg border border-line bg-canvas-soft"
              key={card}
            >
              <div className="border-b border-line-soft px-3.5 py-3">
                <Skeleton className="h-2.5 w-24" />
              </div>
              <div className="flex flex-col gap-2.5 px-3.5 py-3">
                {[38, 54, 46].map(width => (
                  <Skeleton
                    className="h-2.5"
                    key={width}
                    style={{ width: `${width + card * 8}%` }}
                  />
                ))}
              </div>
            </div>
          ))}
        </aside>
      </div>
      <span className="sr-only">Loading marketplace entry</span>
    </div>
  );
}

function MarketplaceDetailNotFound({ onBack }: { onBack: () => void }) {
  return (
    <div className={cn(PAGE_CONTENT_GUTTER, "flex flex-col pb-20")}>
      <Empty
        fill={false}
        className="py-14"
        titleAs="h2"
        icon={PackageX}
        title="This item is no longer in the marketplace"
        description="The catalog source no longer returns this entry."
        action={
          <Button onClick={onBack} size="sm" type="button" variant="neutral">
            Back to marketplace
          </Button>
        }
      />
    </div>
  );
}

export { MarketplaceDetail, MarketplaceDetailNotFound, MarketplaceDetailSkeleton };
export type { MarketplaceDetailProps };
