import { AlertTriangle, RefreshCw, X } from "lucide-react";

import {
  Alert,
  AlertActions,
  AlertDescription,
  AlertMeta,
  AlertTitle,
  Button,
  Eyebrow,
  MonoId,
  Pill,
  Spinner,
} from "@agh/ui";

export interface SessionResumeFailureProps {
  sessionId: string;
  message: string;
  missingProvider: string | null;
  agentName?: string | null;
  isRetrying: boolean;
  onRetry: () => void;
  onDismiss: () => void;
}

export function SessionResumeFailure({
  sessionId,
  message,
  missingProvider,
  agentName,
  isRetrying,
  onRetry,
  onDismiss,
}: SessionResumeFailureProps) {
  const normalizedMissingProvider = missingProvider?.trim() ?? "";
  const normalizedAgentName = agentName?.trim() ?? "";
  const hasProviderDetail = normalizedMissingProvider.length > 0;
  const hasAgentDetail = normalizedAgentName.length > 0;
  const title = hasProviderDetail ? "Attach failed: provider no longer available" : "Attach failed";

  return (
    <Alert
      aria-live="assertive"
      className="mt-2 w-full"
      data-testid="session-resume-failure"
      variant="danger"
    >
      <AlertTriangle aria-hidden="true" className="mt-0.5 size-3.5 shrink-0" />
      <AlertTitle data-testid="session-resume-failure-title">{title}</AlertTitle>
      <AlertDescription data-testid="session-resume-failure-message">
        {hasProviderDetail
          ? `This session was started with provider ${normalizedMissingProvider}, which is not visible in the current workspace configuration. Add the provider back to the workspace or update the agent defaults before retrying.`
          : message}
      </AlertDescription>
      <AlertMeta data-testid="session-resume-failure-meta">
        {hasProviderDetail ? (
          <Pill mono size="xs" data-testid="session-resume-failure-provider" tone="danger">
            {normalizedMissingProvider}
          </Pill>
        ) : null}
        <span className="inline-flex min-w-0 items-center gap-x-1.5">
          <Eyebrow>session</Eyebrow>
          <MonoId value={sessionId} />
        </span>
        {hasAgentDetail ? (
          <span className="inline-flex min-w-0 items-center gap-x-1.5">
            <Eyebrow>agent</Eyebrow>
            <MonoId value={normalizedAgentName} />
          </span>
        ) : null}
      </AlertMeta>
      <AlertActions>
        <Button
          data-testid="session-resume-failure-dismiss"
          onClick={onDismiss}
          size="sm"
          type="button"
          variant="ghost"
        >
          <X aria-hidden="true" className="size-3" />
          Dismiss
        </Button>
        <Button
          data-testid="session-resume-failure-retry"
          disabled={isRetrying}
          onClick={onRetry}
          size="sm"
          type="button"
          variant="neutral"
        >
          {isRetrying ? (
            <Spinner className="size-3" />
          ) : (
            <RefreshCw aria-hidden="true" className="size-3" />
          )}
          Retry attach
        </Button>
      </AlertActions>
    </Alert>
  );
}
