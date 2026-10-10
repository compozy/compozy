import { useQuery } from "@tanstack/react-query";

import { SessionNotFoundError } from "@/systems/session/adapters/session-api-errors";
import { sessionDetailOptions } from "@/systems/session/lib/query-options";
import { getSessionDisplayTitle } from "@/systems/session/lib/session-display-title";
import { useWorkspace } from "@/systems/workspace";

import type { SessionMessageParty } from "./session-message-party";

export interface SessionLabelInput {
  sessionId: string;
  /** The other session's workspace; empty = this transcript's. */
  workspaceId: string;
  /** The transcript's own workspace: a different one is named beside the link (Gap 5). */
  currentWorkspaceId: string;
  /** What the daemon recorded with the message, used until the detail read lands. */
  agentName: string | null;
  titleAtSend?: string | null;
}

/**
 * Any session a message names (S1–S4), read through the canonical detail
 * cache like `useSubagentOrigin`: the current title (a rename shows the new
 * one), the provider mark, and "a deleted session" once the read answers 404.
 * While the read is in flight the recorded title stands in; with none, the
 * label is `pending` and names nothing. A failed read that is not a 404 keeps
 * the recorded title, or the session id, because the session may still exist.
 */
export function useSessionLabel({
  sessionId,
  workspaceId,
  currentWorkspaceId,
  agentName,
  titleAtSend = null,
}: SessionLabelInput): SessionMessageParty {
  const resolvedWorkspaceId = workspaceId || currentWorkspaceId;
  const detail = useQuery({
    ...sessionDetailOptions(resolvedWorkspaceId, sessionId),
    enabled: resolvedWorkspaceId !== "" && sessionId !== "",
    refetchInterval: false,
    retry: false,
  });
  const crossWorkspace = resolvedWorkspaceId !== "" && resolvedWorkspaceId !== currentWorkspaceId;
  const workspace = useWorkspace(resolvedWorkspaceId, { enabled: crossWorkspace });
  const deleted = detail.error instanceof SessionNotFoundError;
  const recorded = titleAtSend || null;
  const pending = !detail.data && !detail.isError && recorded === null;
  const title = deleted
    ? null
    : detail.data
      ? getSessionDisplayTitle(detail.data)
      : (recorded ?? (detail.isError ? sessionId : null));
  return {
    sessionId,
    workspaceId: resolvedWorkspaceId,
    title,
    ...(pending ? { pending } : {}),
    agentName: detail.data?.agent_name || agentName,
    workspaceName: crossWorkspace ? (workspace.data?.workspace.name ?? null) : null,
  };
}
