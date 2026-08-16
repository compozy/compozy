import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type { SessionAttachment } from "../types";

export class SessionAttachmentApiError extends Error {
  constructor(
    message: string,
    public readonly statusCode?: number
  ) {
    super(message);
    this.name = "SessionAttachmentApiError";
  }
}

function throwAttachmentError(response: Response, error: unknown, fallback: string): never {
  throw new SessionAttachmentApiError(
    defaultApiErrorMessage(fallback, response, error),
    response.status
  );
}

/**
 * Multipart upload. openapi-fetch can address the route, but the generated
 * body types the binary field as `string`, so the live File is sent through
 * a FormData `bodySerializer`.
 */
export async function uploadSessionAttachment(
  workspaceId: string,
  sessionId: string,
  file: File,
  signal?: AbortSignal
): Promise<SessionAttachment> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/attachments",
    {
      params: {
        path: { workspace_id: workspaceId, session_id: sessionId },
      },
      body: { file: file.name },
      bodySerializer: () => {
        const form = new FormData();
        form.append("file", file);
        return form;
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwAttachmentError(response, error, "Couldn't save the attachment");
  }
  try {
    return requireResponseData(data, response, "Couldn't save the attachment").attachment;
  } catch (error) {
    throw new SessionAttachmentApiError(
      error instanceof Error ? error.message : "Couldn't save the attachment",
      response.status
    );
  }
}

export async function deleteSessionAttachment(
  workspaceId: string,
  sessionId: string,
  attachmentId: string,
  signal?: AbortSignal
): Promise<void> {
  const { error, response } = await apiClient.DELETE(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/attachments/{attachment_id}",
    {
      params: {
        path: {
          workspace_id: workspaceId,
          session_id: sessionId,
          attachment_id: attachmentId,
        },
      },
      signal,
    }
  );
  if (response.status === 204) return;
  if (response.status === 404) return;
  if (apiRequestFailed(response, error)) {
    throwAttachmentError(response, error, "Couldn't remove the attachment");
  }
}
