import { type ComponentPropsWithoutRef, useRef } from "react";

import { cn } from "@/lib/utils";

import { useThreadScrollController } from "./hooks/use-thread-scroll-controller";
import { ScrollToBottomPill } from "./scroll-to-bottom-pill";
import { ThreadContentRail, type SessionThreadContentInset } from "./session-thread-content-rail";
import { ThreadMessages } from "./session-thread-messages";
import {
  type SessionFailurePayload,
  type SessionState,
  useSessionTranscriptThreadState,
} from "@/systems/session";

type ThreadViewportProps = ComponentPropsWithoutRef<"div">;

/**
 * The transcript scroller: renders the thread rows inside the shared content
 * rail, keeps the reader anchored while older history loads above, and floats
 * the scroll-to-bottom pill over its own edge.
 */
export function ThreadViewport({
  agentName,
  sessionId,
  isSessionRunning,
  contentInset,
  sessionState,
  failure,
  startupFailed,
  className,
  ...props
}: ThreadViewportProps & {
  agentName: string;
  sessionId: string;
  isSessionRunning: boolean;
  contentInset: SessionThreadContentInset;
  sessionState?: SessionState;
  failure?: SessionFailurePayload | null;
  startupFailed: boolean;
}) {
  const viewportRef = useRef<HTMLDivElement | null>(null);
  const contentRef = useRef<HTMLDivElement | null>(null);
  const {
    messages: transcriptMessages,
    status: transcriptStatus,
    error: transcriptError,
    hasOlder,
    isFetchingOlder,
    loadOlder,
    retry: retryTranscript,
  } = useSessionTranscriptThreadState();
  const messageCount = transcriptMessages.length;
  const showLoadOlder = hasOlder || isFetchingOlder;
  const {
    captureHistoryAnchor,
    leadingItemCount,
    measureVirtualElement,
    showScrollToBottom,
    scrollToEnd,
    virtualItems,
    virtualTotalSize,
  } = useThreadScrollController(
    viewportRef,
    contentRef,
    transcriptMessages,
    showLoadOlder && messageCount > 0
  );
  const loadOlderWithAnchor = () => {
    captureHistoryAnchor();
    loadOlder();
  };

  return (
    <div className="relative flex min-h-0 min-w-0 flex-1 flex-col">
      <div
        {...props}
        ref={viewportRef}
        className={cn("min-h-0 flex-1 overflow-y-auto", className)}
        data-testid="chat-view"
      >
        <ThreadContentRail ref={contentRef} inset={contentInset} className="min-h-full">
          <ThreadMessages
            agentName={agentName}
            sessionId={sessionId}
            isSessionRunning={isSessionRunning}
            messageCount={messageCount}
            transcriptStatus={transcriptStatus}
            transcriptError={transcriptError}
            retryTranscript={retryTranscript}
            transcriptMessages={transcriptMessages}
            measureVirtualElement={measureVirtualElement}
            virtualItems={virtualItems}
            virtualTotalSize={virtualTotalSize}
            leadingItemCount={leadingItemCount}
            showLoadOlder={showLoadOlder}
            isFetchingOlder={isFetchingOlder}
            loadOlder={loadOlderWithAnchor}
            sessionState={sessionState}
            failure={failure}
            startupFailed={startupFailed}
          />
        </ThreadContentRail>
      </div>
      <ScrollToBottomPill visible={showScrollToBottom} onClick={scrollToEnd} />
    </div>
  );
}
