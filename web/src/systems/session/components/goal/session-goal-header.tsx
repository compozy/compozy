import { SessionDangerBanner } from "../session-danger-banner";
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
      <SessionDangerBanner
        data-testid="session-goal-header-error"
        className="mx-1 mt-2 mb-1"
        title="Goal status unavailable"
      >
        <p className="text-transcript-body text-muted">{error.message}</p>
      </SessionDangerBanner>
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
