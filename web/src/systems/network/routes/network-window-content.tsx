import { Plus } from "lucide-react";
import { useState } from "react";

import { Button } from "@agh/ui";

import { ActivityFeed } from "../components/activity";
import { DirectRoom, DirectsList, NewDirectDialog } from "../components/directs";
import { DirectsEmpty } from "../components/empty-states/directs-empty";
import { ChannelThreadComposer } from "../components/composer/channel-thread-composer";
import { ThreadOverlay } from "../components/thread-overlay/thread-overlay";
import { ThreadsList } from "../components/threads";
import type { ChannelTab } from "../components/shell/channel-tabs-types";
import { useNetworkChannelDirectsRoute } from "../hooks/use-network-channel-directs-route";
import { useNetworkChannelThreadsRoute } from "../hooks/use-network-channel-threads-route";
import { useNetworkListFiltersContext } from "../hooks/use-network-list-filters-context";

interface NetworkWindowContentProps {
  fanoutPolicy?: string | null;
  workspaceId: string;
  channel: string;
  activeTab: ChannelTab;
  activeThreadId: string | null;
  activeDirectId: string | null;
  threadView: "full" | undefined;
  onCloseThread: () => void;
  onOpenThreadMain: () => void;
}

export function NetworkWindowContent(props: NetworkWindowContentProps) {
  if (props.activeTab === "directs") return <NetworkDirectsLocation {...props} />;
  if (props.activeTab === "activity") return <NetworkActivityLocation {...props} />;
  return <NetworkThreadsLocation {...props} />;
}

function NetworkThreadsLocation({
  workspaceId,
  channel,
  fanoutPolicy,
  activeThreadId,
  threadView,
  onCloseThread,
  onOpenThreadMain,
}: NetworkWindowContentProps) {
  const route = useNetworkChannelThreadsRoute({
    workspaceId,
    channel,
    activeThreadId,
    view: threadView,
  });
  const { isFullPage, showOverlay, showList, activeSession } = route;
  const { filteredThreads, threadsQuery, isFiltered } = useNetworkListFiltersContext();

  return (
    <section
      aria-label={`Threads in #${channel}`}
      className="flex min-h-0 flex-1 flex-col"
      data-testid="network-threads-tab"
    >
      <div className="flex min-h-0 min-w-0 flex-1">
        {showList ? (
          <ThreadsList
            workspaceId={workspaceId}
            activeThreadId={activeThreadId}
            channel={channel}
            dim={showOverlay && !isFullPage}
            status={threadsQuery.isLoading ? "loading" : "ready"}
            paginationStatus={
              threadsQuery.isLoadingMore
                ? "loading"
                : threadsQuery.hasMore
                  ? "available"
                  : undefined
            }
            isFiltered={isFiltered}
            onLoadMore={threadsQuery.loadMore}
            threads={filteredThreads}
            total={threadsQuery.total}
          />
        ) : null}

        {showOverlay && isFullPage && activeThreadId ? (
          <div
            className="flex min-h-0 flex-1 flex-col bg-canvas"
            data-testid="network-thread-overlay-fullpage"
          >
            <ThreadOverlay
              workspaceId={workspaceId}
              channel={channel}
              fullPage
              onClose={onCloseThread}
              onOpenMain={onOpenThreadMain}
              threadId={activeThreadId}
            />
          </div>
        ) : null}
      </div>

      {showList && !showOverlay ? (
        <ChannelThreadComposer
          fanoutPolicy={fanoutPolicy ?? null}
          workspaceId={workspaceId}
          channel={channel}
          disabledReason={activeSession.disabledReason ?? undefined}
          displayName={activeSession.session?.displayName}
          peerFrom={activeSession.session?.peerId ?? ""}
          sessionId={activeSession.session?.sessionId ?? ""}
        />
      ) : null}
    </section>
  );
}

function NetworkDirectsLocation({
  workspaceId,
  channel,
  activeDirectId,
}: NetworkWindowContentProps) {
  const route = useNetworkChannelDirectsRoute(workspaceId, channel);
  const { filteredDirects, directsQuery, isFiltered } = useNetworkListFiltersContext();
  const [newDirectOpen, setNewDirectOpen] = useState(false);

  if (activeDirectId) {
    return (
      <section
        aria-label={`Direct room ${activeDirectId} in #${channel}`}
        className="flex min-h-0 flex-1 flex-col"
        data-testid="network-direct-detail-slot"
      >
        <DirectRoom channel={channel} directId={activeDirectId} workspaceId={workspaceId} />
      </section>
    );
  }

  const activeSession = route.session;
  const channelMembers = route.members;
  const showEmpty = !directsQuery.isLoading && filteredDirects.length === 0;
  const sessionId = activeSession.session?.sessionId ?? "";
  const subheaderLabel = isFiltered
    ? `${directsQuery.total} matching direct ${directsQuery.total === 1 ? "room" : "rooms"}`
    : `${directsQuery.total} direct ${directsQuery.total === 1 ? "room" : "rooms"} in this channel`;

  return (
    <section
      aria-label={`Direct rooms in #${channel}`}
      className="flex min-h-0 flex-1 flex-col"
      data-testid="network-directs-tab"
    >
      <header
        className="flex items-center justify-between gap-3 border-b border-line px-5 py-2"
        data-testid="network-directs-subheader"
      >
        <span className="text-form-label text-subtle">{subheaderLabel}</span>
        <Button
          aria-label="Open new direct room"
          data-testid="network-directs-new-direct"
          disabled={!sessionId}
          onClick={() => setNewDirectOpen(true)}
          size="sm"
          type="button"
          variant="neutral"
        >
          <Plus aria-hidden="true" className="size-3" />
          New direct
        </Button>
      </header>

      {showEmpty ? (
        <div className="flex flex-1 items-center justify-center px-6 py-10">
          <DirectsEmpty
            className="max-w-md"
            filtered={isFiltered}
            onNewDirect={sessionId ? () => setNewDirectOpen(true) : undefined}
          />
        </div>
      ) : (
        <DirectsList
          workspaceId={workspaceId}
          activeDirectId={null}
          channel={channel}
          directs={filteredDirects}
          hasMore={directsQuery.hasMore}
          isLoading={directsQuery.isLoading}
          isLoadingMore={directsQuery.isLoadingMore}
          members={channelMembers.members}
          onLoadMore={directsQuery.loadMore}
          selfSessionId={activeSession.session?.sessionId}
          total={directsQuery.total}
        />
      )}

      <NewDirectDialog
        workspaceId={workspaceId}
        channel={channel}
        onOpenChange={setNewDirectOpen}
        open={newDirectOpen && Boolean(sessionId)}
        selfPeerId={activeSession.session?.peerId}
        sessionId={sessionId}
      />
    </section>
  );
}

function NetworkActivityLocation({ workspaceId, channel }: NetworkWindowContentProps) {
  const { filteredThreads, filteredDirects, threadsQuery, directsQuery, isFiltered, sort } =
    useNetworkListFiltersContext();

  return (
    <section
      aria-label={`Activity in #${channel}`}
      className="flex min-h-0 flex-1 flex-col"
      data-testid="network-activity-tab"
    >
      <ActivityFeed
        workspaceId={workspaceId}
        channel={channel}
        directs={filteredDirects}
        directTotal={directsQuery.total}
        isFiltered={isFiltered}
        status={threadsQuery.isLoading || directsQuery.isLoading ? "loading" : "ready"}
        pagination={{
          threads:
            threadsQuery.hasMore || threadsQuery.isLoadingMore
              ? {
                  status: threadsQuery.isLoadingMore ? "loading" : "available",
                  onLoadMore: threadsQuery.loadMore,
                }
              : undefined,
          directs:
            directsQuery.hasMore || directsQuery.isLoadingMore
              ? {
                  status: directsQuery.isLoadingMore ? "loading" : "available",
                  onLoadMore: directsQuery.loadMore,
                }
              : undefined,
        }}
        sort={sort}
        threadTotal={threadsQuery.total}
        threads={filteredThreads}
      />
    </section>
  );
}
