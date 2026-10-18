import { CircleAlert, Copy, FileText } from "lucide-react";

import { FieldError, Spinner } from "@compozy/ui";

import type { SessionDerivePreviewView } from "../lib/session-derive-view";

const TURN_IN_PROGRESS = "A turn is still in progress; it will not be carried over.";
const NATIVE_CLONE = "Uses the agent's own session clone.";

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
}

/**
 * The context line shared by the Continue and Fork dialogs: the daemon's own
 * measurement of what travels, stated before the primary is clicked. A failed
 * read is an error, never "0 messages".
 */
export function SessionDerivePreviewLine({
  view,
  nativeClone = false,
  refusal = null,
}: SessionDerivePreviewLineProps) {
  if (refusal !== null || view.state === "error") {
    return (
      <FieldError
        className="flex items-start gap-2 text-form-hint text-danger"
        data-state="error"
        data-testid="session-derive-preview"
      >
        <CircleAlert aria-hidden="true" className="mt-0.5 size-3 shrink-0" />
        <span>{refusal ?? "Couldn't measure this session's context."}</span>
      </FieldError>
    );
  }
  if (view.state === "measuring") {
    return (
      <p
        aria-live="polite"
        className="flex items-center gap-2 text-form-hint text-subtle"
        data-state="measuring"
        data-testid="session-derive-preview"
        role="status"
      >
        <Spinner aria-hidden="true" className="size-3 shrink-0" />
        <span>Measuring…</span>
      </p>
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
          <b className="font-medium text-muted">{view.headline}</b> · {view.size}
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
