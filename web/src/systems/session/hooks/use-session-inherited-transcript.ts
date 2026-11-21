import type { ThreadMessage } from "@assistant-ui/react";
import { useInfiniteQuery } from "@tanstack/react-query";

import {
  isInheritedTranscriptUnavailable,
  sessionInheritedTranscriptOptions,
  type SessionInheritedView,
} from "../lib/session-inherited-transcript";
import { toReadonlyThreadMessages } from "../lib/session-thread-repository";
import { flattenTranscriptMessages } from "../lib/session-transcript-query";

export type SessionInheritedTranscriptStatus =
  | "idle"
  | "pending"
  | "unavailable"
  | "error"
  | "ready";

export interface SessionInheritedTranscriptState {
  status: SessionInheritedTranscriptStatus;
  /** Oldest first, read-only: the source's rows through the cut, never the child's. */
  messages: readonly ThreadMessage[];
  hasOlder: boolean;
  isFetchingOlder: boolean;
  /** An older page failed; the loaded rows stay and the older control retries. */
  olderFailed: boolean;
  loadOlder: () => void;
  retry: () => void;
}

const NO_MESSAGES: readonly ThreadMessage[] = [];

/**
 * The inherited block's read: nothing is requested until the operator expands
 * it, then the tail page loads and older pages follow on demand (#1885 C3).
 */
export function useSessionInheritedTranscript(
  view: SessionInheritedView,
  expanded: boolean
): SessionInheritedTranscriptState {
  const query = useInfiniteQuery({
    ...sessionInheritedTranscriptOptions(view.workspaceId, view.sessionId),
    enabled: expanded && view.sessionId !== "",
  });
  const loaded = query.data !== undefined && query.data.pages.length > 0;
  let status: SessionInheritedTranscriptStatus = "idle";
  if (loaded) status = "ready";
  else if (query.isError) {
    status = isInheritedTranscriptUnavailable(query.error) ? "unavailable" : "error";
  } else if (expanded) status = "pending";
  const messages = loaded
    ? toReadonlyThreadMessages(flattenTranscriptMessages(query.data))
    : NO_MESSAGES;
  return {
    status,
    messages,
    hasOlder: query.hasNextPage,
    isFetchingOlder: query.isFetchingNextPage,
    olderFailed: loaded && query.isFetchNextPageError,
    loadOlder: () => void query.fetchNextPage(),
    retry: () => void query.refetch(),
  };
}
