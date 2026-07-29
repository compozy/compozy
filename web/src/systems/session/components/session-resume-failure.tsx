import { AlertTriangle, RefreshCw, X } from "lucide-react";
import { useEffect, useEffectEvent } from "react";

import { Button, Spinner } from "@compozy/ui";

export interface SessionResumeFailureProps {
  sessionId: string;
  message: string;
  missingProvider: string | null;
  agentName?: string | null;
  isRetrying: boolean;
  onRetry: () => void;
  onDismiss: () => void;
}

/**
 * The ONE banner the transcript budget allows — a session-level failure above
 * the transcript: 28% danger hairline on a 4% wash, plain body sentences (the
 * session id, provider, and agent read as text, never id pills), and the
 * retry/dismiss pair on the right.
 */
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
  const title = hasProviderDetail ? "Attach failed: provider no longer available" : "Attach failed";

  const handleEscape = useEffectEvent((event: KeyboardEvent) => {
    if (event.key !== "Escape") return;
    onDismiss();
  });

  useEffect(() => {
    document.addEventListener("keydown", handleEscape);
    return () => document.removeEventListener("keydown", handleEscape);
  }, []);

  return (
    <div
      aria-live="assertive"
      role="alert"
      data-testid="session-resume-failure"
      className="mt-3 mb-1 flex w-full min-w-0 items-start gap-[9px] rounded-lg border border-danger/28 bg-danger/4 px-3 py-2.5 text-small-body leading-normal text-fg"
    >
      <AlertTriangle aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-danger" />
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <b data-testid="session-resume-failure-title" className="font-semibold text-fg-strong">
          {title}
        </b>
        <p data-testid="session-resume-failure-message" className="text-[12px] text-muted">
          {hasProviderDetail
            ? `This session was started with provider ${normalizedMissingProvider}, which is not visible in the current workspace configuration. Add the provider back to the workspace or update the agent defaults before retrying.`
            : message}
        </p>
        <p className="font-mono text-badge text-subtle" data-testid="session-resume-failure-meta">
          session {sessionId}
          {normalizedAgentName ? ` · agent ${normalizedAgentName}` : ""}
        </p>
        <div className="mt-1 flex items-center gap-[7px]">
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
        </div>
      </div>
    </div>
  );
}
