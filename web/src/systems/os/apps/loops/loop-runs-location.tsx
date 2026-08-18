import type { ReactNode } from "react";
import { Activity, AlertCircle } from "lucide-react";

import {
  Button,
  Empty,
  ListingPage,
  ListingToolbar,
  NativeSelect,
  NativeSelectOption,
  SkeletonRows,
  useTopbarSlot,
} from "@compozy/ui";
import { useLoopRunsRoute, type LoopRunsRouteSearch } from "./use-loop-runs-route";

import {
  LOOP_NODE_INVENTORY_LABELS,
  LOOP_NODE_INVENTORY_STATES,
  LoopNodeInventoryView,
  LoopRunsView,
  useLoopRunPendingRequestCounts,
  useNowTick,
} from "@/systems/loops";

export function LoopRunsLocation({ search }: { search: LoopRunsRouteSearch }) {
  const {
    outcome,
    runsQuery,
    setOrigin,
    setOriginSession,
    setOutcome,
    workspaceId,
    inventoryState,
    inventory,
    loopOptions,
    runOptions,
    setInventoryState,
    setInventoryLoop,
    setInventoryRun,
    setInventoryView,
    clearInventoryFilters,
  } = useLoopRunsRoute(search);
  const nowMs = useNowTick(inventoryState !== undefined);

  const runs = runsQuery.data?.runs ?? [];
  const pendingRequestCounts = useLoopRunPendingRequestCounts(
    workspaceId,
    runs.map(run => run.id),
    workspaceId !== ""
  );
  const showToolbar = workspaceId !== "" && !runsQuery.isLoading && !runsQuery.error;

  useTopbarSlot({
    glyph: <Activity />,
    count: showToolbar ? runs.length : undefined,
    toolbar: showToolbar ? (
      <ListingToolbar data-testid="loop-runs-origin-toolbar">
        <ListingToolbar.Leading>
          <NativeSelect
            aria-label="Runs view"
            data-testid="loop-runs-view-switch"
            onChange={event =>
              setInventoryState(
                event.target.value === "runs"
                  ? undefined
                  : (event.target.value as (typeof LOOP_NODE_INVENTORY_STATES)[number])
              )
            }
            value={inventoryState ?? "runs"}
          >
            <NativeSelectOption value="runs">Runs</NativeSelectOption>
            {LOOP_NODE_INVENTORY_STATES.map(value => (
              <NativeSelectOption key={value} value={value}>
                {LOOP_NODE_INVENTORY_LABELS[value]} nodes
              </NativeSelectOption>
            ))}
          </NativeSelect>
          <NativeSelect
            aria-label="Run origin"
            data-testid="loop-runs-origin-filter"
            value={search.origin ?? "all"}
            onChange={event =>
              setOrigin(
                event.target.value === "all"
                  ? undefined
                  : (event.target.value as "catalog" | "session")
              )
            }
          >
            <NativeSelectOption value="all">All origins</NativeSelectOption>
            <NativeSelectOption value="catalog">Catalog</NativeSelectOption>
            <NativeSelectOption value="session">Session</NativeSelectOption>
          </NativeSelect>
          {search.origin === "session" ? (
            <input
              aria-label="Origin session id"
              className="h-8 min-w-search-input rounded-md border border-line bg-input-fill px-2.5 font-mono text-small-body text-fg outline-none placeholder:text-faint focus:border-line-strong"
              data-testid="loop-runs-origin-session-filter"
              onChange={event => setOriginSession(event.target.value.trim())}
              placeholder="Session id"
              value={search.origin_session ?? ""}
            />
          ) : null}
        </ListingToolbar.Leading>
      </ListingToolbar>
    ) : undefined,
  });

  if (workspaceId === "") {
    return (
      <RunsState
        description="Select a workspace to view its Loop runs."
        testId="loop-runs-no-workspace"
        title="No workspace selected"
      />
    );
  }

  // The inventory is a peer view of the same runs area, not a nested state of
  // the roster — it owns its own loading, empty, and paging surfaces.
  if (inventoryState !== undefined) {
    if (inventory.isError) {
      return (
        <RunsState
          action={
            <Button onClick={inventory.refetch} size="sm" type="button" variant="outline">
              Retry inventory
            </Button>
          }
          description={inventory.error?.message ?? "The node inventory could not be loaded."}
          icon={AlertCircle}
          role="alert"
          testId="loop-runs-inventory-error"
          title="Unable to load node inventory"
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
          loopOptions={loopOptions}
          nowMs={nowMs}
          onClearFilters={clearInventoryFilters}
          onLoadMore={inventory.fetchNextPage}
          onLoopFilterChange={setInventoryLoop}
          onRunFilterChange={setInventoryRun}
          onStateChange={setInventoryState}
          onViewChange={setInventoryView}
          runFilter={search.nodes_run ?? ""}
          runOptions={runOptions}
          state={inventoryState}
          view={search.view ?? "rows"}
        />
      </ListingPage>
    );
  }

  if (runsQuery.isLoading) {
    return (
      <div className="min-h-0 flex-1 overflow-hidden p-5" data-testid="loop-runs-loading">
        <SkeletonRows count={6} rowClassName="border-b border-line-soft py-3" />
      </div>
    );
  }

  if (runsQuery.error) {
    return (
      <RunsState
        description={runsQuery.error.message ?? "Failed to load loop runs"}
        icon={AlertCircle}
        testId="loop-runs-error"
        title="Unable to load runs"
      />
    );
  }

  if (runs.length === 0 && !search.origin && !search.origin_session) {
    return (
      <RunsState
        description="No Loop has run in this workspace yet."
        testId="loop-runs-empty"
        title="No runs yet"
      />
    );
  }

  return (
    <ListingPage data-testid="loop-runs">
      {runs.length === 0 ? (
        <Empty
          className="mx-auto max-w-md"
          description="Adjust the origin filters to include more runs."
          icon={Activity}
          title="No matching runs"
        />
      ) : (
        <LoopRunsView
          onOutcomeChange={setOutcome}
          outcome={outcome}
          pendingRequestCounts={pendingRequestCounts}
          runs={runs}
        />
      )}
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
