import { useQuery } from "@tanstack/react-query";

import { automationSuggestionsSourceOptions } from "../lib/query-options";
import type { AutomationSuggestionStatus, AutomationSuggestionsSource } from "../types";

export function useAutomationSuggestions(
  source: AutomationSuggestionsSource,
  status: AutomationSuggestionStatus = "pending"
) {
  return useQuery(automationSuggestionsSourceOptions(source, status));
}
