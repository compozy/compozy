import type { QueryClient } from "@tanstack/react-query";

import {
  ensureRouteQueryData,
  resolveActiveWorkspaceSelection,
  settleRouteQueries,
} from "./-route-preload";
import { sessionRouteOwnership, type SessionRouteOwnership } from "./-session-route-ownership";
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
import { readProfileScopeParams } from "@/systems/profiles";
import { ownerDataScope, windowBindingValue } from "@/systems/workspace";

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
  | {
      status: "loaded";
      workspaceId: string;
      /** Window binding when the All projects desktop opens the session in place. */
      binding?: string;
    }
  | { status: "foreign"; owner: SessionOwnerDialogState }
  | { status: "not-found" };

export async function prefetchAgentSessionRoute({
  queryClient,
  sessionId,
}: {
  queryClient: QueryClient;
  sessionId: string;
}): Promise<AgentSessionRouteLoaderData> {
  const ownership = sessionRouteOwnership(await resolveActiveWorkspaceSelection(queryClient));
  if (!ownership) {
    return { status: "not-found" };
  }

  const knownOwner = cachedForeignSessionOwner(queryClient, sessionId, ownership.ownerWorkspaceId);
  if (knownOwner) {
    if (ownership.accepts(knownOwner.workspaceId)) {
      return resolveForeignSession(queryClient, sessionId, ownership);
    }
    return { status: "foreign", owner: knownOwner };
  }
  if (ownership.proofSelector === null) {
    return resolveForeignSession(queryClient, sessionId, ownership);
  }

  try {
    // Prove ownership before the profile-aware by-id read. The by-id endpoint
    // enforces the profile lens but is intentionally workspace-agnostic; using
    // it alone would silently open another destination's session. The User
    // desktop proves through `@user`, which resolves to the request's profile.
    const proofSelector = ownership.proofSelector;
    await ensureRouteQueryData(() =>
      queryClient.ensureQueryData(sessionDetailOptions(proofSelector, sessionId))
    );
    await ensureRouteQueryData(() =>
      queryClient.ensureQueryData(
        sessionScopedDetailOptions(sessionId, readProfileScopeParams(queryClient))
      )
    );
  } catch (error) {
    if (error instanceof SessionNotFoundError) {
      return resolveForeignSession(queryClient, sessionId, ownership);
    }
    throw error;
  }

  await settleRouteQueries([
    queryClient.ensureInfiniteQueryData(
      sessionTranscriptOptions(ownership.ownerWorkspaceId, sessionId)
    ),
  ]);

  return { status: "loaded", workspaceId: ownership.ownerWorkspaceId };
}

/**
 * The miss is resolved by the minimal owner projection alone — never by the session catalog, and
 * never by reading the foreign detail or transcript before the operator confirms.
 */
async function resolveForeignSession(
  queryClient: QueryClient,
  sessionId: string,
  ownership: SessionRouteOwnership
): Promise<AgentSessionRouteLoaderData> {
  const owner = await ensureRouteQueryData(() => resolveSessionOwner(queryClient, sessionId));
  if (!owner) {
    return { status: "not-found" };
  }
  if (ownership.accepts(owner.workspaceId)) {
    await ensureRouteQueryData(() =>
      queryClient.ensureQueryData(sessionAcrossProfilesOptions(sessionId))
    );
    await settleRouteQueries([
      queryClient.ensureInfiniteQueryData(sessionTranscriptOptions(owner.workspaceId, sessionId)),
    ]);
    return {
      status: "loaded",
      workspaceId: owner.workspaceId,
      ...(ownership.bindsInPlace
        ? { binding: windowBindingValue(ownerDataScope(owner.workspaceId)) }
        : {}),
    };
  }
  return { status: "foreign", owner };
}
