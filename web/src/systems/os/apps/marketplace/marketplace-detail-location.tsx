import { useNavigate } from "@tanstack/react-router";
import { AlertCircle } from "lucide-react";

import { Button, Empty, useTopbarSlot } from "@agh/ui";
import {
  MarketplaceApiError,
  MarketplaceDetail,
  MarketplaceEntryAction,
  MarketplaceEntryStatus,
  MarketplaceDetailNotFound,
  MarketplaceDetailSkeleton,
  marketplaceRouteKindFor,
  useMarketplaceActionController,
  useMarketplaceEntry,
  type MarketplaceKind,
} from "@/systems/marketplace";
import { useActiveWorkspace } from "@/systems/workspace";
import type { MarketplaceDetailSearch } from "./marketplace-detail-search";

export function MarketplaceDetailLocation({
  kind,
  entryId,
  search,
}: {
  kind: MarketplaceKind;
  entryId: string;
  search: MarketplaceDetailSearch;
}) {
  return <MarketplaceDetailRouteBody entryId={entryId} kind={kind} search={search} />;
}

function MarketplaceDetailRouteBody({
  entryId,
  kind,
  search,
}: {
  entryId: string;
  kind: MarketplaceKind;
  search: MarketplaceDetailSearch;
}) {
  const navigate = useNavigate();
  const { activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = search.scope === "global" ? null : (search.workspace_id ?? activeWorkspaceId);
  const managementScope = search.scope ?? (workspaceId ? "workspace" : "global");
  const query = useMarketplaceEntry(
    kind === "bundle"
      ? { entryId, kind, workspaceId }
      : { entryId, installedName: search.installed_name, kind, workspaceId }
  );
  const actions = useMarketplaceActionController(workspaceId);
  const entry = query.data?.entry;
  const entryName = entry?.name ?? entryId;
  const catalogPath = `/marketplace/${marketplaceRouteKindFor(kind)}` as const;
  const backToCatalog = () => {
    void navigate({ to: catalogPath });
  };

  useTopbarSlot({
    onBack: backToCatalog,
    crumbs: [{ id: "marketplace", label: "Marketplace", onSelect: backToCatalog }],
    crumb: entryName,
    status: entry ? <MarketplaceEntryStatus entry={entry} /> : undefined,
    actions: entry ? (
      <MarketplaceEntryAction
        emphasis="primary"
        entry={entry}
        onAction={actions.handleAction}
        pending={actions.isEntryPending(entry)}
      />
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
        managementScope={managementScope}
        managementWorkspaceId={workspaceId ?? undefined}
      />
      {actions.dialogs}
    </>
  );
}
