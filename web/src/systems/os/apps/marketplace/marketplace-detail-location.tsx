import { useNavigate } from "@tanstack/react-router";
import { AlertCircle } from "lucide-react";

import { Button, Empty, useTopbarSlot } from "@compozy/ui";

import { useMarketplaceDetailScope } from "./hooks/use-marketplace-detail-scope";
import type { MarketplaceDetailSearch } from "./marketplace-detail-search";
import {
  MarketplaceApiError,
  MarketplaceCatalogTrail,
  MarketplaceDetail,
  MarketplaceDetailNotFound,
  MarketplaceDetailSkeleton,
  useMarketplaceActionController,
  useMarketplaceCatalogEntry,
} from "@/systems/marketplace";

export function MarketplaceDetailLocation({
  entryId,
  search,
  liveDataEnabled,
}: {
  entryId: string;
  search: MarketplaceDetailSearch;
  liveDataEnabled: boolean;
}) {
  const navigate = useNavigate();
  const { profileName, workspaceId } = useMarketplaceDetailScope(search);
  const query = useMarketplaceCatalogEntry(
    {
      entryId,
      installedName: search.installed_name,
      profileName,
      source: search.source,
      workspaceId,
    },
    liveDataEnabled
  );
  const actions = useMarketplaceActionController();
  const entry = query.data?.entry;
  const entryName = entry?.name ?? entryId;
  const referrer = search.from === "installed" ? "Installed" : "Marketplace";
  const back = () => {
    void navigate({
      search: { q: search.q },
      to: search.from === "installed" ? "/marketplace/installed" : "/marketplace",
    });
  };

  useTopbarSlot({
    onBack: back,
    crumbs: [{ id: "marketplace", label: referrer, onSelect: back }],
    crumb: entryName,
    actions: entry ? (
      <MarketplaceCatalogTrail
        emphasis="primary"
        entry={entry}
        onInstall={actions.install}
        onUpdate={actions.update}
        pending={actions.isEntryPending(entry)}
      />
    ) : undefined,
  });

  if (query.isLoading) return <MarketplaceDetailSkeleton />;
  if (query.error instanceof MarketplaceApiError && query.error.status === 404) {
    return <MarketplaceDetailNotFound onBack={back} />;
  }
  if (query.error || !query.data) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center px-6 py-10">
        <Empty
          framed
          titleAs="h2"
          icon={AlertCircle}
          title="Unable to load this item"
          description="The marketplace entry could not be loaded."
          cause={query.error?.message}
          action={<Button onClick={() => void query.refetch()}>Retry</Button>}
        />
      </div>
    );
  }
  return (
    <>
      <MarketplaceDetail data={query.data} />
      {actions.dialogs}
    </>
  );
}
