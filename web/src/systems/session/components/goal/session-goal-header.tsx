import { AlertTriangle } from "lucide-react";

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
      <div
        role="alert"
        data-testid="session-goal-header-error"
        className="mx-1 mt-2 mb-1 flex min-w-0 items-start gap-[9px] rounded-lg border border-danger/28 bg-danger/4 px-3 py-2.5 text-small-body leading-normal text-fg"
      >
        <AlertTriangle aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-danger" />
        <div className="min-w-0 flex-1">
          <b className="font-semibold text-fg-strong">Goal status unavailable</b>
          <p className="text-[12px] text-muted">{error.message}</p>
        </div>
      </div>
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
