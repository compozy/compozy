import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import { normalizeTranscriptMessages } from "../lib/message-schemas";
import type { NormalizedSessionTranscriptResponse, SessionTranscriptQuery } from "../types";
import { throwSessionRequestError } from "./session-api-errors";

export interface SessionStreamCursor {
  afterSequence?: number;
  epoch?: number;
  generation?: number;
  limit?: number;
}

export function buildSessionStreamUrl(
  workspaceId: string,
  id: string,
  cursor: SessionStreamCursor = {}
): string {
  const path = `/api/workspaces/${encodeURIComponent(workspaceId)}/sessions/${encodeURIComponent(id)}/stream`;
  const params = new URLSearchParams({ frames: "transcript" });
  if (cursor.afterSequence !== undefined && cursor.afterSequence > 0) {
    params.set("after_sequence", String(cursor.afterSequence));
  }
  if (cursor.epoch !== undefined) {
    params.set("epoch", String(cursor.epoch));
  }
  if (cursor.generation !== undefined) {
    params.set("generation", String(cursor.generation));
  }
  if (cursor.limit !== undefined) {
    params.set("limit", String(cursor.limit));
  }
  return `${path}?${params.toString()}`;
}

export async function fetchSessionTranscript(
  workspaceId: string,
  id: string,
  query: SessionTranscriptQuery = {},
  signal?: AbortSignal
): Promise<NormalizedSessionTranscriptResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/transcript",
    {
      params: {
        path: { workspace_id: workspaceId, session_id: id },
        query: Object.keys(query).length > 0 ? query : undefined,
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to fetch session transcript "${id}"`, id);
  }

  const payload = requireResponseData(data, response, `Failed to fetch session transcript "${id}"`);
  const messages = await normalizeTranscriptMessages(payload.entries.map(entry => entry.message));
  return {
    ...payload,
    entries: payload.entries.map((entry, index) => ({
      ...entry,
      message: messages[index]!,
    })),
  };
}
