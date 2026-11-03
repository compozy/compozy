import { Alert, AlertMeta, AlertTitle, Spinner } from "@compozy/ui";

import type { SessionRuntimePayload } from "../types";

type SessionRuntimeRecoveryReason = NonNullable<
  NonNullable<SessionRuntimePayload["recovery"]>["reason"]
>;

export interface SessionRuntimeRecoveryNoticeProps {
  attempt?: number;
  maxAttempts?: number;
  reason?: SessionRuntimeRecoveryReason;
}

export function SessionRuntimeRecoveryNotice({
  attempt,
  maxAttempts,
  reason,
}: SessionRuntimeRecoveryNoticeProps) {
  const hasAttempt =
    typeof attempt === "number" &&
    Number.isInteger(attempt) &&
    attempt > 0 &&
    typeof maxAttempts === "number" &&
    Number.isInteger(maxAttempts) &&
    maxAttempts >= attempt;
  const waitingForSignIn = reason === "provider_auth";

  return (
    <Alert
      aria-live="polite"
      className="my-3 w-full"
      data-testid="session-runtime-recovery"
      role="status"
      variant="warning"
    >
      <Spinner aria-hidden="true" className="size-3.5" />
      <AlertTitle>
        {waitingForSignIn ? "Waiting for the provider sign-in…" : "Reconnecting to the agent…"}
      </AlertTitle>
      {hasAttempt ? (
        <AlertMeta data-testid="session-runtime-recovery-attempt">
          {waitingForSignIn ? "Retrying automatically · " : null}Attempt {attempt} of {maxAttempts}
        </AlertMeta>
      ) : null}
    </Alert>
  );
}
