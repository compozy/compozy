import { TriangleAlert } from "lucide-react";

import { Alert, AlertDescription, AlertTitle } from "@compozy/ui";

import type { GoalComposerAffordance, SessionGoalSnapshot } from "./goal-status-types";
import { SessionGoalStrip } from "./session-goal-strip";

interface SessionGoalHeaderProps {
  composerAffordance?: GoalComposerAffordance;
  error?: Error | null;
  onPrefillComposer?: (text: string) => void;
  snapshot: SessionGoalSnapshot | null;
}

/**
 * Pinned goal zone above the transcript scroller. Renders the one-line goal
 * strip, or — when the goal read fails — a session-level banner (28% danger
 * hairline on a 4% wash), never a full-bleed tinted bar. Lifecycle actions
 * live on the window head, not here.
 */
export function SessionGoalHeader({
  composerAffordance,
  error,
  onPrefillComposer,
  snapshot,
}: SessionGoalHeaderProps) {
  if (error) {
    return (
      <Alert
        aria-live="assertive"
        className="mx-1 mt-2 mb-1"
        data-testid="session-goal-header-error"
        role="alert"
        variant="danger"
      >
        <TriangleAlert aria-hidden="true" className="size-3.5" />
        <AlertTitle>Couldn't load the goal</AlertTitle>
        <AlertDescription>{error.message}</AlertDescription>
      </Alert>
    );
  }
  if (!snapshot) return null;

  return (
    <div className="pt-2" data-testid="session-goal-header">
      <SessionGoalStrip
        snapshot={snapshot}
        composerAffordance={composerAffordance}
        onPrefillComposer={onPrefillComposer}
      />
    </div>
  );
}
