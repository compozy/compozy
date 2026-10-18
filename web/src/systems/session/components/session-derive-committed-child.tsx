import { Button, FieldError, Spinner } from "@compozy/ui";

import type { SessionDeriveCommittedChildModel } from "../hooks/use-session-derive-committed-child";

const COMMITTED_NOTE = "The new session was already created.";

export interface SessionDeriveSubmitOutcomeProps {
  /** The latest derive refusal, shown verbatim. */
  error: string | null;
  model: SessionDeriveCommittedChildModel;
  /** The dialog's own test-id prefix (`session-continue`, `session-fork`). */
  testIdPrefix: string;
}

// The dialog body scrolls inside the session window, so a refusal can land below
// the fold after the primary re-enables. Bring it into view once it appears.
function revealOutcome(node: HTMLDivElement | null) {
  node?.scrollIntoView?.({ block: "nearest" });
}

/**
 * The refusal of a continue or fork. Once a refusal came after the child was
 * created, the dialog keeps offering to open that session, the next safe action
 * instead of a retry, through later edits and refusals.
 */
export function SessionDeriveSubmitOutcome({
  error,
  model,
  testIdPrefix,
}: SessionDeriveSubmitOutcomeProps) {
  if (!error && !model.childSessionId) return null;
  return (
    <div
      className="flex flex-col gap-4"
      data-testid={`${testIdPrefix}-submit-outcome`}
      ref={revealOutcome}
    >
      {error ? <FieldError data-testid={`${testIdPrefix}-submit-error`}>{error}</FieldError> : null}
      {model.childSessionId ? (
        <div
          className="flex flex-col gap-2 text-form-hint text-subtle"
          data-testid={`${testIdPrefix}-committed-child`}
        >
          <div className="flex items-center justify-between gap-3">
            <span>{COMMITTED_NOTE}</span>
            <Button
              data-testid={`${testIdPrefix}-open-committed-child`}
              disabled={model.isOpening}
              onClick={() => void model.open()}
              size="sm"
              type="button"
              variant="secondary"
            >
              {model.isOpening ? <Spinner aria-hidden="true" className="size-3" /> : null}
              Open new session
            </Button>
          </div>
          {model.openError ? <FieldError>{model.openError}</FieldError> : null}
        </div>
      ) : null}
    </div>
  );
}
