import { AlertCircle, Clock3, RefreshCcw } from "lucide-react";
import type { ComponentProps, ReactNode } from "react";

import { CatalogEmptyPanel } from "@/components/catalog-empty-state";
import { Button, CatalogEmptyDisclosureRow, Pill, Skeleton, Spinner } from "@compozy/ui";

import {
  describeFireLimit,
  describeSchedule,
  formatPromptPreview,
} from "../lib/automation-formatters";
import { projectAutomationTarget } from "../lib/automation-target";
import type { AutomationSuggestion } from "../types";

export type AutomationSuggestionPendingAction = "accept" | "dismiss";

export interface AutomationSuggestionsCardProps {
  actionErrors?: Partial<Record<string, string>>;
  className?: string;
  errorMessage?: string | null;
  isLoading?: boolean;
  onAccept: (suggestionID: string) => void;
  onDismiss: (suggestionID: string) => void;
  onRetry?: () => void;
  pendingActions?: Partial<Record<string, AutomationSuggestionPendingAction>>;
  suggestions: AutomationSuggestion[];
}

function SuggestionsShell({
  children,
  className,
  count,
}: {
  children: ReactNode;
  className?: string;
  count?: number;
}) {
  return (
    <CatalogEmptyPanel
      aria-label="Suggested jobs"
      className={className}
      count={count}
      data-testid="automation-suggestions-card"
      label="Suggested jobs"
      note="Review each proposal before creating a workspace Job."
    >
      {children}
    </CatalogEmptyPanel>
  );
}

function AutomationSuggestionsLoading({ className }: { className?: string }) {
  return (
    <SuggestionsShell className={className}>
      <div
        aria-busy="true"
        aria-label="Loading suggested jobs"
        data-testid="automation-suggestions-loading"
      >
        {["first", "second"].map(row => (
          <div
            className="flex items-center justify-between gap-4 border-t border-line-soft px-4 py-3 first:border-t-0"
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
  className,
  errorMessage,
  onRetry,
}: {
  className?: string;
  errorMessage: string;
  onRetry?: () => void;
}) {
  return (
    <SuggestionsShell className={className}>
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

function suggestionFacts(
  suggestion: AutomationSuggestion,
  schedule: string,
  target: ReturnType<typeof projectAutomationTarget>
): string {
  const payload = suggestion.payload;
  const targetLabel =
    target.kind === "loop"
      ? `loop ${target.loopName || "not selected"}`
      : `agent ${target.agentName || "unassigned"}`;
  return [
    schedule,
    targetLabel,
    describeFireLimit(payload.fire_limit),
    payload.enabled ? "starts enabled" : "starts disabled",
  ].join(" · ");
}

export function AutomationSuggestionsCard({
  actionErrors = {},
  className,
  errorMessage = null,
  isLoading = false,
  onAccept,
  onDismiss,
  onRetry,
  pendingActions = {},
  suggestions,
}: AutomationSuggestionsCardProps) {
  if (isLoading && suggestions.length === 0) {
    return <AutomationSuggestionsLoading className={className} />;
  }

  if (errorMessage && suggestions.length === 0) {
    return (
      <AutomationSuggestionsError
        className={className}
        errorMessage={errorMessage}
        onRetry={onRetry}
      />
    );
  }

  if (suggestions.length === 0) {
    return null;
  }

  return (
    <SuggestionsShell className={className} count={suggestions.length}>
      <ul aria-label="Pending automation suggestions">
        {suggestions.map(suggestion => {
          const pendingAction = pendingActions[suggestion.id];
          const actionError = actionErrors[suggestion.id];
          const payload = suggestion.payload;
          const schedule = describeSchedule(payload.schedule);
          const target = projectAutomationTarget(payload);

          return (
            <CatalogEmptyDisclosureRow
              busy={Boolean(pendingAction)}
              actions={
                <>
                  <AutomationSuggestionActionButton
                    disabled={Boolean(pendingAction)}
                    isPending={pendingAction === "accept"}
                    onClick={() => onAccept(suggestion.id)}
                    size="sm"
                    type="button"
                    variant="neutral"
                  >
                    Create job
                  </AutomationSuggestionActionButton>
                  <AutomationSuggestionActionButton
                    disabled={Boolean(pendingAction)}
                    isPending={pendingAction === "dismiss"}
                    onClick={() => onDismiss(suggestion.id)}
                    size="sm"
                    type="button"
                    variant="ghost"
                  >
                    Dismiss
                  </AutomationSuggestionActionButton>
                </>
              }
              description={formatPromptPreview(payload.prompt)}
              detail={
                <>
                  <div className="rounded-md border border-line-soft bg-input-fill px-3 py-2.5">
                    <p className="text-form-label leading-relaxed text-fg">{payload.prompt}</p>
                  </div>
                  <p className="font-mono text-mono-id text-subtle tabular-nums">
                    {suggestionFacts(suggestion, schedule, target)}
                  </p>
                </>
              }
              error={
                actionError ? (
                  <p className="text-form-label text-danger" role="alert">
                    {actionError}
                  </p>
                ) : null
              }
              icon={
                target.kind === "loop" ? (
                  <RefreshCcw className="size-3.5" />
                ) : (
                  <Clock3 className="size-3.5" />
                )
              }
              data-testid={`automation-suggestion-${suggestion.id}`}
              key={suggestion.id}
              pill={
                <Pill mono size="sm" tone="neutral">
                  {schedule}
                </Pill>
              }
              title={payload.name}
              tone="neutral"
            />
          );
        })}
      </ul>
    </SuggestionsShell>
  );
}
