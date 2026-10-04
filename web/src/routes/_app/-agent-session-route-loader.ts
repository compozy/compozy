import type { QueryClient } from "@tanstack/react-query";

import { resolveActiveWorkspaceSelection } from "./-route-preload";
import {
  cachedForeignSessionOwner,
  resolveSessionOwner,
  sessionAcrossProfilesOptions,
  sessionDetailOptions,
  sessionScopedDetailOptions,
  SessionNotFoundError,
  type SessionOwnerDialogState,
  sessionTranscriptOptions,
} from "@/systems/session";
import { readProfileLens, readProfileScopeParams } from "@/systems/profiles";

/**
 * A deep link either loads under the active workspace, belongs to another workspace (the operator
 * confirms the switch — ADR-004), or exists nowhere. Only the `loaded` branch reads session data;
 * the `foreign` branch carries the owner projection and nothing else.
 *
 * This loader decides the *workspace* axis only. The profile axis is resolved when the window
 * mounts, against the by-id route that enforces it — reading it here would make a workspace miss
 * and a profile miss indistinguishable, and this route's confirm dialog depends on telling them
 * apart.
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
  const selection = await resolveActiveWorkspaceSelection(queryClient);
  const workspaceId = selection.desktopWorkspaceId;
  if (!workspaceId) {
    return { status: "not-found" };
  }

  const knownOwner = cachedForeignSessionOwner(queryClient, sessionId, workspaceId);
  if (knownOwner) {
    if (knownOwner.workspaceId === "" && selection.scope === "global") {
      return resolveForeignSession(queryClient, sessionId, workspaceId, true);
    }
    return { status: "foreign", owner: knownOwner };
  }

  try {
    // Prove workspace ownership before the profile-aware by-id read. The by-id
    // endpoint enforces the profile lens but is intentionally workspace-agnostic;
    // using it alone would silently open a foreign workspace's session.
    await queryClient.ensureQueryData(sessionDetailOptions(workspaceId, sessionId));
    await queryClient.ensureQueryData(
      sessionScopedDetailOptions(sessionId, readProfileScopeParams(queryClient, readProfileLens()))
    );
  } catch (error) {
    if (error instanceof SessionNotFoundError) {
      return resolveForeignSession(
        queryClient,
        sessionId,
        workspaceId,
        selection.scope === "global"
      );
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
  activeWorkspaceId: string,
  globalScope: boolean
): Promise<AgentSessionRouteLoaderData> {
  const owner = await resolveSessionOwner(queryClient, sessionId);
  if (!owner) {
    return { status: "not-found" };
  }
  if (owner.workspaceId === activeWorkspaceId || (globalScope && owner.workspaceId === "")) {
    await queryClient.ensureQueryData(sessionAcrossProfilesOptions(sessionId));
    await Promise.allSettled([
      queryClient.ensureInfiniteQueryData(sessionTranscriptOptions(owner.workspaceId, sessionId)),
    ]);
    return { status: "loaded", workspaceId: owner.workspaceId };
  }
  return { status: "foreign", owner };
}
