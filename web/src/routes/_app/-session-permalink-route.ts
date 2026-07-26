import type { QueryClient } from "@tanstack/react-query";
import { redirect } from "@tanstack/react-router";

import type { RouterContext } from "@/integrations/tanstack-query/root-context";
import {
  SessionNotFoundError,
  sessionByIdOptions,
  sessionDetailOptions,
  sessionKeys,
  sessionTranscriptOptions,
  type SessionPayload,
} from "@/systems/session";
import type { TopbarRouteContext } from "@/types/topbar";

export interface SessionPermalinkRouteContext {
  topbar: TopbarRouteContext;
  permalinkError?: string;
}

export async function redirectSessionPermalinkRoute({
  context,
  params,
}: {
  context: RouterContext;
  params: { id: string };
}): Promise<SessionPermalinkRouteContext> {
  const topbar = { crumb: { label: "Session" } };
  let session: SessionPayload;
  try {
    session = await resolveSessionPermalink({
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

  throw redirect({
    to: "/agents/$name/sessions/$id",
    params: { name: session.agent_name, id: session.id },
    replace: true,
  });
}

export async function resolveSessionPermalink({
  queryClient,
  sessionId,
}: {
  queryClient: QueryClient;
  sessionId: string;
}): Promise<SessionPayload> {
  const session = await queryClient.ensureQueryData(sessionByIdOptions(sessionId));
  const workspaceId = session.workspace_id?.trim();
  if (workspaceId) {
    queryClient.setQueryData(sessionKeys.detail(workspaceId, session.id), session);
    await Promise.allSettled([
      queryClient.ensureInfiniteQueryData(sessionTranscriptOptions(workspaceId, session.id)),
      queryClient.ensureQueryData(sessionDetailOptions(workspaceId, session.id)),
    ]);
  }
  return session;
}
