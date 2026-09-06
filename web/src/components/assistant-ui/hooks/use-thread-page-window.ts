import type { RefObject } from "react";
import { useSelector } from "@xstate/store-react";

import { useSessionRuntimeRenderContext } from "@/systems/session/hooks/use-session-runtime-render-context";
import { useTranscriptPageWindow } from "@/systems/session";

import type { ThreadScrollStore } from "./thread-scroll-context";

/**
 * Windowed page memory for the thread viewport (US-019): far pages above the
 * reader are released and reload through load-older; the reading position is
 * re-anchored before every release, and nothing is released while older history
 * is loading or an anchor is still being applied.
 */
export function useThreadPageWindow({
  sessionId,
  viewportRef,
  scrollStore,
  readVisibleMessageId,
  captureHistoryAnchor,
  isFetchingOlder,
}: {
  sessionId: string;
  viewportRef: RefObject<HTMLDivElement | null>;
  scrollStore: ThreadScrollStore;
  readVisibleMessageId: () => string | null;
  captureHistoryAnchor: () => void;
  isFetchingOlder: boolean;
}): void {
  const renderContext = useSessionRuntimeRenderContext();
  const historyAnchorActive = useSelector(
    scrollStore,
    snapshot => snapshot.context.historyAnchor !== null
  );
  useTranscriptPageWindow({
    workspaceId: renderContext?.workspaceId ?? "",
    sessionId,
    viewport: viewportRef,
    readVisibleMessageId,
    onBeforeRelease: captureHistoryAnchor,
    paused: isFetchingOlder || historyAnchorActive,
  });
}
