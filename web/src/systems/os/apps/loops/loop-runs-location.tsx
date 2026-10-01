import type { ReactNode } from "react";
import { Activity, AlertCircle } from "lucide-react";

import { useNavigate } from "@tanstack/react-router";

import {
  Button,
  Empty,
  ListingPage,
  ListingToolbar,
  PillGroup,
  SkeletonRows,
  useTopbarSlot,
} from "@compozy/ui";
import { loopRunsTrail } from "./loop-window-crumbs";
import { useLoopRunsRoute, type LoopRunsRouteSearch } from "./use-loop-runs-route";

import {
  LOOP_NODE_INVENTORY_STATES,
  LoopNodeInventoryView,
  LoopRunsFilters,
  LoopRunsView,
  useNowTick,
} from "@/systems/loops";

type LoopRunsRoute = ReturnType<typeof useLoopRunsRoute>;

export function LoopRunsLocation({ search }: { search: LoopRunsRouteSearch }) {
  const route = useLoopRunsRoute(search);
  const { runsQuery, workspaceId, inventoryState } = route;
  const navigate = useNavigate();
  const openLoops = () => {
    void navigate({ to: "/loops" });
  };
  // Inventory ages and roster durations tick on one clock.
  const nowMs = useNowTick(true);
  const showToolbar = workspaceId !== "" && !runsQuery.isLoading && !runsQuery.error;

  useTopbarSlot({
    ...loopRunsTrail({ level: "runs", onBack: openLoops, openLoops }),
    toolbar: showToolbar ? <LoopRunsToolbar route={route} search={search} /> : undefined,
  });

  if (workspaceId === "") {
    return (
      <RunsState
        description="Select a project to view its Loop runs."
        testId="loop-runs-no-workspace"
        title="No project selected"
      />
    );
  }

  // The inventory is a peer view of the same runs area, not a nested state of
  // the roster — it owns its own loading, empty, and paging surfaces.
  if (inventoryState !== undefined) {
    return (
      <LoopRunsInventoryPane nowMs={nowMs} route={route} search={search} state={inventoryState} />
    );
  }
  return <LoopRunsRosterPane nowMs={nowMs} openLoops={openLoops} route={route} />;
}

function LoopRunsToolbar({ route, search }: { route: LoopRunsRoute; search: LoopRunsRouteSearch }) {
  const { inventoryState, outcome, setInventoryState, setOriginFilter, setOutcome } = route;
  return (
    <ListingToolbar data-testid="loop-runs-origin-toolbar">
      <ListingToolbar.Leading>
        <PillGroup<"runs" | "nodes">
          aria-label="Runs view"
          data-testid="loop-runs-view-switch"
          items={[
            { value: "runs", label: "Runs", testId: "loop-runs-view-runs" },
            { value: "nodes", label: "Steps", testId: "loop-runs-view-nodes" },
          ]}
          onChange={next =>
            setInventoryState(next === "runs" ? undefined : LOOP_NODE_INVENTORY_STATES[0])
          }
          value={inventoryState === undefined ? "runs" : "nodes"}
        />
        {inventoryState === undefined ? (
          <ListingToolbar.Filters>
            <LoopRunsFilters
              onOriginFilterChange={setOriginFilter}
              onOutcomeChange={setOutcome}
              origin={search.origin}
              originSession={search.origin_session}
              outcome={outcome}
            />
          </ListingToolbar.Filters>
        ) : null}
      </ListingToolbar.Leading>
    </ListingToolbar>
  );
}

function LoopRunsInventoryPane({
  route,
  search,
  state,
  nowMs,
}: {
  route: LoopRunsRoute;
  search: LoopRunsRouteSearch;
  state: NonNullable<LoopRunsRoute["inventoryState"]>;
  nowMs: number;
}) {
  const { inventory } = route;
  if (inventory.isError) {
    return (
      <RunsState
        action={
          <Button onClick={inventory.refetch} size="sm" type="button" variant="secondary">
            Try again
          </Button>
        }
        description={inventory.error?.message ?? "The step list could not be loaded."}
        icon={AlertCircle}
        role="alert"
        testId="loop-runs-inventory-error"
        title="Couldn't load steps"
      />
    );
  }
  return (
    <ListingPage data-testid="loop-runs-inventory">
      <LoopNodeInventoryView
        hasMore={inventory.hasMore}
        isFetchingNextPage={inventory.isFetchingNextPage}
        isLoading={inventory.isLoading}
        items={inventory.items}
        loadedCount={inventory.loadedCount}
        loopFilter={search.nodes_loop ?? ""}
        loopOptions={route.loopOptions}
        nowMs={nowMs}
        onClearFilters={route.clearInventoryFilters}
        onLoadMore={inventory.fetchNextPage}
        onLoopFilterChange={route.setInventoryLoop}
        onRunFilterChange={route.setInventoryRun}
        onStateChange={route.setInventoryState}
        onViewChange={route.setInventoryView}
        runFilter={search.nodes_run ?? ""}
        runOptions={route.runOptions}
        state={state}
        view={search.view ?? "rows"}
      />
    </ListingPage>
  );
}

function LoopRunsRosterPane({
  route,
  nowMs,
  openLoops,
}: {
  route: LoopRunsRoute;
  nowMs: number;
  openLoops: () => void;
}) {
  const { runsQuery, outcome, setOutcome, profile } = route;
  if (runsQuery.isLoading) {
    return (
      <div className="min-h-0 flex-1 overflow-hidden p-5" data-testid="loop-runs-loading">
        <SkeletonRows count={6} rowClassName="border-b border-line-soft py-3" />
      </div>
    );
  }
  // This roster is polled, not streamed, so "reconnecting" cannot mean a dropped
  // subscription — and a healthy 15s poll is not degraded either. The one state
  // that honestly reads as reconnecting is a read that has already failed and is
  // being retried right now. The two are kept mutually exclusive so the notice
  // never tells a reader to wait for a retry that has already settled into an
  // error they have to act on.
  const isRetrying = runsQuery.isFetching && runsQuery.failureCount > 0;
  const isReadFailed = Boolean(runsQuery.error) && !isRetrying;

  // A failed read is degraded transport, not an empty workspace: the rows below
  // are the last good read, and the roster keeps showing them while saying so
  // (task_05 requirement 5, VC-36). Emptiness is the roster model's call too —
  // it is the one place that knows whether a filter is hiding the rows.
  return (
    <ListingPage data-testid="loop-runs">
      <LoopRunsView
        isError={isReadFailed}
        isReconnecting={isRetrying}
        // The cache's own last-success stamp, so the age the notice prints is
        // when these rows were read rather than when the page happened to mount.
        lastReadAt={
          runsQuery.dataUpdatedAt > 0 ? new Date(runsQuery.dataUpdatedAt).toISOString() : undefined
        }
        nowMs={nowMs}
        onEmptyAction={outcome === "all" ? openLoops : () => setOutcome("all")}
        onRetry={() => void runsQuery.refetch()}
        outcome={outcome}
        profileScope={profile}
        runs={runsQuery.data?.runs ?? []}
      />
    </ListingPage>
  );
}

interface RunsStateProps {
  title: string;
  description: string;
  testId: string;
  icon?: typeof Activity;
  action?: ReactNode;
  role?: "alert";
}

function RunsState({ title, description, testId, icon = Activity, action, role }: RunsStateProps) {
  return (
    <div
      className="flex min-h-0 flex-1 items-center justify-center py-10"
      data-testid={testId}
      role={role}
    >
      <Empty
        action={action}
        className="max-w-md"
        description={description}
        icon={icon}
        title={title}
      />
    </div>
  );
}
