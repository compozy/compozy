import { useState, type ReactNode } from "react";

import {
  useAcceptAutomationSuggestion,
  useDismissAutomationSuggestion,
} from "../hooks/use-automation-suggestion-actions";
import { useAutomationSuggestions } from "../hooks/use-automation-suggestions";
import {
  AutomationSuggestionsCard,
  type AutomationSuggestionPendingAction,
} from "./automation-suggestions-card";
import type { AutomationSuggestion, AutomationSuggestionsSource } from "../types";

const LOAD_ERROR = "Couldn't load automation suggestions. Retry the request.";
const ACCEPT_ERROR = "Couldn't create this job. Review the proposal and try again.";
const DISMISS_ERROR = "Couldn't dismiss this suggestion. Try again.";

function actionErrorMessage(error: unknown, fallback: string): string {
  if (!(error instanceof Error)) return fallback;
  const message = error.message.trim();
  return message === "" ? fallback : message;
}

export function AutomationSuggestionsPanel({
  className,
  projectBadge,
  source,
  timeZone,
}: {
  className?: string;
  /** All projects: each suggestion names its project. */
  projectBadge?: (suggestion: AutomationSuggestion) => ReactNode;
  source: AutomationSuggestionsSource;
  /** The global automation time zone the suggested schedules read in. */
  timeZone?: string;
}) {
  const suggestionsQuery = useAutomationSuggestions(source);
  const acceptSuggestion = useAcceptAutomationSuggestion();
  const dismissSuggestion = useDismissAutomationSuggestion();
  const suggestions = suggestionsQuery.data?.suggestions ?? [];
  const workspaceOf = (suggestionID: string) =>
    source.kind === "workspace"
      ? source.workspaceID
      : (suggestions.find(suggestion => suggestion.id === suggestionID)?.workspace_id ?? "");
  const [pendingActions, setPendingActions] = useState<
    Partial<Record<string, AutomationSuggestionPendingAction>>
  >({});
  const [actionErrors, setActionErrors] = useState<Partial<Record<string, string>>>({});

  async function runAction(
    suggestionID: string,
    action: AutomationSuggestionPendingAction
  ): Promise<void> {
    setPendingActions(current => ({ ...current, [suggestionID]: action }));
    setActionErrors(current => {
      const next = { ...current };
      delete next[suggestionID];
      return next;
    });

    try {
      const workspaceID = workspaceOf(suggestionID);
      if (action === "accept") {
        await acceptSuggestion.mutateAsync({ workspaceID, suggestionID });
      } else {
        await dismissSuggestion.mutateAsync({ workspaceID, suggestionID });
      }
    } catch (error) {
      const fallback = action === "accept" ? ACCEPT_ERROR : DISMISS_ERROR;
      setActionErrors(current => ({
        ...current,
        [suggestionID]: actionErrorMessage(error, fallback),
      }));
    } finally {
      setPendingActions(current => {
        const next = { ...current };
        delete next[suggestionID];
        return next;
      });
    }
  }

  return (
    <AutomationSuggestionsCard
      timeZone={timeZone}
      actionErrors={actionErrors}
      className={className}
      errorMessage={suggestionsQuery.isError ? LOAD_ERROR : null}
      isLoading={suggestionsQuery.isPending}
      onAccept={suggestionID => void runAction(suggestionID, "accept")}
      onDismiss={suggestionID => void runAction(suggestionID, "dismiss")}
      onRetry={() => void suggestionsQuery.refetch()}
      pendingActions={pendingActions}
      projectBadge={projectBadge}
      suggestions={suggestions}
    />
  );
}
