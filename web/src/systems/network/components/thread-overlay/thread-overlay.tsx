import { ScrollArea } from "@agh/ui";

import { DetailComposer } from "../composer/detail-composer";
import { ConversationError } from "../empty-states/conversation-error";
import { ThreadEmpty } from "../empty-states/thread-empty";
import { useMessageCopyActions } from "../timeline/use-message-copy-actions";
import { WorkBanner } from "../work/work-banner";
import { ThreadOverlayHeader, ThreadTaskLinks } from "./thread-overlay-header";
import { ThreadOverlayReplies } from "./thread-overlay-replies";
import { ThreadOverlayRoot } from "./thread-overlay-root";
import { useThreadOverlayView } from "./use-thread-overlay-view";

export interface ThreadOverlayProps {
  workspaceId: string;
  channel: string;
  threadId: string;
  /** Render in full-page mode (no fixed width / no border-left) for `<1024px` or `?view=full`. */
  fullPage?: boolean;
  onClose: () => void;
  onOpenMain: () => void;
}

export function ThreadOverlay({
  workspaceId,
  channel,
  threadId,
  fullPage = false,
  onClose,
  onOpenMain,
}: ThreadOverlayProps) {
  const view = useThreadOverlayView({ workspaceId, channel, fullPage, threadId, onClose });
  const { overlay, session, disabledReason, openWork, handleRetry, handleDiscard } = view;
  const detailError = overlay.detailError;
  const isResolvingDetail = !detailError && !overlay.detail;
  const toolbarHandlers = useMessageCopyActions({
    surface: "thread",
    workspaceId,
    channel,
    conversationId: threadId,
  });

  return (
    <section
      aria-label={fullPage ? "Thread" : "Thread overlay"}
      className="flex min-h-0 flex-1 flex-col bg-canvas"
      data-fullpage={fullPage ? "true" : "false"}
      data-testid="network-thread-overlay"
    >
      <ThreadOverlayHeader
        workspaceId={workspaceId}
        channel={channel}
        detail={overlay.detail}
        rootMessageId={overlay.rootMessage?.message_id ?? null}
        selfSessionId={session?.sessionId ?? null}
        threadId={threadId}
        onClose={onClose}
        onOpenMain={onOpenMain}
      />
      <ThreadTaskLinks links={overlay.detail?.task_links ?? []} />
      {detailError ? (
        <div className="flex flex-1 items-center justify-center px-5 py-10" role="alert">
          <ConversationError
            description={`Could not load thread ${threadId}. Choose an existing thread from #${channel}.`}
            testId="network-thread-overlay-error"
            title="Thread unavailable"
          />
        </div>
      ) : isResolvingDetail ? (
        <ScrollArea className="min-h-0 flex-1">
          <ThreadOverlayRoot isLoading rootMessage={null} />
          <ThreadOverlayReplies isLoading messages={[]} replyCount={0} />
        </ScrollArea>
      ) : (
        <>
          <WorkBanner hasNeedsInput={false} openCount={openWork.openCount} />
          <ScrollArea className="min-h-0 flex-1">
            <ThreadOverlayRoot
              isLoading={overlay.isDetailLoading}
              rootMessage={overlay.rootMessage}
              toolbarHandlers={toolbarHandlers}
            />
            <ThreadOverlayReplies
              emptyOverride={<ThreadEmpty />}
              isLoading={overlay.isMessagesLoading}
              hasOlder={overlay.hasOlder}
              isLoadingOlder={overlay.isLoadingOlder}
              lastReadAt={overlay.lastReadIso}
              messages={overlay.replies}
              onDiscardOptimistic={handleDiscard}
              onLoadOlder={overlay.loadOlder}
              onRetryOptimistic={handleRetry}
              replyCount={overlay.replyCount}
              toolbarHandlers={toolbarHandlers}
            />
          </ScrollArea>

          <DetailComposer
            workspaceId={workspaceId}
            channel={channel}
            disabledReason={disabledReason ?? undefined}
            displayName={session?.displayName}
            peerFrom={session?.peerId ?? ""}
            sessionId={session?.sessionId ?? ""}
            surface="thread"
            threadId={threadId}
          />
        </>
      )}
    </section>
  );
}
