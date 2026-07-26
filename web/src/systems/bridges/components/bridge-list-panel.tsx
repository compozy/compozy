import { AlertCircle, Waypoints } from "lucide-react";
import { Link } from "@tanstack/react-router";

import {
  Button,
  CatalogCard,
  Empty,
  ListingRow,
  Pill,
  Skeleton,
  SkeletonRows,
  type ListingViewMode,
} from "@agh/ui";

import {
  bridgeStatusLabel,
  bridgeStatusTone,
  formatBridgeRelativeTime,
} from "../lib/bridge-formatters";
import { effectiveBridgeStatus } from "../lib/bridge-list-filters";
import type { BridgeHealthMap, BridgeSummary } from "../types";

export interface BridgeListPanelProps {
  bridgeHealth: BridgeHealthMap;
  bridges: BridgeSummary[];
  view: ListingViewMode;
  emptyState?: "default" | "filtered";
  paginationStatus?: "available" | "loading";
  onClearFilters: () => void;
  onLoadMore?: () => void;
  status?: "loading" | "ready";
  errorMessage?: string | null;
}

interface BridgeRowProps {
  bridge: BridgeSummary;
  health?: BridgeHealthMap[string];
}

function BridgeMetaFacts({ bridge, health }: BridgeRowProps) {
  const facts: Array<{ id: string; label: string }> = [];
  const relative = formatBridgeRelativeTime(health?.last_success_at);
  if (relative) {
    facts.push({ id: "last-success", label: relative });
  }
  facts.push({ id: "scope", label: bridge.scope });
  if (health?.route_count !== undefined) {
    facts.push({ id: "routes", label: `${health.route_count} routes` });
  }

  if (facts.length === 0) {
    return null;
  }

  return (
    <ListingRow.Meta>
      {facts.map((fact, index) => (
        <span className="inline-flex items-center gap-1.5" key={fact.id}>
          {index > 0 ? <ListingRow.MetaDot /> : null}
          <span className="font-mono text-badge text-subtle">{fact.label}</span>
        </span>
      ))}
    </ListingRow.Meta>
  );
}

function BridgeStatusTrail({ bridge, health }: BridgeRowProps) {
  const status = effectiveBridgeStatus(bridge, health);
  return (
    <>
      <Pill mono size="sm" tone="neutral">
        {bridge.platform}
      </Pill>
      <Pill mono size="sm" tone={bridgeStatusTone(status)}>
        {bridgeStatusLabel(status)}
      </Pill>
    </>
  );
}

function BridgeListingRow({ bridge, health }: BridgeRowProps) {
  return (
    <ListingRow data-bridge={bridge.id} data-testid={`bridge-item-${bridge.id}`}>
      <ListingRow.Link
        render={
          <Link
            aria-label={`Open ${bridge.display_name}`}
            params={{ id: bridge.id }}
            to="/bridges/$id"
          />
        }
      >
        <ListingRow.Icon>
          <Waypoints aria-hidden="true" className="size-4" />
        </ListingRow.Icon>
        <ListingRow.Main>
          <ListingRow.Name>
            <ListingRow.Title>{bridge.display_name}</ListingRow.Title>
            <ListingRow.Slug>{bridge.extension_name}</ListingRow.Slug>
          </ListingRow.Name>
          <BridgeMetaFacts bridge={bridge} health={health} />
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail>
        <BridgeStatusTrail bridge={bridge} health={health} />
      </ListingRow.Trail>
    </ListingRow>
  );
}

function BridgeCatalogCard({ bridge, health }: BridgeRowProps) {
  const status = effectiveBridgeStatus(bridge, health);
  const relative = formatBridgeRelativeTime(health?.last_success_at);

  return (
    <CatalogCard actionable data-bridge={bridge.id} data-testid={`bridge-card-${bridge.id}`}>
      <Link
        aria-label={`Open ${bridge.display_name}`}
        className="flex min-w-0 flex-col gap-3"
        params={{ id: bridge.id }}
        to="/bridges/$id"
      >
        <div className="flex items-start gap-3">
          <CatalogCard.Logo>
            <Waypoints className="size-4" />
          </CatalogCard.Logo>
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <CatalogCard.Title>{bridge.display_name}</CatalogCard.Title>
            <CatalogCard.Meta>
              {relative ? <span>{relative}</span> : null}
              <span>{bridge.scope}</span>
              {health?.route_count !== undefined ? <span>{health.route_count} routes</span> : null}
            </CatalogCard.Meta>
          </div>
        </div>
      </Link>
      <CatalogCard.Actions className="justify-between">
        <Pill mono size="sm" tone="neutral">
          {bridge.platform}
        </Pill>
        <Pill mono size="sm" tone={bridgeStatusTone(status)}>
          {bridgeStatusLabel(status)}
        </Pill>
      </CatalogCard.Actions>
    </CatalogCard>
  );
}

function BridgeListPanel({
  bridgeHealth,
  bridges,
  view,
  emptyState = "default",
  paginationStatus,
  onClearFilters,
  onLoadMore,
  status = "ready",
  errorMessage = null,
}: BridgeListPanelProps) {
  const isEmpty = bridges.length === 0;

  if (status === "loading" && isEmpty) {
    return <BridgeListSkeleton view={view} />;
  }

  if (errorMessage && isEmpty) {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center p-4"
        data-testid="bridge-list-error"
      >
        <Empty
          className="max-w-sm"
          description={errorMessage}
          icon={AlertCircle}
          title="Unable to load bridges"
        />
      </div>
    );
  }

  if (isEmpty) {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center p-4"
        data-testid="bridge-list-empty"
      >
        <Empty
          action={
            emptyState === "filtered" ? (
              <Button
                data-testid="bridge-list-clear-filters"
                onClick={onClearFilters}
                size="sm"
                type="button"
                variant="ghost"
              >
                Clear filters
              </Button>
            ) : undefined
          }
          className="max-w-sm"
          description={
            emptyState === "filtered"
              ? "Try clearing search or filters."
              : "No bridges are configured yet."
          }
          icon={Waypoints}
          title={emptyState === "filtered" ? "No bridges match" : "No bridges yet"}
        />
      </div>
    );
  }

  const catalog =
    view === "cards" ? (
      <div
        className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3"
        data-testid="bridge-list-card-grid"
      >
        {bridges.map(bridge => (
          <BridgeCatalogCard bridge={bridge} health={bridgeHealth[bridge.id]} key={bridge.id} />
        ))}
      </div>
    ) : (
      <div
        className="overflow-hidden rounded-lg border border-line bg-canvas-soft"
        data-testid="bridge-list-rows"
      >
        {bridges.map(bridge => (
          <BridgeListingRow bridge={bridge} health={bridgeHealth[bridge.id]} key={bridge.id} />
        ))}
      </div>
    );

  return (
    <div className="flex flex-col gap-3">
      {catalog}
      {errorMessage ? (
        <p className="text-caption text-danger" data-testid="bridge-list-page-error" role="alert">
          {errorMessage}
        </p>
      ) : null}
      {paginationStatus && onLoadMore ? (
        <div className="flex justify-center">
          <Button
            aria-busy={paginationStatus === "loading"}
            data-testid="bridge-list-load-more"
            disabled={paginationStatus === "loading"}
            onClick={onLoadMore}
            size="sm"
            type="button"
            variant="ghost"
          >
            {paginationStatus === "loading" ? "Loading more bridges…" : "Load more bridges"}
          </Button>
        </div>
      ) : null}
    </div>
  );
}

function BridgeListSkeleton({ view }: { view: ListingViewMode }) {
  return (
    <div
      aria-busy="true"
      aria-label={`Loading bridges as ${view}`}
      data-testid="bridge-list-loading"
      role="status"
    >
      {view === "cards" ? (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {[0, 1, 2, 3, 4, 5].map(index => (
            <Skeleton className="h-32 w-full rounded-lg" key={index} />
          ))}
        </div>
      ) : (
        <SkeletonRows
          className="overflow-hidden rounded-lg border border-line"
          count={5}
          rowClassName="border-b border-line-soft p-3 last:border-b-0"
        >
          <div className="flex items-center gap-3">
            <Skeleton className="size-8 shrink-0" />
            <div className="flex min-w-0 flex-1 flex-col gap-2">
              <Skeleton className="h-3.5 w-2/5" />
              <Skeleton className="h-3 w-1/2" />
            </div>
            <Skeleton className="h-5 w-16" />
            <Skeleton className="h-5 w-20" />
          </div>
        </SkeletonRows>
      )}
    </div>
  );
}

export { BridgeListPanel };
