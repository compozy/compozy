import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  acceptAutomationSuggestion,
  dismissAutomationSuggestion,
} from "../adapters/automation-suggestions-api";
import { automationKeys } from "../lib/query-keys";

interface AutomationSuggestionActionParams {
  suggestionID: string;
}

export function useAcceptAutomationSuggestion(workspaceID: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ suggestionID }: AutomationSuggestionActionParams) =>
      acceptAutomationSuggestion(workspaceID, suggestionID),
    onSuccess: () =>
      Promise.all([
        queryClient.invalidateQueries({
          exact: true,
          queryKey: automationKeys.suggestionList(workspaceID, "pending"),
        }),
        queryClient.invalidateQueries({ queryKey: automationKeys.jobLists() }),
      ]),
  });
}

export function useDismissAutomationSuggestion(workspaceID: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ suggestionID }: AutomationSuggestionActionParams) =>
      dismissAutomationSuggestion(workspaceID, suggestionID),
    onSuccess: () =>
      queryClient.invalidateQueries({
        exact: true,
        queryKey: automationKeys.suggestionList(workspaceID, "pending"),
      }),
  });
}
