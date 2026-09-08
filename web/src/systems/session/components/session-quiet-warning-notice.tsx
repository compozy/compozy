import { Square, TriangleAlert } from "lucide-react";

import { Alert, AlertActions, AlertDescription, AlertTitle, Button, Spinner } from "@compozy/ui";

import { type SessionQuietWarning, sessionQuietWarningFacts } from "../lib/session-quiet-warning";

export interface SessionQuietWarningNoticeProps {
  /** The daemon's quiet episode, as read from `supervision.quiet_warning`. */
  warning: SessionQuietWarning;
  /** A session stop is on the wire or the daemon already reads `stopping`. */
  isStopping: boolean;
  /** The public session stop; omitted when the operator may not stop this session. */
  onStop?: () => void;
}

/**
 * The single inactivity warning, where the operator already is (US-014.EC-2).
 * It states what the daemon stated when it warned — quiet for `quiet_after`,
 * stopping after `stop_grace` — and offers the only action that exists: stop
 * now. There is no "keep alive": work is what keeps a session alive, so any
 * work signal clears this notice from the daemon side. Warning, not danger:
 * nothing failed. When automatic stop is off the notice says so instead of
 * promising a countdown the daemon will not run.
 */
export function SessionQuietWarningNotice({
  warning,
  isStopping,
  onStop,
}: SessionQuietWarningNoticeProps) {
  const facts = sessionQuietWarningFacts(warning);
  return (
    <Alert
      aria-live="polite"
      className="my-3 w-full"
      data-quiet-stop={facts.stopsIn === null ? "off" : "scheduled"}
      data-testid="session-quiet-warning"
      role="status"
      variant="warning"
    >
      <TriangleAlert aria-hidden="true" className="size-3.5" />
      <AlertTitle data-testid="session-quiet-warning-title">Quiet for {facts.quietFor}.</AlertTitle>
      <AlertDescription data-testid="session-quiet-warning-message">
        {facts.stopsIn === null
          ? "Automatic stop is off, so this session keeps waiting until the agent gets back to work or you stop it."
          : `This session stops in ${facts.stopsIn} unless the agent gets back to work.`}
      </AlertDescription>
      {onStop ? (
        <AlertActions>
          <Button
            aria-busy={isStopping}
            data-testid="session-quiet-warning-stop"
            disabled={isStopping}
            onClick={onStop}
            size="xs"
            type="button"
            variant="neutral"
          >
            {isStopping ? (
              <Spinner aria-hidden="true" className="size-3" />
            ) : (
              <Square aria-hidden="true" className="size-3" />
            )}
            Stop now
          </Button>
        </AlertActions>
      ) : null}
    </Alert>
  );
}
