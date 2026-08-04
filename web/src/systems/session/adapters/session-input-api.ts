import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type {
  PromoteSessionInputRequest,
  ReplaceSessionInputRequest,
  SessionInputPayload,
  SessionInputsResponse,
  SessionPromptPayload,
} from "../types";
import { throwSessionRequestError } from "./session-api-errors";

export async function cancelQueuedSessionPrompt(
  workspaceId: string,
  id: string,
  queueEntryId: string,
  signal?: AbortSignal
): Promise<SessionPromptPayload> {
  const { data, error, response } = await apiClient.DELETE(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/prompt/queue/{queue_entry_id}",
    {
      params: {
        path: {
          workspace_id: workspaceId,
          session_id: id,
          queue_entry_id: queueEntryId,
        },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(
      response,
      error,
      `Failed to cancel queued prompt "${queueEntryId}" for session "${id}"`,
      id
    );
  }
  return requireResponseData(
    data,
    response,
    `Failed to cancel queued prompt "${queueEntryId}" for session "${id}"`
  ).prompt;
}

export async function fetchSessionInputs(
  workspaceId: string,
  id: string,
  signal?: AbortSignal
): Promise<SessionInputsResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/prompt/queue",
    {
      params: { path: { workspace_id: workspaceId, session_id: id } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(
      response,
      error,
      `Failed to fetch pending inputs for session "${id}"`,
      id
    );
  }
  return requireResponseData(data, response, `Failed to fetch pending inputs for session "${id}"`);
}

export async function replaceSessionInput(
  workspaceId: string,
  id: string,
  queueEntryId: string,
  params: ReplaceSessionInputRequest,
  signal?: AbortSignal
): Promise<SessionInputPayload> {
  const { data, error, response } = await apiClient.PUT(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/prompt/queue/{queue_entry_id}",
    {
      params: {
        path: { workspace_id: workspaceId, session_id: id, queue_entry_id: queueEntryId },
      },
      body: params,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(
      response,
      error,
      `Failed to replace pending input "${queueEntryId}" for session "${id}"`,
      id
    );
  }
  return requireResponseData(
    data,
    response,
    `Failed to replace pending input "${queueEntryId}" for session "${id}"`
  ).input;
}

export async function promoteSessionInputToSteer(
  workspaceId: string,
  id: string,
  queueEntryId: string,
  params: PromoteSessionInputRequest,
  signal?: AbortSignal
): Promise<SessionPromptPayload> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/prompt/queue/{queue_entry_id}/steer",
    {
      params: {
        path: { workspace_id: workspaceId, session_id: id, queue_entry_id: queueEntryId },
      },
      body: params,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(
      response,
      error,
      `Failed to steer pending input "${queueEntryId}" for session "${id}"`,
      id
    );
  }
  return requireResponseData(
    data,
    response,
    `Failed to steer pending input "${queueEntryId}" for session "${id}"`
  ).prompt;
}
