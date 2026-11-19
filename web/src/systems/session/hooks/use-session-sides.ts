import { useQuery } from "@tanstack/react-query";

import type { SessionSideItem } from "../adapters/session-side-api";
import { openSideChats, sideChatNeedsYou } from "../lib/session-side";
import { sessionSidesOptions } from "../lib/session-side-query-options";

const NO_SIDES: SessionSideItem[] = [];

export interface SessionSidesModel {
  /** Open side chats, newest first (the switcher order). */
  sides: SessionSideItem[];
  /** Any open side chat waits on a permission or an answer (the head toggle's dot). */
  attention: boolean;
  isLoading: boolean;
  isError: boolean;
  refetch: () => void;
}

/**
 * One session's side chats (`GET …/sides`). The catalog stream invalidates every side list of a
 * parent on `session_side_changed`, so every window showing that session rereads the same list.
 */
export function useSessionSides(
  workspaceId: string,
  sessionId: string,
  options: { enabled?: boolean } = {}
): SessionSidesModel {
  const query = useQuery(sessionSidesOptions(workspaceId, sessionId, options));
  const sides = openSideChats(query.data ?? NO_SIDES);
  return {
    sides,
    attention: sides.some(sideChatNeedsYou),
    isLoading: query.isPending && query.fetchStatus !== "idle",
    isError: query.isError,
    refetch: () => {
      void query.refetch();
    },
  };
}

/** Whether a session has any side chat, closed ones included (the delete confirmation line). */
export function useSessionHasSideChats(
  workspaceId: string,
  sessionId: string,
  options: { enabled?: boolean } = {}
): boolean {
  const query = useQuery(
    sessionSidesOptions(workspaceId, sessionId, { ...options, archived: true })
  );
  return (query.data?.length ?? 0) > 0;
}
