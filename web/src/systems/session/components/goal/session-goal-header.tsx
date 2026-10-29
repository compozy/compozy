import { TriangleAlert } from "lucide-react";

import { Alert, AlertAction, AlertDescription, AlertTitle, Button } from "@compozy/ui";

import type { GoalComposerAffordance, SessionGoalSnapshot } from "./goal-status-types";
import { SessionGoalStrip } from "./session-goal-strip";
import type { SessionGoalControlsProps } from "./session-goal-controls";

interface SessionGoalHeaderProps extends SessionGoalControlsProps {
  composerAffordance?: GoalComposerAffordance;
  error?: Error | null;
  actionError?: Error | null;
  refreshing?: boolean;
  onRefresh?: () => void;
  onPrefillComposer?: (text: string) => void;
  snapshot: SessionGoalSnapshot | null;
}

/** Keeps Goal state, controls, and their feedback together above the transcript. */
export function SessionGoalHeader({
  composerAffordance,
  error,
  actionError,
  refreshing,
  onRefresh,
  onPrefillComposer,
  snapshot,
  ...controls
}: SessionGoalHeaderProps) {
  if (!snapshot && !error && !actionError) return null;
  return (
    <div className="pt-2" data-testid="session-goal-header">
      {snapshot ? (
        <SessionGoalStrip
          snapshot={snapshot}
          composerAffordance={composerAffordance}
          onPrefillComposer={onPrefillComposer}
          {...controls}
        />
      ) : null}
      {error ? (
        <Alert
          aria-live="assertive"
          className="mx-1 mt-2 mb-1"
          data-testid="session-goal-header-error"
          role="alert"
          variant="danger"
        >
          <TriangleAlert aria-hidden="true" className="size-3.5" />
          <AlertTitle>
            {snapshot ? "Couldn't refresh the goal" : "Couldn't load the goal"}
          </AlertTitle>
          <AlertDescription>
            {snapshot ? "Showing the last confirmed state. " : ""}
            {error.message}
          </AlertDescription>
          {onRefresh ? (
            <AlertAction>
              <Button variant="ghost" size="sm" onClick={onRefresh} disabled={refreshing}>
                {refreshing ? "Refreshing…" : "Retry"}
              </Button>
            </AlertAction>
          ) : null}
        </Alert>
      ) : null}
      {actionError ? (
        <Alert
          variant="danger"
          role="alert"
          className="mt-2"
          data-testid="session-goal-control-error"
        >
          <TriangleAlert aria-hidden="true" className="size-3.5" />
          <AlertTitle>Goal control failed</AlertTitle>
          <AlertDescription>{actionError.message}</AlertDescription>
        </Alert>
      ) : null}
    </div>
  );
}
