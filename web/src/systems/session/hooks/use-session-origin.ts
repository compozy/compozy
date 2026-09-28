import { useQuery } from "@tanstack/react-query";

import { sessionDetailOptions } from "../lib/query-options";
import { getSessionDisplayTitle } from "../lib/session-display-title";
import {
  sessionOriginKind,
  sessionOriginView,
  type SessionOriginParent,
  type SessionOriginView,
} from "../lib/session-origin";
import type { SessionPayload } from "../types";

/**
 * Where a continued or forked session came from, with the source read through
 * the canonical detail cache: its title names the divider, and whether it is
 * readable decides if the pill and divider may open it (a deleted source keeps
 * the words, not the door).
 */
export function useSessionOrigin(
  session: SessionPayload,
  workspaceId: string
): SessionOriginView | null {
  const kind = sessionOriginKind(session);
  const parentId = session.lineage?.parent_session_id?.trim() ?? "";
  const parentQuery = useQuery({
    ...sessionDetailOptions(workspaceId, parentId),
    enabled: kind !== null && workspaceId !== "" && parentId !== "",
    refetchInterval: false,
    retry: false,
  });
  let parent: SessionOriginParent | null | undefined;
  if (parentQuery.data) parent = { title: getSessionDisplayTitle(parentQuery.data) };
  else if (parentQuery.isError) parent = null;
  return sessionOriginView(session, parent);
}
