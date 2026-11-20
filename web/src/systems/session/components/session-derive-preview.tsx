import { CircleAlert, Copy, FileText } from "@compozy/ui/icons";

import { Button, FieldError, Skeleton, SkeletonRegion, SkeletonText } from "@compozy/ui";

import type { SessionDerivePreviewView } from "../lib/session-derive-view";

const TURN_IN_PROGRESS = "A turn is still in progress; it will not be carried over.";
const NATIVE_CLONE = "Uses the agent's own session clone.";
const MEASURE_FAILED = "Couldn't measure this session's context.";
const MEASURE_TIMED_OUT = "Measuring this session's context took too long.";
const MEASURE_SKIPPABLE = "The conversation still carries over.";

export interface SessionDerivePreviewLineProps {
  view: SessionDerivePreviewView;
  /**
   * Fork only: the daemon reported `native_fork_possible`. The line never
   * claims the capability on its own; absent means the replay seed runs.
   */
  nativeClone?: boolean;
  /**
   * A daemon fact that replaces the measurement (fork: the cut turn has not
   * settled). Rendered in the error tone in the line's place.
   */
  refusal?: string | null;
  /** Measures again after a failed or timed-out read. */
  onRetry?: () => void;
}

/**
 * The context line shared by the Continue and Fork dialogs: the daemon's own
 * measurement of what travels, stated before the primary is clicked. A failed
 * or timed-out read is an error, never "0 messages"; it offers Try again and
 * says the primary still works, since the measurement only informs the request.
 */
export function SessionDerivePreviewLine({
  view,
  nativeClone = false,
  refusal = null,
  onRetry,
}: SessionDerivePreviewLineProps) {
  if (refusal !== null) {
    return (
      <FieldError
        className="flex items-start gap-2 text-form-hint text-destructive"
        data-state="error"
        data-testid="session-derive-preview"
      >
        <CircleAlert aria-hidden="true" className="mt-0.5 size-3 shrink-0" />
        <span>{refusal}</span>
      </FieldError>
    );
  }
  if (view.state === "error") {
    return (
      <div
        className="flex items-start justify-between gap-3"
        data-reason={view.reason}
        data-state="error"
        data-testid="session-derive-preview"
      >
        <FieldError className="flex items-start gap-2 text-form-hint text-destructive">
          <CircleAlert aria-hidden="true" className="mt-0.5 size-3 shrink-0" />
          <span className="flex flex-col gap-0.5">
            <span>{view.reason === "timeout" ? MEASURE_TIMED_OUT : MEASURE_FAILED}</span>
            <span className="text-subtle">{MEASURE_SKIPPABLE}</span>
          </span>
        </FieldError>
        {onRetry ? (
          <Button
            data-testid="session-derive-preview-retry"
            onClick={onRetry}
            size="sm"
            type="button"
            variant="secondary"
          >
            Try again
          </Button>
        ) : null}
      </div>
    );
  }
  if (view.state === "measuring") {
    // The measurement's own line, in its place: the glyph well and one line of the sentence.
    return (
      <SkeletonRegion
        label="Measuring…"
        className="flex items-start gap-2 text-form-hint"
        data-state="measuring"
        data-testid="session-derive-preview"
      >
        <Skeleton className="mt-0.5 size-3 shrink-0 rounded-xs" />
        <SkeletonText className="w-56" />
      </SkeletonRegion>
    );
  }
  const Glyph = nativeClone ? Copy : FileText;
  return (
    <p
      className="flex items-start gap-2 text-form-hint text-subtle"
      data-native={nativeClone || undefined}
      data-state={view.state}
      data-testid="session-derive-preview"
    >
      <Glyph aria-hidden="true" className="mt-0.5 size-3 shrink-0 text-faint" />
      <span className="flex flex-col gap-0.5">
        <span>
          <b className="font-medium text-muted-foreground">{view.headline}</b> · {view.size}
        </span>
        {view.omitted ? (
          <span data-testid="session-derive-preview-omitted">{view.omitted}</span>
        ) : null}
        {view.turnInProgress ? (
          <span data-testid="session-derive-preview-turn">{TURN_IN_PROGRESS}</span>
        ) : null}
        {nativeClone ? (
          <span data-testid="session-derive-preview-native">{NATIVE_CLONE}</span>
        ) : null}
      </span>
    </p>
  );
}
