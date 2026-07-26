import { Link } from "@tanstack/react-router";

import { Button, ListingRow, Pill, Skeleton, SkeletonRows } from "@agh/ui";

import { cn } from "@/lib/utils";

import { formatNetworkRelativeTime } from "../../lib/network-formatters";
import type { NetworkThreadSummary } from "../../types";
import { ThreadsEmpty } from "../empty-states/threads-empty";

export interface ThreadsListProps {
  workspaceId: string;
  channel: string;
  threads: ReadonlyArray<NetworkThreadSummary>;
  activeThreadId: string | null;
  status: "loading" | "ready";
  total?: number;
  paginationStatus?: "available" | "loading";
  onLoadMore?: () => void | Promise<void>;
  isFiltered?: boolean;
  /** Reduced contrast applied when the right-rail thread overlay is open. */
  dim?: boolean;
  onStartThread?: () => void;
}

interface ThreadsListRowProps {
  workspaceId: string;
  channel: string;
  thread: NetworkThreadSummary;
  active: boolean;
}

function ThreadWorkPill({ openWorkCount }: { openWorkCount: number }) {
  if (openWorkCount === 0) {
    return null;
  }
  // Without per-state breakdown on the summary we can only surface that the
  // thread has open work; clearly truthful, not invented.
  return (
    <Pill data-testid="network-thread-list-row-state-chip" mono size="xs" tone="warning">
      {openWorkCount === 1 ? "1 work open" : `${openWorkCount} work open`}
    </Pill>
  );
}

function ThreadsListRow({ workspaceId, channel, thread, active }: ThreadsListRowProps) {
  const messageCount = thread.message_count ?? 0;
  const replyCount = Math.max(0, messageCount - 1); // root + replies - guard against historical zero.
  const peerCount = thread.participant_count ?? 0;
  const openWorkCount = thread.open_work_count ?? 0;
  const lastActivity = formatNetworkRelativeTime(thread.last_activity_at ?? null);
  const opener = thread.opened_by_peer_id?.trim() || "unknown";
  const title = thread.title ?? "Untitled thread";
  const preview = thread.last_message_preview ?? "No messages yet.";

  return (
    <ListingRow
      aria-current={active ? "page" : undefined}
      className="grid-cols-[minmax(0,1fr)]"
      data-testid={`network-thread-list-row-${thread.thread_id}`}
      selected={active}
    >
      <ListingRow.Link
        className="col-span-1 grid-cols-[minmax(0,1fr)]"
        render={
          <Link
            params={{ workspaceId, channel, threadId: thread.thread_id }}
            to="/network/$workspaceId/$channel/threads/$threadId"
            aria-label={`Open ${title}`}
          />
        }
      >
        <ListingRow.Main>
          <ListingRow.Name>
            <ListingRow.Title
              data-testid={`network-thread-list-row-title-${thread.thread_id}`}
              title={thread.title ?? undefined}
            >
              {title}
            </ListingRow.Title>
            <ThreadWorkPill openWorkCount={openWorkCount} />
            <span
              className="ml-auto shrink-0 text-eyebrow text-faint"
              data-testid="network-thread-list-row-meta-time"
            >
              {lastActivity}
            </span>
          </ListingRow.Name>
          <ListingRow.Description
            data-testid={`network-thread-list-row-preview-${thread.thread_id}`}
            title={thread.last_message_preview ?? undefined}
          >
            {preview}
          </ListingRow.Description>
          <ListingRow.Meta>
            <span data-testid="network-thread-list-row-meta-peers">
              {peerCount} {peerCount === 1 ? "peer" : "peers"}
            </span>
            <ListingRow.MetaDot />
            <span data-testid="network-thread-list-row-meta-replies">
              {replyCount} {replyCount === 1 ? "reply" : "replies"}
            </span>
            <ListingRow.MetaDot />
            <span data-testid="network-thread-list-row-meta-opener">started by {opener}</span>
          </ListingRow.Meta>
        </ListingRow.Main>
      </ListingRow.Link>
    </ListingRow>
  );
}

function ThreadsListSkeleton() {
  return (
    <SkeletonRows
      count={5}
      data-testid="network-thread-list-skeleton"
      rowClassName="border-b border-line px-4 py-3"
    >
      <div className="flex min-w-0 flex-1 flex-col gap-2">
        <Skeleton className="h-3.5 w-2/3" />
        <Skeleton className="h-3 w-full" />
        <Skeleton className="h-3 w-3/4" />
      </div>
    </SkeletonRows>
  );
}

export function ThreadsList({
  workspaceId,
  channel,
  threads,
  activeThreadId,
  status,
  total,
  paginationStatus,
  onLoadMore,
  isFiltered = false,
  dim = false,
  onStartThread,
}: ThreadsListProps) {
  if (status === "loading" && threads.length === 0) {
    return <ThreadsListSkeleton />;
  }

  if (threads.length === 0) {
    return (
      <div className="flex flex-1 items-center justify-center px-6 py-10">
        <ThreadsEmpty className="max-w-md" filtered={isFiltered} onStartThread={onStartThread} />
      </div>
    );
  }

  return (
    <div
      aria-label={`Threads in #${channel}`}
      aria-live="polite"
      className={cn(
        "flex min-w-0 flex-1 flex-col overflow-x-hidden overflow-y-auto transition-opacity",
        dim ? "opacity-55" : "opacity-100"
      )}
      data-dim={dim ? "true" : "false"}
      data-testid="network-thread-list"
    >
      <div className="mx-5 mt-1 mb-4 overflow-hidden rounded-lg border border-line bg-canvas-soft">
        {threads.map(thread => (
          <ThreadsListRow
            active={thread.thread_id === activeThreadId}
            channel={channel}
            key={thread.thread_id}
            thread={thread}
            workspaceId={workspaceId}
          />
        ))}
      </div>
      {paginationStatus && onLoadMore ? (
        <div className="flex items-center justify-between gap-3 border-t border-line px-4 py-3">
          <span className="text-small-body text-muted">
            {threads.length} of {total ?? threads.length} threads loaded
          </span>
          <Button
            aria-busy={paginationStatus === "loading"}
            aria-label="Load more threads"
            disabled={paginationStatus === "loading"}
            onClick={() => void onLoadMore()}
            size="sm"
            variant="outline"
          >
            {paginationStatus === "loading" ? "Loading threads…" : "Load more threads"}
          </Button>
        </div>
      ) : null}
    </div>
  );
}
