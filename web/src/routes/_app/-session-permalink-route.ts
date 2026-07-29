import type { QueryClient } from "@tanstack/react-query";
import { redirect } from "@tanstack/react-router";

import type { RouterContext } from "@/integrations/tanstack-query/root-context";
import {
  cachedForeignSessionOwner,
  resolveSessionOwner,
  SessionNotFoundError,
  sessionDetailOptions,
  sessionTranscriptOptions,
  type SessionDeepLinkSearch,
  type SessionOwnerDialogState,
  type SessionPayload,
} from "@/systems/session";
import type { TopbarRouteContext } from "@/types/topbar";
import { resolveActiveWorkspaceId } from "./-route-preload";

export interface SessionPermalinkRouteContext {
  topbar: TopbarRouteContext;
  permalinkError?: string;
  /** Present only while the operator is deciding whether to switch workspaces. */
  sessionOwner?: SessionOwnerDialogState;
}

export type SessionPermalinkResolution =
  | { status: "resolved"; session: SessionPayload }
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
  const workspaceId = await resolveActiveWorkspaceId(queryClient);
  if (!workspaceId) {
    throw new SessionNotFoundError(sessionId);
  }

  const knownOwner = cachedForeignSessionOwner(queryClient, sessionId, workspaceId);
  if (knownOwner) {
    return { status: "foreign", owner: knownOwner };
  }

  let session: SessionPayload;
  try {
    session = await queryClient.ensureQueryData(sessionDetailOptions(workspaceId, sessionId));
  } catch (error) {
    if (error instanceof SessionNotFoundError) {
      return resolveForeignPermalink(queryClient, sessionId, workspaceId);
    }
    throw error;
  }

  await Promise.allSettled([
    queryClient.ensureInfiniteQueryData(sessionTranscriptOptions(workspaceId, session.id)),
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
  activeWorkspaceId: string
): Promise<SessionPermalinkResolution> {
  const owner = await resolveSessionOwner(queryClient, sessionId);
  if (!owner || owner.workspaceId === activeWorkspaceId) {
    throw new SessionNotFoundError(sessionId);
  }
  return { status: "foreign", owner };
}
