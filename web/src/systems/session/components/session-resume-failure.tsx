import { RefreshCw, TriangleAlert, X } from "lucide-react";
import { useEffect, useEffectEvent } from "react";

import { Alert, AlertActions, AlertDescription, AlertTitle, Button, Spinner } from "@compozy/ui";

export interface SessionResumeFailureProps {
  sessionId: string;
  message: string;
  missingProvider: string | null;
  agentName?: string | null;
  isRetrying: boolean;
  onRetry: () => void;
  onDismiss: () => void;
  title?: string;
  retryLabel?: string;
  showDismiss?: boolean;
}

/**
 * The ONE banner the transcript budget allows — a session-level failure above
 * the transcript, composed from the danger `Alert`: a plain sentence and the
 * recovery actions. The session id and agent ride on data attributes (the id
 * is in the URL, and copying it belongs to the window overflow menu).
 */
export function SessionResumeFailure({
  sessionId,
  message,
  missingProvider,
  agentName,
  isRetrying,
  onRetry,
  onDismiss,
  title = "Couldn't reconnect to this session",
  retryLabel = "Try again",
  showDismiss = true,
}: SessionResumeFailureProps) {
  const normalizedMissingProvider = missingProvider?.trim() ?? "";
  const normalizedAgentName = agentName?.trim() ?? "";
  const hasProviderDetail = normalizedMissingProvider.length > 0;

  const handleEscape = useEffectEvent((event: KeyboardEvent) => {
    if (event.key !== "Escape" || event.defaultPrevented) return;
    onDismiss();
  });

  useEffect(() => {
    document.addEventListener("keydown", handleEscape);
    return () => document.removeEventListener("keydown", handleEscape);
  }, []);

  return (
    <Alert
      aria-live="assertive"
      className="my-3 w-full"
      data-agent={normalizedAgentName || undefined}
      data-session-id={sessionId}
      data-testid="session-resume-failure"
      role="alert"
      variant="danger"
    >
      <TriangleAlert aria-hidden="true" className="size-3.5" />
      <AlertTitle data-testid="session-resume-failure-title">{title}</AlertTitle>
      <AlertDescription data-testid="session-resume-failure-message">
        {hasProviderDetail
          ? `This session used ${normalizedMissingProvider}, which isn't set up in this project anymore. Add it back in Settings → Providers, then try again.`
          : message}
      </AlertDescription>
      <AlertActions>
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
          {retryLabel}
        </Button>
        {showDismiss ? (
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
        ) : null}
      </AlertActions>
    </Alert>
  );
}
