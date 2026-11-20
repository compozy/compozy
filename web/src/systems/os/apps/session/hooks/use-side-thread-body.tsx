import { useQuery } from "@tanstack/react-query";

import { SessionThreadSkeleton } from "@/components/assistant-ui/session-thread-skeleton";
import {
  type SessionPayload,
  type SessionSidePanelBodyState,
  sessionDetailOptions,
} from "@/systems/session";

import { SessionSideThread } from "../session-side-thread";

/** The body for an existing side chat: its thread once its session reads, Retry when it can't. */
export function useSideThreadBody(
  workspaceId: string,
  sideId: string | null
): SessionSidePanelBodyState {
  const query = useQuery(sessionDetailOptions(workspaceId, sideId ?? ""));
  if (query.isError) return { kind: "unavailable", onRetry: () => void query.refetch() };
  const side: SessionPayload | undefined = query.data;
  if (!side) return { kind: "thread", thread: <SessionThreadSkeleton /> };
  return {
    kind: "thread",
    stopped: side.state === "stopped",
    thread: <SessionSideThread side={side} workspaceId={workspaceId} />,
  };
}
