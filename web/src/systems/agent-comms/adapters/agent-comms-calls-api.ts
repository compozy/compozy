/**
 * Call reads and mutations.
 *
 * **Every operation is workspace-scoped**, including the ones addressed by call
 * id. That is not a stylistic choice: `callSurfaceScope`
 * (`internal/api/core/calls_requests.go:26-45`) derives the scope from the route
 * itself, so a request without a `workspace_id` path segment resolves to
 * `scope = global` and the SQL predicate becomes `AND scope = 'global'`. Reading
 * a workspace call through `/api/calls/{id}` therefore answers not-found, and
 * creating through `/api/calls` makes Global work. There is no ambiguity to
 * resolve on the client and no fallback to try — the workspace route is the only
 * correct address for workspace work.
 *
 * Filters are projected from the generated operation, so a daemon-side rename
 * breaks the build here instead of silently dropping a filter.
 */
import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";
import type { ProfileScopeParams } from "@/systems/profiles";

import { AgentCommsApiError, toAgentCommsApiError } from "./agent-comms-api-error";
import type {
  CallPayload,
  CallPromptResponse,
  CallResultResponse,
  CallSupersededResponse,
  CallsListQuery,
  CallsListResponse,
  CancelCallRequest,
  CancelCallResponse,
  CreateCallRequest,
  CreateCallResponse,
} from "../types";

/**
 * The population a caller wants, before the profile scope is attached.
 *
 * Projected off the wire rather than retyped: the profile pair is what
 * `ProfileScopeParams` supplies, and everything else is a filter a surface may
 * choose. `attention` arrives for free because the daemon has it.
 */
export type CallsListFilter = Omit<CallsListQuery, "profile" | "all_profiles">;

/**
 * Build the request query.
 *
 * Written key by key rather than accumulated into a `Record` and asserted: the
 * accumulator erased every name, which made the cast load-bearing and let a
 * renamed filter pass typecheck.
 */
function listQuery(filter: CallsListFilter, scope: ProfileScopeParams): CallsListQuery {
  return {
    ...(filter.state ? { state: filter.state } : {}),
    ...(filter.attention === undefined ? {} : { attention: filter.attention }),
    ...(filter.caller ? { caller: filter.caller } : {}),
    ...(filter.child_session_id ? { child_session_id: filter.child_session_id } : {}),
    ...(filter.root_session_id ? { root_session_id: filter.root_session_id } : {}),
    ...(filter.agent ? { agent: filter.agent } : {}),
    ...(filter.cursor ? { cursor: filter.cursor } : {}),
    ...(filter.limit === undefined ? {} : { limit: filter.limit }),
    ...scope,
  };
}

export async function listCalls(
  workspaceId: string,
  filter: CallsListFilter,
  scope: ProfileScopeParams,
  signal?: AbortSignal
): Promise<CallsListResponse> {
  const { data, error, response } = await apiClient.GET("/api/workspaces/{workspace_id}/calls", {
    params: { path: { workspace_id: workspaceId }, query: listQuery(filter, scope) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw toAgentCommsApiError("Failed to load calls", response, error);
  }
  return requireResponseData(data, response, "Failed to load calls");
}

export async function fetchCall(
  workspaceId: string,
  callId: string,
  scope: ProfileScopeParams,
  signal?: AbortSignal
): Promise<CallPayload> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/calls/{call_id}",
    {
      params: { path: { workspace_id: workspaceId, call_id: callId }, query: scope },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw toAgentCommsApiError(`Call not found: ${callId}`, response, error);
    }
    throw toAgentCommsApiError(`Failed to load call ${callId}`, response, error);
  }
  return requireResponseData(data, response, `Failed to load call ${callId}`);
}

/**
 * The whole stored payload — only meaningful once the call has settled. A read
 * before then answers `409 call_not_settled`, which the caller surfaces as-is
 * rather than pretending the result is merely loading.
 */
export async function fetchCallResult(
  workspaceId: string,
  callId: string,
  scope: ProfileScopeParams,
  signal?: AbortSignal
): Promise<CallResultResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/calls/{call_id}/result",
    {
      params: { path: { workspace_id: workspaceId, call_id: callId }, query: scope },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw toAgentCommsApiError(`Failed to load the result for ${callId}`, response, error);
  }
  return requireResponseData(data, response, `Failed to load the result for ${callId}`);
}

/**
 * The whole ask, when the inline preview was bounded.
 *
 * A prompt is stored behind a reference rather than inlined on every list row,
 * so reading all of it is its own request — and, like the result fetch, an
 * explicit one the operator triggers.
 */
export async function fetchCallPrompt(
  workspaceId: string,
  callId: string,
  scope: ProfileScopeParams,
  signal?: AbortSignal
): Promise<CallPromptResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/calls/{call_id}/prompt",
    {
      params: { path: { workspace_id: workspaceId, call_id: callId }, query: scope },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw toAgentCommsApiError(`Failed to load the prompt for ${callId}`, response, error);
  }
  return requireResponseData(data, response, `Failed to load the prompt for ${callId}`);
}

/**
 * A result that landed after the call had already settled.
 *
 * Preserved as evidence and fetchable forever; it never reopens the call or
 * changes its state.
 */
export async function fetchCallSuperseded(
  workspaceId: string,
  callId: string,
  scope: ProfileScopeParams,
  signal?: AbortSignal
): Promise<CallSupersededResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/calls/{call_id}/superseded",
    {
      params: { path: { workspace_id: workspaceId, call_id: callId }, query: scope },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw toAgentCommsApiError(`Failed to load superseded evidence for ${callId}`, response, error);
  }
  return requireResponseData(data, response, `Failed to load superseded evidence for ${callId}`);
}

/**
 * Create one call in this workspace.
 *
 * The route answers two shapes — `201` with a single acceptance, or `200` with a
 * per-item array when the body carried `tasks`. This helper sends a single item,
 * so it narrows by checking for a `call_id` rather than asserting: a batch answer
 * reaching here would mean the caller sent something this function does not
 * model, and failing loudly beats reading the first item as the whole answer.
 */
export async function createCall(
  workspaceId: string,
  body: CreateCallRequest,
  profile: string,
  signal?: AbortSignal
): Promise<CreateCallResponse> {
  const { data, error, response } = await apiClient.POST("/api/workspaces/{workspace_id}/calls", {
    params: { path: { workspace_id: workspaceId }, query: { profile } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw toAgentCommsApiError("Failed to create the call", response, error);
  }
  const payload = requireResponseData(data, response, "Failed to create the call");
  if (!isCallAcceptance(payload)) {
    throw new AgentCommsApiError(
      "The runtime accepted a batch where one call was expected",
      response.status
    );
  }
  return payload;
}

function isCallAcceptance(payload: unknown): payload is CreateCallResponse {
  return (
    typeof payload === "object" &&
    payload !== null &&
    typeof (payload as { call_id?: unknown }).call_id === "string"
  );
}

/** Idempotent: cancelling a settled call answers 200 with its terminal state. */
export async function cancelCall(
  workspaceId: string,
  callId: string,
  body: CancelCallRequest,
  profile: string,
  signal?: AbortSignal
): Promise<CancelCallResponse> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/calls/{call_id}/cancel",
    {
      params: { path: { workspace_id: workspaceId, call_id: callId }, query: { profile } },
      body,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw toAgentCommsApiError(`Failed to cancel ${callId}`, response, error);
  }
  return requireResponseData(data, response, `Failed to cancel ${callId}`);
}
