import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import { AutomationApiError } from "./automation-api";
import type {
  AutomationSuggestionAcceptanceResponse,
  AutomationSuggestionDismissalResponse,
  AutomationSuggestionsListResponse,
  AutomationSuggestionStatus,
} from "../types";

export async function listAutomationSuggestions(
  workspaceID: string,
  status: AutomationSuggestionStatus,
  signal?: AbortSignal
): Promise<AutomationSuggestionsListResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/automation/suggestions",
    {
      params: {
        path: { workspace_id: workspaceID },
        query: { status },
      },
      signal,
    }
  );

  if (apiRequestFailed(response, error)) {
    throw new AutomationApiError(
      defaultApiErrorMessage("Failed to fetch automation suggestions", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to fetch automation suggestions");
}

export async function acceptAutomationSuggestion(
  workspaceID: string,
  suggestionID: string,
  signal?: AbortSignal
): Promise<AutomationSuggestionAcceptanceResponse> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/automation/suggestions/{suggestion_id}/accept",
    {
      params: { path: { workspace_id: workspaceID, suggestion_id: suggestionID } },
      signal,
    }
  );

  if (apiRequestFailed(response, error)) {
    throw new AutomationApiError(
      defaultApiErrorMessage("Failed to create job from automation suggestion", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to create job from automation suggestion");
}

export async function dismissAutomationSuggestion(
  workspaceID: string,
  suggestionID: string,
  signal?: AbortSignal
): Promise<AutomationSuggestionDismissalResponse> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/automation/suggestions/{suggestion_id}/dismiss",
    {
      params: { path: { workspace_id: workspaceID, suggestion_id: suggestionID } },
      signal,
    }
  );

  if (apiRequestFailed(response, error)) {
    throw new AutomationApiError(
      defaultApiErrorMessage("Failed to dismiss automation suggestion", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to dismiss automation suggestion");
}
