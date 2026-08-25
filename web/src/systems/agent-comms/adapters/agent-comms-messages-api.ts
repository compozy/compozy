/**
 * Mailbox reads and sends.
 *
 * Workspace-routed for the same reason calls are: the daemon derives scope from
 * the route, so `POST /api/messages` without a workspace segment addresses Global
 * work. See the note in `agent-comms-calls-api.ts`.
 *
 * Unlike calls, message pages are **uncounted** — `MessagePage` in
 * `internal/calls/read.go` carries only items and a cursor, and the wire response
 * has no `total`. Message surfaces therefore never render one; substituting a
 * page length for a count is the exact thing the truthful-UI rule forbids.
 *
 * There is also no read/seen state anywhere in the runtime, so nothing here
 * reads or writes one and no surface may show an unread mark.
 */
import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";
import type { ProfileScopeParams } from "@/systems/profiles";

import { toAgentCommsApiError } from "./agent-comms-api-error";
import type {
  CallMessagesListQuery,
  CallMessagesListResponse,
  SendCallMessageRequest,
  SendCallMessageResponse,
} from "../types";

/** Projected off the wire; the profile pair comes from `ProfileScopeParams`. */
export type CallMessagesFilter = Omit<CallMessagesListQuery, "profile" | "all_profiles">;

function messagesQuery(
  filter: CallMessagesFilter,
  scope: ProfileScopeParams
): CallMessagesListQuery {
  return {
    ...(filter.session ? { session: filter.session } : {}),
    ...(filter.cursor ? { cursor: filter.cursor } : {}),
    ...(filter.limit === undefined ? {} : { limit: filter.limit }),
    ...scope,
  };
}

export async function listCallMessages(
  workspaceId: string,
  filter: CallMessagesFilter,
  scope: ProfileScopeParams,
  signal?: AbortSignal
): Promise<CallMessagesListResponse> {
  const { data, error, response } = await apiClient.GET("/api/workspaces/{workspace_id}/messages", {
    params: { path: { workspace_id: workspaceId }, query: messagesQuery(filter, scope) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw toAgentCommsApiError("Failed to load messages", response, error);
  }
  return requireResponseData(data, response, "Failed to load messages");
}

/**
 * Accepted with a delivery receipt, not a delivery guarantee: `202` carries
 * `queued`, `delivered-into-turn`, or `woke`, and the receipt updates in place
 * on the message record as the runtime moves it along.
 */
export async function sendCallMessage(
  workspaceId: string,
  body: SendCallMessageRequest,
  profile: string,
  signal?: AbortSignal
): Promise<SendCallMessageResponse> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/messages",
    {
      params: { path: { workspace_id: workspaceId }, query: { profile } },
      body,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw toAgentCommsApiError("Failed to send the message", response, error);
  }
  return requireResponseData(data, response, "Failed to send the message");
}
