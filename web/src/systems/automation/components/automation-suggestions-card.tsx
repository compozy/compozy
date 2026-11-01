import { AlertCircle, Clock3, RefreshCcw } from "@compozy/ui/icons";
import type { ComponentProps, ReactNode } from "react";

import { CatalogEmptyPanel } from "@/components/catalog-empty-state";
import {
  Button,
  CatalogEmptyDisclosureRow,
  Pill,
  Skeleton,
  SkeletonRegion,
  SkeletonText,
  Spinner,
} from "@compozy/ui";

import { describeFireLimit, formatPromptPreview } from "../lib/automation-formatters";
import { describeSchedule } from "../lib/automation-sentence";
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
  /** All projects: the project badge each suggestion leads its pill with. */
  projectBadge?: (suggestion: AutomationSuggestion) => ReactNode;
  suggestions: AutomationSuggestion[];
  /** The global automation time zone the schedule phrases read in. */
  timeZone?: string;
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
      aria-label="Suggested automations"
      className={className}
      count={count}
      data-testid="automation-suggestions-card"
      label="Suggested automations"
      note="Review each suggestion before creating it."
    >
      {children}
    </CatalogEmptyPanel>
  );
}

const SUGGESTION_SKELETON_IDS = ["suggestion-skeleton-1", "suggestion-skeleton-2"] as const;

/**
 * Two collapsed suggestion rows in the real disclosure grid: the 12px chevron column, the 28px
 * icon well, the title and description lines, and the two action buttons.
 */
function AutomationSuggestionsLoading({ className }: { className?: string }) {
  return (
    <SuggestionsShell className={className}>
      <SkeletonRegion
        data-testid="automation-suggestions-loading"
        label="Loading suggested automations"
      >
        <ul>
          {SUGGESTION_SKELETON_IDS.map(id => (
            <li
              aria-hidden="true"
              className="@container/catalog-row animate-skeleton-reveal border-t border-line-soft first:border-t-0"
              key={id}
            >
              <div className="grid grid-cols-1 items-center gap-3.5 px-4 @lg/catalog-row:grid-cols-[minmax(0,1fr)_auto]">
                <div className="grid grid-cols-[12px_28px_minmax(0,1fr)] items-start gap-2.5 py-3">
                  <span />
                  <Skeleton className="mt-px size-7" />
                  <span className="min-w-0">
                    <b className="block text-card-title font-medium tracking-row-title">
                      <SkeletonText className="w-40" />
                    </b>
                    <span className="mt-0.5 block text-small-body">
                      <SkeletonText className="w-64" />
                    </span>
                  </span>
                </div>
                <div className="flex shrink-0 flex-wrap items-center gap-1.5 pb-3 pl-16 @lg/catalog-row:pb-0 @lg/catalog-row:pl-0">
                  <Skeleton className="h-button-sm w-32" />
                  <Skeleton className="h-button-sm w-16" />
                </div>
              </div>
            </li>
          ))}
        </ul>
      </SkeletonRegion>
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
        <div className="flex min-w-0 items-start gap-2 text-small-body text-destructive">
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
  projectBadge,
  suggestions,
  timeZone,
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
          const schedule = describeSchedule(payload.schedule, { timeZone });
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
                    Create automation
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
                  <div className="rounded-lg bg-sunken px-3 py-2.5">
                    <p className="text-form-label leading-relaxed text-fg">{payload.prompt}</p>
                  </div>
                  <p className="font-mono text-mono-id text-subtle tabular-nums">
                    {suggestionFacts(suggestion, schedule, target)}
                  </p>
                </>
              }
              error={
                actionError ? (
                  <p className="text-form-label text-destructive" role="alert">
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
                <span className="flex min-w-0 items-center gap-2">
                  {projectBadge?.(suggestion)}
                  <Pill mono size="sm" tone="neutral">
                    {schedule}
                  </Pill>
                </span>
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
