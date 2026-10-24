import type { QueryClient } from "@tanstack/react-query";
import { redirect } from "@tanstack/react-router";

import type { RouterContext } from "@/integrations/tanstack-query/root-context";

import type { TopbarRouteContext } from "@/types/topbar";
import { resolveActiveWorkspaceSelection, settleRouteQueries } from "./-route-preload";
import { sessionRouteOwnership, type SessionRouteOwnership } from "./-session-route-ownership";
import {
  cachedForeignSessionOwner,
  resolveSessionOwner,
  sessionAcrossProfilesOptions,
  sessionDetailOptions,
  type SessionDeepLinkSearch,
  sessionScopedDetailOptions,
  SessionNotFoundError,
  type SessionOwnerDialogState,
  type SessionPayload,
  sessionTranscriptOptions,
} from "@/systems/session";
import { readProfileScopeParams } from "@/systems/profiles";
import { ownerDataScope, windowBindingValue } from "@/systems/workspace";

export interface SessionPermalinkRouteContext {
  topbar: TopbarRouteContext;
  permalinkError?: string;
  /** Present only while the operator is deciding whether to switch workspaces. */
  sessionOwner?: SessionOwnerDialogState;
}

export type SessionPermalinkResolution =
  | {
      status: "resolved";
      session: SessionPayload;
      /** Window binding when the All projects desktop opens the session in place. */
      binding?: string;
    }
  | { status: "foreign"; owner: SessionOwnerDialogState };

export async function redirectSessionPermalinkRoute({
  context,
  params,
  search,
}: {
  context: RouterContext;
  params: { id: string };
  search: SessionDeepLinkSearch;
}): Promise<SessionPermalinkRouteContext> {
  const topbar = { crumb: { label: "Session" } };
  let resolution: SessionPermalinkResolution;
  try {
    resolution = await resolveSessionPermalink({
      queryClient: context.queryClient,
      sessionId: params.id,
    });
  } catch (error) {
    return {
      topbar,
      permalinkError:
        error instanceof SessionNotFoundError
          ? "Session not found"
          : error instanceof Error
            ? error.message
            : "Session not found",
    };
  }

  if (resolution.status === "foreign") {
    // Declining leaves today's not-found rendering in place; the URL keeps that decision replayable.
    if (search.workspaceSwitch === "declined") {
      return { topbar, permalinkError: "Session not found" };
    }
    if (search.workspaceSwitch !== "confirm") {
      throw redirect({
        to: "/session/$id",
        params,
        search: { workspaceSwitch: "confirm" },
        replace: true,
      });
    }
    return { topbar, sessionOwner: resolution.owner };
  }

  throw redirect({
    to: "/agents/$name/sessions/$id",
    params: { name: resolution.session.agent_name, id: resolution.session.id },
    search: resolution.binding ? { workspace: resolution.binding } : {},
    replace: true,
  });
}

export async function resolveSessionPermalink({
  queryClient,
  sessionId,
}: {
  queryClient: QueryClient;
  sessionId: string;
}): Promise<SessionPermalinkResolution> {
  // Session windows belong to the mounted desktop's partition; the All
  // projects desktop opens any session in place, bound to its own scope.
  const ownership = sessionRouteOwnership(await resolveActiveWorkspaceSelection(queryClient));
  if (!ownership) {
    throw new SessionNotFoundError(sessionId);
  }

  const knownOwner = cachedForeignSessionOwner(queryClient, sessionId, ownership.ownerWorkspaceId);
  if (knownOwner) {
    if (ownership.accepts(knownOwner.workspaceId)) {
      return resolveForeignPermalink(queryClient, sessionId, ownership);
    }
    return { status: "foreign", owner: knownOwner };
  }
  if (ownership.proofSelector === null) {
    return resolveForeignPermalink(queryClient, sessionId, ownership);
  }

  let session: SessionPayload;
  try {
    // The profile-aware by-id endpoint does not enforce workspace ownership.
    // Prove the desktop's scope first (`@user` on the User desktop) so a
    // permalink cannot jump across destinations without the operator's
    // confirmation.
    await queryClient.ensureQueryData(sessionDetailOptions(ownership.proofSelector, sessionId));
    session = await queryClient.ensureQueryData(
      sessionScopedDetailOptions(sessionId, readProfileScopeParams(queryClient))
    );
  } catch (error) {
    if (error instanceof SessionNotFoundError) {
      return resolveForeignPermalink(queryClient, sessionId, ownership);
    }
    throw error;
  }

  await settleRouteQueries([
    queryClient.ensureInfiniteQueryData(
      sessionTranscriptOptions(ownership.ownerWorkspaceId, session.id)
    ),
  ]);
  return { status: "resolved", session };
}

/**
 * Only the minimal owner projection may answer a permalink miss — no foreign detail, transcript,
 * or catalog read happens before the operator confirms the workspace switch (ADR-004).
 */
async function resolveForeignPermalink(
  queryClient: QueryClient,
  sessionId: string,
  ownership: SessionRouteOwnership
): Promise<SessionPermalinkResolution> {
  const owner = await resolveSessionOwner(queryClient, sessionId);
  if (!owner) {
    throw new SessionNotFoundError(sessionId);
  }
  if (ownership.accepts(owner.workspaceId)) {
    const session = await queryClient.ensureQueryData(sessionAcrossProfilesOptions(sessionId));
    await settleRouteQueries([
      queryClient.ensureInfiniteQueryData(sessionTranscriptOptions(owner.workspaceId, sessionId)),
    ]);
    return {
      status: "resolved",
      session,
      ...(ownership.bindsInPlace
        ? { binding: windowBindingValue(ownerDataScope(owner.workspaceId)) }
        : {}),
    };
  }
  return { status: "foreign", owner };
}
