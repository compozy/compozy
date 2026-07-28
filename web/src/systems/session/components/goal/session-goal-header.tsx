import type { GoalControlAction } from "@/systems/loops";
import { GoalStatusChip } from "./goal-status-chip";
import type { GoalComposerAffordance, SessionGoalSnapshot } from "./goal-status-types";

interface SessionGoalHeaderProps {
  composerAffordance?: GoalComposerAffordance;
  error?: Error | null;
  onApprove?: () => void;
  onClear?: () => void;
  onPause?: () => void;
  onPrefillComposer?: (text: string) => void;
  onResume?: () => void;
  pendingAction?: GoalControlAction;
  snapshot: SessionGoalSnapshot | null;
}

export function SessionGoalHeader({
  composerAffordance,
  error,
  onApprove,
  onClear,
  onPause,
  onPrefillComposer,
  onResume,
  pendingAction,
  snapshot,
}: SessionGoalHeaderProps) {
  if (error) {
    return (
      <div
        className="border-b border-line bg-danger-tint px-4 py-2 text-small-body text-danger"
        role="alert"
      >
        Goal status unavailable. {error.message}
      </div>
    );
  }
  if (!snapshot) return null;

  return (
    <div className="border-b border-line bg-canvas px-4 py-3" data-testid="session-goal-header">
      <GoalStatusChip
        snapshot={snapshot}
        composerAffordance={composerAffordance}
        pendingAction={pendingAction}
        onPause={onPause}
        onResume={onResume}
        onApprove={onApprove}
        onClear={onClear}
        onPrefillComposer={onPrefillComposer}
      />
    </div>
  );
}
