import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  acceptAutomationSuggestion,
  dismissAutomationSuggestion,
} from "../adapters/automation-suggestions-api";
import { automationKeys } from "../lib/query-keys";

interface AutomationSuggestionActionParams {
  /** The suggestion's own project; the aggregate lists several. */
  workspaceID: string;
  suggestionID: string;
}

/** A resolved suggestion leaves its project's pending list and every All projects list. */
function invalidateSuggestionLists(
  queryClient: ReturnType<typeof useQueryClient>,
  workspaceID: string
) {
  return Promise.all([
    queryClient.invalidateQueries({
      exact: true,
      queryKey: automationKeys.suggestionList(workspaceID, "pending"),
    }),
    queryClient.invalidateQueries({ queryKey: automationKeys.suggestionAggregates() }),
  ]);
}

export function useAcceptAutomationSuggestion() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ workspaceID, suggestionID }: AutomationSuggestionActionParams) =>
      acceptAutomationSuggestion(workspaceID, suggestionID),
    onSuccess: (_data, { workspaceID }) =>
      Promise.all([
        invalidateSuggestionLists(queryClient, workspaceID),
        queryClient.invalidateQueries({ queryKey: automationKeys.jobLists() }),
      ]),
  });
}

export function useDismissAutomationSuggestion() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ workspaceID, suggestionID }: AutomationSuggestionActionParams) =>
      dismissAutomationSuggestion(workspaceID, suggestionID),
    onSuccess: (_data, { workspaceID }) => invalidateSuggestionLists(queryClient, workspaceID),
  });
}
