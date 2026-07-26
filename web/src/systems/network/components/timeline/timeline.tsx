import { Fragment, useLayoutEffect, useRef } from "react";

import { Button, Skeleton, SkeletonRows } from "@agh/ui";

import { cn } from "@/lib/utils";

import { buildTimelineEntries } from "../../lib/group-messages";
import type { NetworkConversationMessage } from "../../types";
import { DatePill } from "./date-pill";
import type { HoverToolbarHandlers } from "./hover-toolbar";
import { MessageRow, type MessageRowDensity } from "./message-row";
import { MessageRowCollapsed } from "./message-row-collapsed";
import { MessageRowSystem } from "./message-row-system";
import { NewDivider } from "./new-divider";

export interface MessageTimelineProps {
  messages: ReadonlyArray<NetworkConversationMessage>;
  isLoading?: boolean;
  emptyState?: React.ReactNode;
  errorState?: React.ReactNode;
  density?: MessageRowDensity;
  /** Reference moment for date pill labels. */
  now?: Date;
  /** Last-read timestamp used to position the "New" divider. */
  lastReadAt?: string | null;
  className?: string;
  /** Stable id used by aria-label and data attributes. */
  ariaLabel?: string;
  /**
   * Whether the list owns its own vertical scroll. Defaults to `true`. Set to
   * `false` when an ancestor (e.g. a `ScrollArea`) owns scrolling, so the list
   * renders at natural height instead of double-scrolling.
   */
  asScrollContainer?: boolean;
  toolbarHandlers?: (message: NetworkConversationMessage) => HoverToolbarHandlers | undefined;
  /** Retry handler for failed optimistic sends per `_design.md` §7.3. */
  onRetryOptimistic?: (message: NetworkConversationMessage) => void;
  /** Discard handler for failed optimistic sends. */
  onDiscardOptimistic?: (message: NetworkConversationMessage) => void;
  /** Click handler for the inline work chip per `_design.md` §5.8.1. */
  onWorkChipClick?: (message: NetworkConversationMessage) => void;
  hasOlder?: boolean;
  isLoadingOlder?: boolean;
  onLoadOlder?: () => void | Promise<void>;
}

interface TimelineSkeletonProps {
  density: MessageRowDensity;
}

function TimelineSkeleton({ density }: TimelineSkeletonProps) {
  return (
    <div
      aria-label="Loading messages"
      className="space-y-4 px-5 py-4"
      data-testid="network-timeline-skeleton"
      role="status"
    >
      <SkeletonRows count={5} className="gap-4" rowClassName="flex-row gap-3">
        <Skeleton className={cn("rounded-chip", density === "overlay" ? "size-8" : "size-9")} />
        <div className="flex flex-1 flex-col gap-1.5">
          <Skeleton className="h-3 w-40" />
          <Skeleton className="h-3.5 w-full max-w-md" />
          <Skeleton className="h-3.5 w-3/4 max-w-88" />
        </div>
      </SkeletonRows>
    </div>
  );
}

export function MessageTimeline({
  messages,
  isLoading = false,
  emptyState,
  errorState,
  density = "channel",
  now,
  lastReadAt,
  className,
  ariaLabel = "Timeline",
  asScrollContainer = true,
  toolbarHandlers,
  onRetryOptimistic,
  onDiscardOptimistic,
  onWorkChipClick,
  hasOlder = false,
  isLoadingOlder = false,
  onLoadOlder,
}: MessageTimelineProps) {
  const rootRef = useRef<HTMLDivElement>(null);
  const anchorRef = useRef<{
    element: HTMLElement;
    messageId: string | null;
    offset: number;
    height: number;
    top: number;
  } | null>(null);
  const entries = buildTimelineEntries({ messages, now, lastReadAt });
  const firstMessageId = messages[0]?.message_id;
  useLayoutEffect(() => {
    const anchor = anchorRef.current;
    if (!anchor) return;
    const candidate = [
      ...(rootRef.current?.querySelectorAll<HTMLElement>("[data-message-id]") ?? []),
    ].find(element => element.dataset.messageId === anchor.messageId);
    const viewportTop = anchor.element.getBoundingClientRect().top;
    const nextOffset = candidate ? candidate.getBoundingClientRect().top - viewportTop : 0;
    const offsetDelta = nextOffset - anchor.offset;
    anchor.element.scrollTop =
      anchor.top + (offsetDelta !== 0 ? offsetDelta : anchor.element.scrollHeight - anchor.height);
    anchorRef.current = null;
  }, [firstMessageId, messages.length]);
  const handleLoadOlder = () => {
    const root = rootRef.current;
    if (!root || !onLoadOlder || isLoadingOlder) return;
    const viewport = asScrollContainer
      ? root
      : ((root.closest('[data-slot="scroll-area-viewport"]') as HTMLElement | null) ?? root);
    const viewportTop = viewport.getBoundingClientRect().top;
    const visibleAnchor = [...root.querySelectorAll<HTMLElement>("[data-message-id]")].find(
      element => element.getBoundingClientRect().bottom >= viewportTop
    );
    anchorRef.current = {
      element: viewport,
      messageId: visibleAnchor?.dataset.messageId ?? null,
      offset: visibleAnchor ? visibleAnchor.getBoundingClientRect().top - viewportTop : 0,
      height: viewport.scrollHeight,
      top: viewport.scrollTop,
    };
    void onLoadOlder();
  };

  if (errorState) {
    return (
      <div className="flex flex-1 items-center justify-center px-5 py-10" role="alert">
        {errorState}
      </div>
    );
  }

  if (isLoading && messages.length === 0) {
    return <TimelineSkeleton density={density} />;
  }

  if (entries.length === 0) {
    return (
      <div
        className="flex flex-1 items-center justify-center px-5 py-10"
        data-testid="network-timeline-empty"
      >
        {emptyState}
      </div>
    );
  }

  return (
    <div
      className={cn(
        "flex flex-col gap-1 py-2",
        asScrollContainer && "flex-1 overflow-y-auto",
        className
      )}
      data-density={density}
      data-testid="network-timeline"
      ref={rootRef}
    >
      {hasOlder && onLoadOlder ? (
        <div className="flex justify-center px-4 py-2">
          <Button
            aria-busy={isLoadingOlder}
            aria-label="Load older messages"
            disabled={isLoadingOlder}
            onClick={handleLoadOlder}
            size="sm"
            variant="outline"
          >
            {isLoadingOlder ? "Loading older messages…" : "Load older messages"}
          </Button>
        </div>
      ) : null}
      <div aria-label={ariaLabel} className="flex flex-col gap-1" role="log">
        {entries.map(entry => {
          if (entry.kind === "date-pill") {
            return <DatePill key={entry.id} now={now} timestamp={entry.timestamp} />;
          }
          if (entry.kind === "new-divider") {
            return <NewDivider key={entry.id} />;
          }
          const handlers = toolbarHandlers?.(entry.message);
          if (entry.variant === "system") {
            return (
              <Fragment key={entry.id}>
                <MessageRowSystem message={entry.message} />
              </Fragment>
            );
          }
          if (entry.variant === "collapsed") {
            return (
              <MessageRowCollapsed
                density={density}
                key={entry.id}
                message={entry.message}
                onCopyLink={handlers?.onCopyLink}
                onCopyText={handlers?.onCopyText}
              />
            );
          }
          return (
            <MessageRow
              density={density}
              key={entry.id}
              message={entry.message}
              onCopyLink={handlers?.onCopyLink}
              onCopyText={handlers?.onCopyText}
              onDiscard={onDiscardOptimistic}
              onRetry={onRetryOptimistic}
              onWorkChipClick={onWorkChipClick}
            />
          );
        })}
      </div>
    </div>
  );
}
