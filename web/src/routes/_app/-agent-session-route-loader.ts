import type { QueryClient } from "@tanstack/react-query";

import {
  SessionNotFoundError,
  sessionDetailOptions,
  sessionTranscriptOptions,
} from "@/systems/session";
import { resolveActiveWorkspaceId } from "./-route-preload";

export interface AgentSessionRouteLoaderData {
  workspaceId: string | null;
}

export async function prefetchAgentSessionRoute({
  queryClient,
  sessionId,
}: {
  queryClient: QueryClient;
  sessionId: string;
}): Promise<AgentSessionRouteLoaderData> {
  const workspaceId = await resolveActiveWorkspaceId(queryClient);
  if (!workspaceId) {
    return { workspaceId: null };
  }

  try {
    await queryClient.ensureQueryData(sessionDetailOptions(workspaceId, sessionId));
  } catch (error) {
    if (error instanceof SessionNotFoundError) {
      return { workspaceId: null };
    }
    throw error;
  }

  await Promise.allSettled([
    queryClient.ensureInfiniteQueryData(sessionTranscriptOptions(workspaceId, sessionId)),
  ]);

  return { workspaceId };
}
