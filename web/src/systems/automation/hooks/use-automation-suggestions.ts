import { useQuery } from "@tanstack/react-query";

import { automationSuggestionsListOptions } from "../lib/query-options";
import type { AutomationSuggestionStatus } from "../types";

export function useAutomationSuggestions(
  workspaceID: string,
  status: AutomationSuggestionStatus = "pending"
) {
  return useQuery(automationSuggestionsListOptions(workspaceID, status));
}
