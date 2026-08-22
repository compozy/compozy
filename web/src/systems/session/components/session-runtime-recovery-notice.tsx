import { Alert, AlertMeta, AlertTitle, Spinner } from "@compozy/ui";

export interface SessionRuntimeRecoveryNoticeProps {
  attempt?: number;
  maxAttempts?: number;
}

export function SessionRuntimeRecoveryNotice({
  attempt,
  maxAttempts,
}: SessionRuntimeRecoveryNoticeProps) {
  const hasAttempt =
    typeof attempt === "number" &&
    Number.isInteger(attempt) &&
    attempt > 0 &&
    typeof maxAttempts === "number" &&
    Number.isInteger(maxAttempts) &&
    maxAttempts >= attempt;

  return (
    <Alert
      aria-live="polite"
      className="mt-3 mb-1 w-full"
      data-testid="session-runtime-recovery"
      role="status"
      variant="warning"
    >
      <Spinner aria-hidden="true" className="size-3.5" />
      <AlertTitle>Recovering runtime</AlertTitle>
      {hasAttempt ? (
        <AlertMeta data-testid="session-runtime-recovery-attempt">
          Attempt {attempt} of {maxAttempts}
        </AlertMeta>
      ) : null}
    </Alert>
  );
}
