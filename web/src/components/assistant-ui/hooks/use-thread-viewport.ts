import { type RefObject } from "react";
import { useSelector } from "@xstate/store-react";

import { useSessionRuntimeRenderContext } from "@/systems/session/hooks/use-session-runtime-render-context";
import { useSessionTranscriptThreadState, useTranscriptPageWindow } from "@/systems/session";

import { useSessionFindHighlights } from "./use-session-find-highlights";
import { useSessionNavigationHost } from "./use-session-navigation-host";
import { useThreadScrollController } from "./use-thread-scroll-controller";

/**
 * Everything the transcript scroller composes behind its markup: the thread's
 * transcript state, the one scroll owner, the navigation host (find + trail
 * jumps through that owner), the in-transcript match marks, and the windowed
 * page memory — paused while older history loads, while a reading position
 * is being restored, and while a navigation jump loads or lands.
 */
export function useThreadViewport({
  sessionId,
  isSessionRunning,
  viewportRef,
  contentRef,
}: {
  sessionId: string;
  isSessionRunning: boolean;
  viewportRef: RefObject<HTMLDivElement | null>;
  contentRef: RefObject<HTMLDivElement | null>;
}) {
  const transcript = useSessionTranscriptThreadState();
  const messageCount = transcript.messages.length;
  const showLoadOlder = transcript.hasOlder || transcript.isFetchingOlder;
  const scroll = useThreadScrollController(
    viewportRef,
    contentRef,
    transcript.messages,
    showLoadOlder && messageCount > 0
  );
  const loadOlderWithAnchor = () => {
    scroll.captureHistoryAnchor();
    transcript.loadOlder();
  };
  const renderContext = useSessionRuntimeRenderContext();
  const workspaceId = renderContext?.workspaceId ?? "";
  const navigation = useSessionNavigationHost({
    isSessionRunning,
    messageCount,
    readVisibleMessageIds: scroll.readVisibleMessageIds,
    scrollToMessage: scroll.scrollToMessage,
    sessionId,
    viewportRef,
    workspaceId,
  });
  useSessionFindHighlights(contentRef, navigation.target.query, navigation.target.activeMessageId);
  const historyAnchorActive = useSelector(
    scroll.store,
    snapshot => snapshot.context.historyAnchor !== null
  );
  useTranscriptPageWindow({
    workspaceId,
    sessionId,
    viewport: viewportRef,
    readVisibleMessageId: scroll.readVisibleMessageId,
    onBeforeRelease: scroll.captureHistoryAnchor,
    paused: transcript.isFetchingOlder || historyAnchorActive || navigation.jumpActive,
  });

  return {
    loadOlderWithAnchor,
    messageCount,
    navigation,
    scroll,
    showLoadOlder,
    transcript,
    workspaceId,
  };
}
