import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type { ToolArtifactPage } from "../types";

const TOOL_ARTIFACT_URI_PATTERN = /^agh:\/\/tool-artifacts\/(art_[a-f0-9]{64})$/;

export class ToolArtifactApiError extends Error {
  constructor(
    message: string,
    public readonly statusCode?: number
  ) {
    super(message);
    this.name = "ToolArtifactApiError";
  }
}

function artifactIDFromURI(uri: string): string {
  const match = TOOL_ARTIFACT_URI_PATTERN.exec(uri);
  if (!match?.[1]) {
    throw new ToolArtifactApiError("Invalid retained result reference");
  }
  return match[1];
}

export async function fetchToolArtifactPage(
  workspaceId: string,
  artifactURI: string,
  offset: number,
  limit: number,
  signal?: AbortSignal
): Promise<ToolArtifactPage> {
  const artifactID = artifactIDFromURI(artifactURI);
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/tool-artifacts/{artifact_id}",
    {
      params: {
        path: { workspace_id: workspaceId, artifact_id: artifactID },
        query: { offset, limit },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    const message =
      response.status === 404
        ? "This retained result is no longer available"
        : defaultApiErrorMessage("Couldn't load the retained result", response, error);
    throw new ToolArtifactApiError(message, response.status);
  }
  return requireResponseData(data, response, "Couldn't load the retained result");
}
