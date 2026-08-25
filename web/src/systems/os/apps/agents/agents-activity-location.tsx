/**
 * The Activity location — every delegation tree in the workspace, live.
 *
 * This is where the shell's two liveness facts are read and handed down: whether
 * this window is visible (`useWindowLiveDataEnabled`) and whether the session
 * catalog stream is connected. The agent-comms system takes both as inputs
 * rather than reading them itself, because that stream's hook owns its socket —
 * a second caller's unmount would close the shell's connection.
 */
import { AlertCircle, GitBranch, RefreshCw, WifiOff } from "lucide-react";

import { Button, Empty, ListingPage, useTopbarSlot } from "@compozy/ui";

import { AgentCallTree } from "@/systems/agent-comms";

import { useAgentsActivity } from "./use-agents-activity";

export function AgentsActivityLocation({ windowId }: { windowId: string }) {
  const page = useAgentsActivity(windowId);

  useTopbarSlot({
    glyph: <GitBranch />,
    // The daemon's count for the whole workspace population, or nothing while
    // it is still in flight — never a page length standing in for a total.
    count: page.total,
    actions: (
      <Button
        data-testid="agents-activity-refresh"
        onClick={page.refetch}
        size="sm"
        type="button"
        variant="ghost"
      >
        <RefreshCw aria-hidden="true" className="size-3" />
        Refresh
      </Button>
    ),
  });

  if (page.surface.status === "error") {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center py-10"
        data-testid="agents-activity-error"
      >
        <Empty
          action={
            <Button onClick={page.refetch} size="sm" type="button" variant="ghost">
              <RefreshCw aria-hidden="true" className="size-3" />
              Retry
            </Button>
          }
          description="The calls request failed. Check that CompozyOS is running and try again."
          icon={AlertCircle}
          title="Couldn't load activity"
        />
      </div>
    );
  }

  if (page.surface.status === "empty") {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center py-10"
        data-testid="agents-activity-empty"
      >
        <Empty
          action={
            <Button onClick={page.openCatalog} size="sm" type="button" variant="ghost">
              Browse agents
            </Button>
          }
          description="When an agent hands work to a specialist, the whole exchange shows up here, live."
          icon={GitBranch}
          title="No agent is delegating work right now"
        />
      </div>
    );
  }

  return (
    <ListingPage data-testid="agents-activity-page">
      {/*
        Out of date is stated, never silent. Rows stay clickable while the stream
        is down — an old jump target beats no target — but their numbers stop
        counting toward badges.
      */}
      {page.stale ? (
        <p
          className="mb-2 flex items-center gap-1.5 rounded-md border border-line-soft bg-canvas-soft px-3 py-1.5 text-form text-muted"
          data-testid="agents-activity-stale"
        >
          <WifiOff className="size-3 shrink-0" aria-hidden="true" />
          Live updates disconnected — showing the last known state. Rows still open.
        </p>
      ) : null}

      <AgentCallTree
        data-testid="agents-activity-tree"
        tree={page.tree}
        countsByRoot={page.countsByRoot}
        childStates={page.childStates}
        onSelectCall={page.openCall}
        onStopSubtree={page.stopSubtree}
        pendingStopRootSessionId={page.pendingStopRootSessionId}
      />

      {page.hasMore ? (
        <div className="flex items-center gap-2 pt-2">
          <span className="text-form text-muted">
            {page.total === undefined
              ? `${page.calls.length} loaded`
              : `${page.calls.length} of ${page.total} loaded`}
          </span>
          <span className="flex-1" />
          <Button
            data-testid="agents-activity-more"
            disabled={page.loadingMore}
            onClick={page.loadMore}
            size="xs"
            type="button"
            variant="outline"
          >
            Load older
          </Button>
        </div>
      ) : null}
    </ListingPage>
  );
}
