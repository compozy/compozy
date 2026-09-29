import { Button, FieldError, Spinner } from "@compozy/ui";

import type { SessionDeriveCommittedChildModel } from "../hooks/use-session-derive-committed-child";

const COMMITTED_NOTE = "The new session was already created.";

export interface SessionDeriveCommittedChildProps {
  model: SessionDeriveCommittedChildModel;
  /** The dialog's own test-id prefix (`session-continue`, `session-fork`). */
  testIdPrefix: string;
}

/**
 * Shown under a derive refusal that came after the child was created: the
 * session exists, so the next safe action is to open it rather than retry.
 */
export function SessionDeriveCommittedChild({
  model,
  testIdPrefix,
}: SessionDeriveCommittedChildProps) {
  if (!model.childSessionId) return null;
  return (
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
          variant="outline"
        >
          {model.isOpening ? <Spinner aria-hidden="true" className="size-3" /> : null}
          Open new session
        </Button>
      </div>
      {model.openError ? <FieldError>{model.openError}</FieldError> : null}
    </div>
  );
}
