import { CircleAlert, FileText } from "lucide-react";

import { FieldError, Spinner } from "@compozy/ui";

import type { SessionDerivePreviewView } from "../lib/session-derive-view";

const TURN_IN_PROGRESS = "A turn is still in progress; it will not be carried over.";

export interface SessionDerivePreviewLineProps {
  view: SessionDerivePreviewView;
}

/**
 * The context line shared by the Continue and Fork dialogs: the daemon's own
 * measurement of what travels, stated before the primary is clicked. A failed
 * read is an error, never "0 messages".
 */
export function SessionDerivePreviewLine({ view }: SessionDerivePreviewLineProps) {
  if (view.state === "error") {
    return (
      <FieldError
        className="flex items-start gap-2 text-form-hint text-danger"
        data-state="error"
        data-testid="session-derive-preview"
      >
        <CircleAlert aria-hidden="true" className="mt-0.5 size-3 shrink-0" />
        <span>Couldn't measure this session's context.</span>
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
  return (
    <p
      className="flex items-start gap-2 text-form-hint text-subtle"
      data-state={view.state}
      data-testid="session-derive-preview"
    >
      <FileText aria-hidden="true" className="mt-0.5 size-3 shrink-0 text-faint" />
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
      </span>
    </p>
  );
}
