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

import {
  ActionResultBanner,
  Alert,
  AlertDescription,
  Button,
  DataSurface,
  ListingPage,
  PAGE_CONTENT_GUTTER,
  useTopbarSlot,
} from "@compozy/ui";

import { AgentCallTree } from "@/systems/agent-comms";

import { AgentsAppTabs } from "./agents-app-tabs";
import { useAgentsActivity } from "./use-agents-activity";
import type { AgentActivitySearch } from "./agent-activity-search";

const ACTIVITY_EMPTY_HINT = `compozy call reviewer "…" --expect @contract.json`;

export function AgentsActivityLocation({
  windowId,
  search = {},
}: {
  windowId: string;
  search?: AgentActivitySearch;
}) {
  const page = useAgentsActivity(windowId, search);

  useTopbarSlot({
    glyph: <GitBranch />,
    // The daemon's count for the whole workspace population, or nothing while
    // it is still in flight — never a page length standing in for a total.
    count: page.total,
    actions: (
      <Button
        aria-label="Refresh"
        data-testid="agents-activity-refresh"
        onClick={page.refetch}
        size="sm"
        type="button"
        variant="ghost"
      >
        <RefreshCw aria-hidden="true" className="size-3" />
      </Button>
    ),
  });

  const emptyTitle =
    page.surface.emptyReason === "no-matches"
      ? "No calls match"
      : "No agent is delegating work right now";
  const emptyDescription =
    page.surface.emptyReason === "no-matches"
      ? "Nothing in this filter is delegating work."
      : "When an agent hands work to a specialist, the whole exchange shows up here.";

  return (
    <ListingPage
      banner={
        <div className={`${PAGE_CONTENT_GUTTER} pt-7`}>
          <AgentsAppTabs value="activity" />
        </div>
      }
      bodyClassName="pt-5"
      data-testid="agents-activity-page"
    >
      <DataSurface className="flex min-h-0 flex-1 flex-col" state={page.surface.status}>
        <DataSurface.Loading data-testid="agents-activity-loading" label="Loading activity" />
        <DataSurface.Error
          action={
            <Button onClick={page.refetch} size="sm" type="button" variant="ghost">
              <RefreshCw aria-hidden="true" className="size-3" />
              Retry
            </Button>
          }
          data-testid="agents-activity-error"
          description="The calls request failed. Check that CompozyOS is running and try again."
          icon={AlertCircle}
          title="Couldn't load activity"
        />
        <DataSurface.Empty
          action={
            <Button onClick={page.openCatalog} size="sm" type="button" variant="ghost">
              Browse agents
            </Button>
          }
          data-testid="agents-activity-empty"
          description={emptyDescription}
          hint={
            page.surface.emptyReason === "no-calls" ? (
              <span className="font-mono text-badge">{ACTIVITY_EMPTY_HINT}</span>
            ) : undefined
          }
          icon={GitBranch}
          title={emptyTitle}
        />
        <DataSurface.Content className="flex min-h-0 flex-1 flex-col gap-2">
          {page.stale ? (
            <Alert data-testid="agents-activity-stale" role="status" variant="warning">
              <WifiOff aria-hidden="true" className="size-4" />
              <AlertDescription>
                Live updates disconnected — showing the last known state. Rows still open.
              </AlertDescription>
            </Alert>
          ) : null}

          {page.drainFailure ? (
            <ActionResultBanner
              actions={
                <Button size="xs" type="button" variant="outline" onClick={page.retryStopSubtree}>
                  Retry
                </Button>
              }
              data-testid="agents-activity-drain-failure"
              description={
                <span>
                  {page.drainFailure.message}
                  {page.drainFailure.code ? (
                    <span className="ml-1 font-mono text-form">{page.drainFailure.code}</span>
                  ) : null}
                </span>
              }
              title="Couldn't stop this subtree."
              tone="danger"
            />
          ) : null}

          {page.drainOutcome ? (
            <ActionResultBanner
              data-testid="agents-activity-drain-outcome"
              description={`${page.drainOutcome.stopped_children} children stopped · ${page.drainOutcome.closed_calls} calls closed · ${page.drainOutcome.preserved_results} results preserved`}
              title="Subtree stopped."
              tone="success"
            />
          ) : null}

          <AgentCallTree
            countsByRoot={page.countsByRoot}
            data-testid="agents-activity-tree"
            onSelectCall={page.openCall}
            onStopSubtree={page.stopSubtree}
            pendingStopRootSessionId={page.pendingStopRootSessionId}
            selectedCallId={search.call ?? null}
            tree={page.tree}
          />

          {page.hasMore ? (
            <div className="flex justify-end pt-2">
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
        </DataSurface.Content>
      </DataSurface>
    </ListingPage>
  );
}
