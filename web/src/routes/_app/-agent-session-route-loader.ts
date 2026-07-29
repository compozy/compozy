import type { QueryClient } from "@tanstack/react-query";

import {
  cachedForeignSessionOwner,
  resolveForeignSessionOwner,
  SessionNotFoundError,
  sessionDetailOptions,
  sessionTranscriptOptions,
  type SessionOwnerDialogState,
} from "@/systems/session";
import { resolveActiveWorkspaceId } from "./-route-preload";

/**
 * A deep link either loads under the active workspace, belongs to another workspace (the operator
 * confirms the switch — ADR-004), or exists nowhere. Only the `loaded` branch reads session data;
 * the `foreign` branch carries the owner projection and nothing else.
 */
export type AgentSessionRouteLoaderData =
  | { status: "loaded"; workspaceId: string }
  | { status: "foreign"; owner: SessionOwnerDialogState }
  | { status: "not-found" };

export async function prefetchAgentSessionRoute({
  queryClient,
  sessionId,
}: {
  queryClient: QueryClient;
  sessionId: string;
}): Promise<AgentSessionRouteLoaderData> {
  const workspaceId = await resolveActiveWorkspaceId(queryClient);
  if (!workspaceId) {
    return { status: "not-found" };
  }

  const knownOwner = cachedForeignSessionOwner(queryClient, sessionId, workspaceId);
  if (knownOwner) {
    return { status: "foreign", owner: knownOwner };
  }

  try {
    await queryClient.ensureQueryData(sessionDetailOptions(workspaceId, sessionId));
  } catch (error) {
    if (error instanceof SessionNotFoundError) {
      return resolveForeignSession(queryClient, sessionId, workspaceId);
    }
    throw error;
  }

  await Promise.allSettled([
    queryClient.ensureInfiniteQueryData(sessionTranscriptOptions(workspaceId, sessionId)),
  ]);

  return { status: "loaded", workspaceId };
}

/**
 * The miss is resolved by the minimal owner projection alone — never by the session catalog, and
 * never by reading the foreign detail or transcript before the operator confirms.
 */
async function resolveForeignSession(
  queryClient: QueryClient,
  sessionId: string,
  activeWorkspaceId: string
): Promise<AgentSessionRouteLoaderData> {
  const owner = await resolveForeignSessionOwner(queryClient, sessionId, activeWorkspaceId);
  if (!owner) {
    return { status: "not-found" };
  }
  return { status: "foreign", owner };
}
