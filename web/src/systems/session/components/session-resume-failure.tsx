import { RefreshCw, X } from "lucide-react";
import { useEffect, useEffectEvent } from "react";

import { Button, Spinner } from "@compozy/ui";

import { SessionDangerBanner } from "./session-danger-banner";

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
 * the transcript: 28% danger hairline on a 4% wash, plain body sentences (the
 * session id, provider, and agent read as text, never id pills), and the
 * recovery actions on the right.
 */
export function SessionResumeFailure({
  sessionId,
  message,
  missingProvider,
  agentName,
  isRetrying,
  onRetry,
  onDismiss,
  title,
  retryLabel = "Retry attach",
  showDismiss = true,
}: SessionResumeFailureProps) {
  const normalizedMissingProvider = missingProvider?.trim() ?? "";
  const normalizedAgentName = agentName?.trim() ?? "";
  const hasProviderDetail = normalizedMissingProvider.length > 0;
  const resolvedTitle =
    title ?? (hasProviderDetail ? "Attach failed: provider no longer available" : "Attach failed");

  const handleEscape = useEffectEvent((event: KeyboardEvent) => {
    if (event.key !== "Escape" || event.defaultPrevented) return;
    onDismiss();
  });

  useEffect(() => {
    document.addEventListener("keydown", handleEscape);
    return () => document.removeEventListener("keydown", handleEscape);
  }, []);

  return (
    <SessionDangerBanner
      data-testid="session-resume-failure"
      className="my-3 w-full"
      title={<span data-testid="session-resume-failure-title">{resolvedTitle}</span>}
    >
      <p data-testid="session-resume-failure-message" className="text-transcript-body text-muted">
        {hasProviderDetail
          ? `This session was started with provider ${normalizedMissingProvider}, which is not visible in the current workspace configuration. Add the provider back to the workspace or update the agent defaults before retrying.`
          : message}
      </p>
      <p className="font-mono text-badge text-subtle" data-testid="session-resume-failure-meta">
        session {sessionId}
        {normalizedAgentName ? ` · agent ${normalizedAgentName}` : ""}
      </p>
      <div className="mt-1 flex items-center gap-transcript-inline-gap">
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
      </div>
    </SessionDangerBanner>
  );
}
