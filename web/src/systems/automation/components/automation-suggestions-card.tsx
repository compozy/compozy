import { AlertCircle } from "lucide-react";
import type { ComponentProps, ReactNode } from "react";

import { Button, Pill, Section, Skeleton, Spinner } from "@agh/ui";

import { describeSchedule } from "../lib/automation-formatters";
import type { AutomationSuggestion } from "../types";

export type AutomationSuggestionPendingAction = "accept" | "dismiss";

export interface AutomationSuggestionsCardProps {
  actionErrors?: Partial<Record<string, string>>;
  errorMessage?: string | null;
  isLoading?: boolean;
  onAccept: (suggestionID: string) => void;
  onDismiss: (suggestionID: string) => void;
  onRetry?: () => void;
  pendingActions?: Partial<Record<string, AutomationSuggestionPendingAction>>;
  suggestions: AutomationSuggestion[];
}

function SuggestionsShell({ children, count }: { children: ReactNode; count?: number }) {
  return (
    <Section
      aria-label="Suggested jobs"
      bodyClassName="min-h-0 max-h-80 overflow-y-auto overscroll-contain border-t border-line"
      className="m-3 mb-0 shrink-0 gap-0 overflow-hidden rounded-lg border border-line bg-canvas-soft"
      count={count}
      data-testid="automation-suggestions-card"
      headClassName="px-4 py-3"
      label="Suggested jobs"
      note="Review each proposal before creating a workspace Job."
    >
      {children}
    </Section>
  );
}

function AutomationSuggestionsLoading() {
  return (
    <SuggestionsShell>
      <div
        aria-busy="true"
        aria-label="Loading suggested jobs"
        data-testid="automation-suggestions-loading"
      >
        {["first", "second"].map(row => (
          <div
            className="flex items-center justify-between gap-4 border-b border-line px-4 py-3 last:border-b-0"
            key={row}
          >
            <div className="flex min-w-0 flex-1 flex-col gap-2">
              <Skeleton className="h-4 w-40" />
              <Skeleton className="h-3 w-64 max-w-full" />
              <Skeleton className="h-3 w-full max-w-xl" />
            </div>
            <div className="flex shrink-0 gap-2">
              <Skeleton className="h-button-default w-28" />
              <Skeleton className="h-button-default w-20" />
            </div>
          </div>
        ))}
      </div>
    </SuggestionsShell>
  );
}

function AutomationSuggestionsError({
  errorMessage,
  onRetry,
}: {
  errorMessage: string;
  onRetry?: () => void;
}) {
  return (
    <SuggestionsShell>
      <div
        className="flex items-center justify-between gap-4 px-4 py-3"
        data-testid="automation-suggestions-error"
        role="alert"
      >
        <div className="flex min-w-0 items-start gap-2 text-small-body text-danger">
          <AlertCircle className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
          <span>{errorMessage}</span>
        </div>
        {onRetry ? (
          <Button onClick={onRetry} type="button" variant="neutral">
            Retry
          </Button>
        ) : null}
      </div>
    </SuggestionsShell>
  );
}

function AutomationSuggestionActionButton({
  children,
  isPending,
  ...props
}: ComponentProps<typeof Button> & { isPending: boolean }) {
  return (
    <Button {...props} aria-busy={isPending} disabled={isPending || props.disabled}>
      <span aria-hidden="true" className="inline-flex size-3 items-center justify-center">
        {isPending ? <Spinner className="size-3" /> : null}
      </span>
      {children}
    </Button>
  );
}

export function AutomationSuggestionsCard({
  actionErrors = {},
  errorMessage = null,
  isLoading = false,
  onAccept,
  onDismiss,
  onRetry,
  pendingActions = {},
  suggestions,
}: AutomationSuggestionsCardProps) {
  if (isLoading && suggestions.length === 0) {
    return <AutomationSuggestionsLoading />;
  }

  if (errorMessage && suggestions.length === 0) {
    return <AutomationSuggestionsError errorMessage={errorMessage} onRetry={onRetry} />;
  }

  if (suggestions.length === 0) {
    return null;
  }

  return (
    <SuggestionsShell count={suggestions.length}>
      <ul aria-label="Pending automation suggestions" className="divide-y divide-line">
        {suggestions.map(suggestion => {
          const pendingAction = pendingActions[suggestion.id];
          const actionError = actionErrors[suggestion.id];
          const payload = suggestion.payload;
          const schedule = describeSchedule(payload.schedule);

          return (
            <li
              aria-busy={Boolean(pendingAction)}
              className="flex flex-col gap-3 px-4 py-3 lg:flex-row lg:items-center lg:justify-between"
              data-testid={`automation-suggestion-${suggestion.id}`}
              key={suggestion.id}
            >
              <div className="min-w-0 flex-1">
                <div className="flex min-w-0 flex-wrap items-center gap-2">
                  <h3 className="truncate text-small-body font-medium text-fg-strong">
                    {payload.name}
                  </h3>
                  <Pill mono tone="neutral">
                    {schedule}
                  </Pill>
                  <Pill tone="neutral">{suggestion.source}</Pill>
                </div>
                <p className="mt-1 text-form-label text-muted">
                  Creates an {payload.enabled ? "enabled" : "disabled"} workspace Job for{" "}
                  <span className="font-medium text-fg">{payload.agent_name}</span>. Schedule:{" "}
                  {schedule}.
                </p>
                <p className="mt-1 break-words text-form-hint leading-relaxed text-subtle">
                  <span className="font-medium text-muted">Prompt:</span> {payload.prompt}
                </p>
                {actionError ? (
                  <p className="mt-2 text-form-label text-danger" role="alert">
                    {actionError}
                  </p>
                ) : null}
              </div>

              <div className="flex shrink-0 items-center gap-2">
                <AutomationSuggestionActionButton
                  className="min-w-28"
                  disabled={Boolean(pendingAction)}
                  isPending={pendingAction === "accept"}
                  onClick={() => onAccept(suggestion.id)}
                  type="button"
                  variant="neutral"
                >
                  Create job
                </AutomationSuggestionActionButton>
                <AutomationSuggestionActionButton
                  className="min-w-20"
                  disabled={Boolean(pendingAction)}
                  isPending={pendingAction === "dismiss"}
                  onClick={() => onDismiss(suggestion.id)}
                  type="button"
                  variant="ghost"
                >
                  Dismiss
                </AutomationSuggestionActionButton>
              </div>
            </li>
          );
        })}
      </ul>
    </SuggestionsShell>
  );
}
