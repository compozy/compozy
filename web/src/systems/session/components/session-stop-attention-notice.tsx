import { RefreshCw, TriangleAlert } from "lucide-react";

import {
  Alert,
  AlertActions,
  AlertDescription,
  AlertMeta,
  AlertTitle,
  Button,
  Spinner,
} from "@compozy/ui";

import { STOP_VERIFICATION_FAILED_ATTENTION } from "../lib/session-stop-attention";

export interface SessionStopAttentionNoticeProps {
  /** A retry of the stop is on the wire or still landing. */
  isRetrying: boolean;
  /** The session stop action itself; omitted when the operator may not stop this session. */
  onRetry?: () => void;
}

/**
 * The daemon ran the whole stop ladder and still could not verify the agent
 * process is gone, so the session truthfully stays `stopping` and asks for
 * attention (US-009.AC-3, ADR-004 invariant 3). Warning, not danger: nothing
 * in the operator's work failed. The only action is the one that exists — the
 * same session stop, again — and the notice stays until the daemon itself
 * reads `stopped`, never on acceptance, escalation, or a timer.
 */
export function SessionStopAttentionNotice({
  isRetrying,
  onRetry,
}: SessionStopAttentionNoticeProps) {
  return (
    <Alert
      aria-live="polite"
      className="my-3 w-full"
      data-attention={STOP_VERIFICATION_FAILED_ATTENTION}
      data-testid="session-stop-attention"
      role="alert"
      variant="warning"
    >
      <TriangleAlert aria-hidden="true" className="size-3.5" />
      <AlertTitle data-testid="session-stop-attention-title">
        Couldn&rsquo;t confirm the agent stopped.
      </AlertTitle>
      <AlertDescription data-testid="session-stop-attention-message">
        The stop ran all the way to a forced kill, and the runtime still couldn&rsquo;t confirm the
        process is gone. The session stays &ldquo;stopping&rdquo; until it can.
      </AlertDescription>
      <AlertMeta data-testid="session-stop-attention-meta">
        <span>needs attention</span>
        <span className="font-mono">{STOP_VERIFICATION_FAILED_ATTENTION}</span>
      </AlertMeta>
      {onRetry ? (
        <AlertActions>
          <Button
            aria-busy={isRetrying}
            data-testid="session-stop-attention-retry"
            disabled={isRetrying}
            onClick={onRetry}
            size="sm"
            type="button"
            variant="neutral"
          >
            {isRetrying ? (
              <Spinner aria-hidden="true" className="size-3" />
            ) : (
              <RefreshCw aria-hidden="true" className="size-3" />
            )}
            Retry stop
          </Button>
        </AlertActions>
      ) : null}
    </Alert>
  );
}
