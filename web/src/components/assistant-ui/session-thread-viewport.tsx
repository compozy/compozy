import { type ComponentPropsWithoutRef, useRef } from "react";

import { cn } from "@/lib/utils";

import { SessionNavigationTargetContext } from "./hooks/session-navigation-target-context";
import { useThreadViewport } from "./hooks/use-thread-viewport";
import { ScrollToBottomPill } from "./scroll-to-bottom-pill";
import { ThreadContentRail, type SessionThreadContentInset } from "./session-thread-content-rail";
import { ThreadMessages } from "./session-thread-messages";
import { SessionFindBar } from "@/systems/session/components/session-find-bar";
import { SessionMessageTrail } from "@/systems/session/components/session-message-trail";
import type { SessionFailurePayload, SessionState } from "@/systems/session";

type ThreadViewportProps = ComponentPropsWithoutRef<"div">;

/**
 * The transcript scroller: renders the thread rows inside the shared content
 * rail, keeps the reader anchored while older history loads above, floats the
 * scroll-to-bottom pill over its own edge, and hosts navigation (S8/S9): the
 * find bar docked above the scroller, the message trail on its left edge, and
 * both jumps landing through the one scroll owner.
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
    loadOlderWithAnchor,
    messageCount,
    navigation,
    scroll,
    showLoadOlder,
    transcript,
    workspaceId,
  } = useThreadViewport({ contentRef, isSessionRunning, sessionId, viewportRef });

  return (
    <div className="relative flex min-h-0 min-w-0 flex-1 flex-col">
      {navigation.findOpen ? (
        <SessionFindBar
          agentName={agentName}
          find={navigation.find}
          focusKey={navigation.findFocusKey}
          isSequenceFolded={navigation.isSequenceFolded}
          onClose={navigation.closeFind}
          resolveMatchTime={navigation.resolveMatchTime}
        />
      ) : null}
      <div className="relative flex min-h-0 min-w-0 flex-1">
        <div
          {...props}
          ref={viewportRef}
          className={cn("min-h-0 min-w-0 flex-1 overflow-y-auto", className)}
          data-testid="chat-view"
        >
          <ThreadContentRail
            ref={contentRef}
            inset={contentInset}
            className={cn("min-h-full", workspaceId && navigation.trail.enabled && "pl-9")}
          >
            <SessionNavigationTargetContext.Provider value={navigation.target}>
              <ThreadMessages
                agentName={agentName}
                sessionId={sessionId}
                isSessionRunning={isSessionRunning}
                messageCount={messageCount}
                transcriptStatus={transcript.status}
                transcriptError={transcript.error}
                retryTranscript={transcript.retry}
                transcriptMessages={transcript.messages}
                measureVirtualElement={scroll.measureVirtualElement}
                virtualItems={scroll.virtualItems}
                virtualTotalSize={scroll.virtualTotalSize}
                leadingItemCount={scroll.leadingItemCount}
                showLoadOlder={showLoadOlder}
                isFetchingOlder={transcript.isFetchingOlder}
                loadOlder={loadOlderWithAnchor}
                sessionState={sessionState}
                failure={failure}
                startupFailed={startupFailed}
              />
            </SessionNavigationTargetContext.Provider>
          </ThreadContentRail>
        </div>
        {workspaceId && navigation.trail.enabled ? (
          <SessionMessageTrail
            className="absolute top-1/2 left-1 z-10 -translate-y-1/2"
            onJumpToSequence={navigation.trail.onJumpToSequence}
            paneHeightPx={navigation.trail.paneHeightPx}
            refreshKey={navigation.refreshKey}
            sessionId={sessionId}
            viewportTopSequence={navigation.trail.viewportTopSequence}
            visibleRange={navigation.trail.visibleRange}
            workspaceId={workspaceId}
          />
        ) : null}
        <ScrollToBottomPill visible={scroll.showScrollToBottom} onClick={scroll.scrollToEnd} />
      </div>
    </div>
  );
}
