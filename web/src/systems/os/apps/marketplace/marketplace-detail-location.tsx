import { useNavigate } from "@tanstack/react-router";
import { AlertCircle } from "lucide-react";

import { Button, Empty, useTopbarSlot } from "@compozy/ui";

import type { MarketplaceDetailSearch } from "./marketplace-detail-search";
import {
  MarketplaceApiError,
  MarketplaceDetail,
  MarketplaceDetailNotFound,
  MarketplaceDetailSkeleton,
  MarketplaceEntryAction,
  MarketplaceEntryStatus,
  type MarketplaceKind,
  MarketplaceMCPDetailTopbarActions,
  marketplaceRouteKindFor,
  useMarketplaceActionController,
  useMarketplaceEntry,
} from "@/systems/marketplace";
import { useActiveWorkspace } from "@/systems/workspace";

export function MarketplaceDetailLocation({
  kind,
  entryId,
  search,
  liveDataEnabled,
}: {
  kind: MarketplaceKind;
  entryId: string;
  search: MarketplaceDetailSearch;
  liveDataEnabled: boolean;
}) {
  return (
    <MarketplaceDetailRouteBody
      entryId={entryId}
      kind={kind}
      liveDataEnabled={liveDataEnabled}
      search={search}
    />
  );
}

function MarketplaceDetailRouteBody({
  entryId,
  kind,
  search,
  liveDataEnabled,
}: {
  entryId: string;
  kind: MarketplaceKind;
  search: MarketplaceDetailSearch;
  liveDataEnabled: boolean;
}) {
  const navigate = useNavigate();
  const { activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = search.scope === "global" ? null : (search.workspace_id ?? activeWorkspaceId);
  const managementScope = search.scope ?? (workspaceId ? "workspace" : "global");
  const query = useMarketplaceEntry({
    entryId,
    installedName: search.installed_name,
    kind,
    workspaceId,
  });
  const actions = useMarketplaceActionController(workspaceId);
  const entry = query.data?.entry;
  const entryName = entry?.name ?? entryId;
  const catalogPath = `/marketplace/${marketplaceRouteKindFor(kind)}` as const;
  const backToCatalog = () => {
    void navigate({ search: search.tab === "market" ? { tab: "market" } : {}, to: catalogPath });
  };

  const mcpInstalled = Boolean(entry && kind === "mcp" && entry.installed);
  useTopbarSlot({
    onBack: backToCatalog,
    crumbs: [{ id: "marketplace", label: "Marketplace", onSelect: backToCatalog }],
    crumb: entryName,
    status: entry && !mcpInstalled ? <MarketplaceEntryStatus entry={entry} /> : undefined,
    actions: entry ? (
      mcpInstalled ? (
        <MarketplaceMCPDetailTopbarActions
          entry={entry}
          liveDataEnabled={liveDataEnabled}
          onAction={actions.handleAction}
          pending={actions.isEntryPending(entry)}
          scope={managementScope}
          workspaceId={workspaceId ?? undefined}
        />
      ) : (
        <MarketplaceEntryAction
          emphasis="primary"
          entry={entry}
          onAction={actions.handleAction}
          pending={actions.isEntryPending(entry)}
        />
      )
    ) : undefined,
  });

  if (query.isLoading) return <MarketplaceDetailSkeleton />;
  if (query.error instanceof MarketplaceApiError && query.error.status === 404) {
    return <MarketplaceDetailNotFound onBack={backToCatalog} />;
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
      <MarketplaceDetail
        data={query.data}
        liveDataEnabled={liveDataEnabled}
        managementScope={managementScope}
        managementWorkspaceId={workspaceId ?? undefined}
      />
      {actions.dialogs}
    </>
  );
}
