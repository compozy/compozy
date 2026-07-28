import type { QueryClient } from "@tanstack/react-query";

import {
  SessionNotFoundError,
  sessionByIdOptions,
  sessionDetailOptions,
  sessionKeys,
  sessionTranscriptOptions,
  type SessionPayload,
} from "@/systems/session";
import { selectRouteWorkspaceForNavigation } from "./-route-preload";

export interface AgentSessionRouteLoaderData {
  workspaceId: string | null;
}

export async function prefetchAgentSessionRoute({
  queryClient,
  sessionId,
  preload = false,
}: {
  queryClient: QueryClient;
  sessionId: string;
  preload?: boolean;
}): Promise<AgentSessionRouteLoaderData> {
  const workspaceId = await resolveSessionRouteWorkspace(queryClient, sessionId);
  if (!workspaceId) {
    return { workspaceId: null };
  }

  // A session navigation always lands in the session's own space: selecting
  // the owner here starts the workspace-switch cycle, and the shell opens the
  // window there once the target space hydrates (Routing rule 8, US-016.EC-2).
  // Hover/viewport preloads must never switch the operator's workspace.
  if (!preload) {
    await selectRouteWorkspaceForNavigation(queryClient, workspaceId);
  }
  await Promise.allSettled([
    queryClient.ensureQueryData(sessionDetailOptions(workspaceId, sessionId)),
    queryClient.ensureInfiniteQueryData(sessionTranscriptOptions(workspaceId, sessionId)),
  ]);

  return { workspaceId };
}

async function resolveSessionRouteWorkspace(
  queryClient: QueryClient,
  sessionId: string
): Promise<string | null> {
  const cachedWorkspaceId = normalizeWorkspaceId(
    queryClient.getQueryData<SessionPayload>(sessionKeys.byId(sessionId))?.workspace_id
  );
  if (cachedWorkspaceId) {
    return cachedWorkspaceId;
  }
  try {
    const session = await queryClient.ensureQueryData(sessionByIdOptions(sessionId));
    return normalizeWorkspaceId(session.workspace_id);
  } catch (error) {
    if (error instanceof SessionNotFoundError) {
      return null;
    }
    throw error;
  }
}

function normalizeWorkspaceId(value: string | null | undefined): string | null {
  const trimmed = value?.trim();
  return trimmed ? trimmed : null;
}
