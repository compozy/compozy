import { ThreadPrimitive } from "@assistant-ui/react";
import { type ComponentPropsWithoutRef, useLayoutEffect, useRef } from "react";

import { cn } from "@/lib/utils";
import { SessionLoadOlderButton } from "@/systems/session/components/session-load-older-button";
import { useSessionTranscriptThreadState } from "@/systems/session";
import type { SessionFailurePayload, SessionState } from "@/systems/session";
import { useThreadScrollController } from "./hooks/use-thread-scroll-controller";
import { ScrollToBottomPill } from "./scroll-to-bottom-pill";
import { ThreadContentRail, type SessionThreadContentInset } from "./session-thread-content-rail";
import { ThreadMessages } from "./session-thread-messages";

type ThreadViewportProps = ComponentPropsWithoutRef<typeof ThreadPrimitive.Viewport>;

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
  const {
    messages: transcriptMessages,
    status: transcriptStatus,
    error: transcriptError,
    hasOlder,
    isFetchingOlder,
    loadOlder,
    retry: retryTranscript,
  } = useSessionTranscriptThreadState();
  const olderAnchorRef = useRef<{ messageId: string; offsetTop: number } | null>(null);
  const messageCount = transcriptMessages.length;
  const { showScrollToBottom, scrollToEnd } = useThreadScrollController(
    viewportRef,
    transcriptMessages
  );
  const loadOlderWithAnchor = () => {
    const viewport = viewportRef.current;
    if (viewport) {
      const viewportTop = viewport.getBoundingClientRect().top;
      const messageRows = [...viewport.querySelectorAll<HTMLElement>("[data-message-id]")];
      const anchorRow =
        messageRows.find(row => row.getBoundingClientRect().bottom >= viewportTop) ??
        messageRows[0];
      const messageId = anchorRow?.dataset.messageId;
      olderAnchorRef.current =
        anchorRow && messageId
          ? { messageId, offsetTop: anchorRow.getBoundingClientRect().top - viewportTop }
          : null;
    }
    loadOlder();
  };

  useLayoutEffect(() => {
    const viewport = viewportRef.current;
    const anchor = olderAnchorRef.current;
    if (!viewport || !anchor || isFetchingOlder) return;
    const anchorRow = [...viewport.querySelectorAll<HTMLElement>("[data-message-id]")].find(
      row => row.dataset.messageId === anchor.messageId
    );
    if (anchorRow) {
      const nextOffset =
        anchorRow.getBoundingClientRect().top - viewport.getBoundingClientRect().top;
      viewport.scrollTop += nextOffset - anchor.offsetTop;
    }
    olderAnchorRef.current = null;
  }, [isFetchingOlder, messageCount]);

  return (
    <div className="relative flex min-h-0 min-w-0 flex-1 flex-col">
      <ThreadPrimitive.Viewport
        {...props}
        ref={viewportRef}
        className={cn("min-h-0 flex-1 overflow-y-auto", className)}
        data-testid="chat-view"
      >
        <ThreadContentRail inset={contentInset} className="min-h-full">
          {hasOlder || isFetchingOlder ? (
            <SessionLoadOlderButton isLoading={isFetchingOlder} onClick={loadOlderWithAnchor} />
          ) : null}
          <ThreadMessages
            agentName={agentName}
            sessionId={sessionId}
            isSessionRunning={isSessionRunning}
            messageCount={messageCount}
            transcriptStatus={transcriptStatus}
            transcriptError={transcriptError}
            retryTranscript={retryTranscript}
            transcriptMessages={transcriptMessages}
            sessionState={sessionState}
            failure={failure}
            startupFailed={startupFailed}
          />
        </ThreadContentRail>
      </ThreadPrimitive.Viewport>
      <ScrollToBottomPill visible={showScrollToBottom} onClick={scrollToEnd} />
    </div>
  );
}
