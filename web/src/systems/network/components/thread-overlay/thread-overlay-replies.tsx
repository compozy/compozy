import type { ReactNode } from "react";

import { Separator } from "@agh/ui";

import type { NetworkConversationMessage } from "../../types";
import type { HoverToolbarHandlers } from "../timeline/hover-toolbar";
import { MessageTimeline } from "../timeline/timeline";

export interface ThreadOverlayRepliesProps {
  messages: ReadonlyArray<NetworkConversationMessage>;
  isLoading: boolean;
  /** Reply count from thread detail (excludes the root message). */
  replyCount: number;
  lastReadAt?: string | null;
  now?: Date;
  /** Override the default empty placeholder (used to render `ThreadEmpty`). */
  emptyOverride?: ReactNode;
  toolbarHandlers?: (message: NetworkConversationMessage) => HoverToolbarHandlers;
  onRetryOptimistic?: (message: NetworkConversationMessage) => void;
  onDiscardOptimistic?: (message: NetworkConversationMessage) => void;
  onWorkChipClick?: (message: NetworkConversationMessage) => void;
  hasOlder?: boolean;
  isLoadingOlder?: boolean;
  onLoadOlder?: () => void | Promise<void>;
}

export function ThreadOverlayReplies({
  messages,
  isLoading,
  replyCount,
  lastReadAt,
  now,
  emptyOverride,
  toolbarHandlers,
  onRetryOptimistic,
  onDiscardOptimistic,
  onWorkChipClick,
  hasOlder,
  isLoadingOlder,
  onLoadOlder,
}: ThreadOverlayRepliesProps) {
  const replyLabel = replyCount === 1 ? "1 reply" : `${replyCount} replies`;

  return (
    <div className="flex flex-col">
      <Separator
        className="px-4 pt-4 pb-2"
        data-testid="network-thread-overlay-replies-divider"
        label={replyLabel}
        labelClassName="text-subtle"
      />
      <MessageTimeline
        ariaLabel="Thread replies"
        asScrollContainer={false}
        density="overlay"
        emptyState={
          emptyOverride ?? (
            <p className="px-4 py-6 text-center text-small-body text-subtle">No replies yet.</p>
          )
        }
        isLoading={isLoading}
        hasOlder={hasOlder}
        isLoadingOlder={isLoadingOlder}
        lastReadAt={lastReadAt}
        messages={messages}
        now={now}
        onDiscardOptimistic={onDiscardOptimistic}
        onLoadOlder={onLoadOlder}
        onRetryOptimistic={onRetryOptimistic}
        onWorkChipClick={onWorkChipClick}
        toolbarHandlers={toolbarHandlers}
      />
    </div>
  );
}
