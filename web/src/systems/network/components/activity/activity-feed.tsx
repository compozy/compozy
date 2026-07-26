import { ActivitySquare, AtSign, MessagesSquare } from "lucide-react";
import { Link } from "@tanstack/react-router";
import { toast } from "sonner";

import { Button, Empty, Skeleton, SkeletonRows } from "@agh/ui";

import { formatNetworkRelativeTime } from "../../lib/network-formatters";
import type { NetworkDirectRoomSummary, NetworkThreadSummary } from "../../types";

export interface ActivityFeedProps {
  workspaceId: string;
  channel: string;
  threads: ReadonlyArray<NetworkThreadSummary>;
  directs: ReadonlyArray<NetworkDirectRoomSummary>;
  status: "loading" | "ready";
  threadTotal?: number;
  directTotal?: number;
  pagination?: {
    threads?: ActivityFeedPagination;
    directs?: ActivityFeedPagination;
  };
  isFiltered?: boolean;
  sort?: "recent_activity" | "created" | "alphabetical";
}

interface ActivityFeedPagination {
  status: "available" | "loading";
  onLoadMore: () => void | Promise<void>;
}

async function loadMore(
  pagination: ActivityFeedPagination,
  fallbackMessage: string
): Promise<void> {
  try {
    await pagination.onLoadMore();
  } catch (error) {
    toast.error(error instanceof Error && error.message ? error.message : fallbackMessage);
  }
}

type ThreadEntry = {
  kind: "thread";
  id: string;
  preview: string;
  title: string;
  timestamp: string | null;
  openedAt: string | null;
  to: "/network/$workspaceId/$channel/threads/$threadId";
  params: { workspaceId: string; channel: string; threadId: string };
};

type DirectEntry = {
  kind: "direct";
  id: string;
  preview: string;
  title: string;
  timestamp: string | null;
  openedAt: string | null;
  to: "/network/$workspaceId/$channel/directs/$directId";
  params: { workspaceId: string; channel: string; directId: string };
};

type ActivityEntry = ThreadEntry | DirectEntry;

function buildEntries(
  workspaceId: string,
  channel: string,
  threads: ReadonlyArray<NetworkThreadSummary>,
  directs: ReadonlyArray<NetworkDirectRoomSummary>,
  sort: "recent_activity" | "created" | "alphabetical"
): ActivityEntry[] {
  const entries: ActivityEntry[] = [];
  for (const thread of threads) {
    entries.push({
      id: `thread:${thread.thread_id}`,
      kind: "thread",
      params: { workspaceId, channel, threadId: thread.thread_id },
      preview: thread.last_message_preview ?? "No messages yet.",
      timestamp: thread.last_activity_at ?? null,
      openedAt: thread.opened_at ?? null,
      title: thread.title ?? "Untitled thread",
      to: "/network/$workspaceId/$channel/threads/$threadId",
    });
  }
  for (const direct of directs) {
    entries.push({
      id: `direct:${direct.direct_id}`,
      kind: "direct",
      params: { workspaceId, channel, directId: direct.direct_id },
      preview: direct.last_message_preview ?? "No messages yet.",
      timestamp: direct.last_activity_at ?? null,
      openedAt: direct.opened_at ?? null,
      title: `${direct.session_a} ↔ ${direct.session_b}`,
      to: "/network/$workspaceId/$channel/directs/$directId",
    });
  }

  return entries.sort((left, right) => {
    if (sort === "alphabetical") {
      const byTitle = left.title.localeCompare(right.title, undefined, { sensitivity: "base" });
      return byTitle === 0 ? left.id.localeCompare(right.id) : byTitle;
    }
    if (sort === "created") {
      const leftCreated = left.openedAt ? new Date(left.openedAt).getTime() : 0;
      const rightCreated = right.openedAt ? new Date(right.openedAt).getTime() : 0;
      const byCreated = leftCreated - rightCreated;
      return byCreated === 0 ? left.id.localeCompare(right.id) : byCreated;
    }
    const leftTs = left.timestamp ? new Date(left.timestamp).getTime() : 0;
    const rightTs = right.timestamp ? new Date(right.timestamp).getTime() : 0;
    const byActivity = rightTs - leftTs;
    return byActivity === 0 ? left.id.localeCompare(right.id) : byActivity;
  });
}

function ActivityFeedSkeleton() {
  return (
    <SkeletonRows
      count={4}
      data-testid="network-activity-feed-skeleton"
      rowClassName="border-b border-line px-5 py-3"
    >
      <Skeleton className="h-3 w-24" />
      <Skeleton className="h-4 w-2/3" />
      <Skeleton className="h-3 w-full" />
    </SkeletonRows>
  );
}

const ACTIVITY_ROW_CLASS = [
  "grid grid-cols-[26px_minmax(0,1fr)_auto] items-center gap-2.5",
  "border-b border-line-soft px-5 py-2.5 text-left",
  "transition-colors duration-base hover:bg-row-hover",
  "focus-visible:outline-none focus-visible:shadow-focus-inset",
].join(" ");

function ActivityRowBody({ entry }: { entry: ActivityEntry }) {
  const Icon = entry.kind === "thread" ? MessagesSquare : AtSign;
  return (
    <>
      <span
        aria-hidden="true"
        className="flex size-6.5 items-center justify-center rounded-sm bg-elevated text-subtle"
      >
        <Icon className="size-3.5" />
      </span>
      <span className="min-w-0 truncate text-small-body text-muted">
        <span className="font-medium text-fg">{entry.title}</span>
        <span aria-hidden="true"> — </span>
        {entry.preview}
      </span>
      <span className="font-mono text-mono-id tabular-nums text-faint">
        {formatNetworkRelativeTime(entry.timestamp)}
      </span>
    </>
  );
}

/**
 * Channel activity as an event-styled feed (`arow` anatomy): per-kind icon
 * well, actor-first text, trailing freshness. Rows come from thread and
 * direct-room container updates — the runtime exposes no finer event stream.
 */
export function ActivityFeed({
  workspaceId,
  channel,
  threads,
  directs,
  status,
  pagination,
  isFiltered = false,
  sort = "recent_activity",
}: ActivityFeedProps) {
  const entries = buildEntries(workspaceId, channel, threads, directs, sort);
  const threadPagination = pagination?.threads;
  const directPagination = pagination?.directs;

  if (status === "loading" && entries.length === 0) {
    return <ActivityFeedSkeleton />;
  }

  if (entries.length === 0 && !threadPagination && !directPagination) {
    return (
      <div className="flex flex-1 items-center justify-center px-6 py-10">
        <Empty
          className="max-w-md"
          description={
            isFiltered
              ? "Try another search or remove a filter."
              : "No activity yet across threads or direct rooms."
          }
          icon={ActivitySquare}
          title={isFiltered ? "No matching activity." : "Quiet across the channel."}
        />
      </div>
    );
  }

  return (
    <div
      aria-label={`Activity in #${channel}`}
      className="flex flex-1 flex-col overflow-y-auto"
      data-testid="network-activity-feed"
    >
      {entries.map(entry =>
        entry.kind === "thread" ? (
          <Link
            className={ACTIVITY_ROW_CLASS}
            data-testid={`network-activity-entry-${entry.id}`}
            key={entry.id}
            params={entry.params}
            to={entry.to}
          >
            <ActivityRowBody entry={entry} />
          </Link>
        ) : (
          <Link
            className={ACTIVITY_ROW_CLASS}
            data-testid={`network-activity-entry-${entry.id}`}
            key={entry.id}
            params={entry.params}
            to={entry.to}
          >
            <ActivityRowBody entry={entry} />
          </Link>
        )
      )}
      {threadPagination || directPagination ? (
        <div className="flex flex-wrap items-center justify-end gap-2 border-t border-line px-5 py-3">
          {threadPagination ? (
            <Button
              aria-busy={threadPagination.status === "loading"}
              disabled={threadPagination.status === "loading"}
              onClick={() => void loadMore(threadPagination, "Failed to load more threads.")}
              size="sm"
              variant="neutral"
            >
              {threadPagination.status === "loading" ? "Loading threads…" : "Load more threads"}
            </Button>
          ) : null}
          {directPagination ? (
            <Button
              aria-busy={directPagination.status === "loading"}
              disabled={directPagination.status === "loading"}
              onClick={() => void loadMore(directPagination, "Failed to load more direct rooms.")}
              size="sm"
              variant="neutral"
            >
              {directPagination.status === "loading" ? "Loading rooms…" : "Load more direct rooms"}
            </Button>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
