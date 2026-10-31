import { useQuery } from "@tanstack/react-query";

import { SessionNotFoundError } from "@/systems/session/adapters/session-api-errors";
import { sessionDetailOptions, sessionOwnerOptions } from "@/systems/session/lib/query-options";
import { getSessionDisplayTitle } from "@/systems/session/lib/session-display-title";
import { useWorkspace } from "@/systems/workspace";

import type { SessionMessageParty } from "./session-message-party";

export interface SessionLabelInput {
  sessionId: string;
  /** The other session's workspace; empty resolves its canonical owner. */
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
  const owner = useQuery({
    ...sessionOwnerOptions(sessionId),
    enabled: workspaceId === "" && sessionId !== "",
  });
  const resolvedWorkspaceId = workspaceId || ownerWorkspaceSelector(owner.data);
  const detail = useQuery({
    ...sessionDetailOptions(resolvedWorkspaceId, sessionId),
    enabled: resolvedWorkspaceId !== "" && sessionId !== "",
    refetchInterval: false,
    retry: false,
  });
  const crossWorkspace = resolvedWorkspaceId !== "" && resolvedWorkspaceId !== currentWorkspaceId;
  const userDestination = resolvedWorkspaceId === "@user";
  const workspace = useWorkspace(resolvedWorkspaceId, {
    enabled: crossWorkspace && !userDestination,
  });
  const labelRead = workspaceId === "" && !owner.data ? owner : detail;
  const { title, pending } = sessionLabelTitle(
    { data: detail.data, error: labelRead.error, isError: labelRead.isError },
    sessionId,
    titleAtSend || null
  );
  return {
    sessionId,
    workspaceId: userDestination ? "" : resolvedWorkspaceId,
    title,
    ...(pending ? { pending } : {}),
    agentName: detail.data?.agent_name || agentName,
    provider: sessionRuntimeProvider(detail.data?.runtime),
    workspaceName: sessionLabelWorkspaceName(
      crossWorkspace,
      userDestination,
      workspace.data?.workspace.name
    ),
  };
}

interface SessionRuntimeRoutes {
  effective?: { provider?: string } | null;
  selected?: { provider?: string } | null;
}

function sessionRuntimeProvider(runtime: SessionRuntimeRoutes | null | undefined): string | null {
  return runtime?.effective?.provider?.trim() || runtime?.selected?.provider?.trim() || null;
}

interface SessionDetailRead {
  data?: Parameters<typeof getSessionDisplayTitle>[0];
  error: unknown;
  isError: boolean;
}

// The label's title: the current one once read, nothing for a deleted session,
// the recorded title while the read is in flight, and the id after a failed read.
function sessionLabelTitle(
  detail: SessionDetailRead,
  sessionId: string,
  recorded: string | null
): { title: string | null; pending: boolean } {
  if (detail.error instanceof SessionNotFoundError) return { title: null, pending: false };
  if (detail.data) return { title: getSessionDisplayTitle(detail.data), pending: false };
  if (detail.isError) return { title: recorded ?? sessionId, pending: false };
  return { title: recorded, pending: recorded === null };
}

/** An unresolved owner disables detail reads; a resolved owner without a project is User. */
function ownerWorkspaceSelector(owner: { workspace_id?: string } | undefined): string {
  return owner ? owner.workspace_id || "@user" : "";
}

/** Name foreign destinations without querying project metadata for the User scope. */
function sessionLabelWorkspaceName(
  crossWorkspace: boolean,
  user: boolean,
  name: string | undefined
): string | null {
  if (!crossWorkspace) return null;
  return user ? "User" : (name ?? null);
}
