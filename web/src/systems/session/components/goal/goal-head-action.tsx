import { CheckCircle2, Pause, Play, X } from "lucide-react";
import type { ReactNode } from "react";

import { Button, Spinner } from "@compozy/ui";

import type { SessionGoalSnapshot } from "./goal-status-types";
import type { GoalControlAction } from "@/systems/loops";

export interface SessionGoalHeadActionProps {
  snapshot: SessionGoalSnapshot | null;
  pendingAction?: GoalControlAction;
  onPause?: () => void;
  onResume?: () => void;
  onApprove?: () => void;
  onClear?: () => void;
}

interface HeadAction {
  action: GoalControlAction;
  label: string;
  icon: ReactNode;
  variant: "primary" | "ghost";
  className?: string;
  onAct: () => void;
}

// State-gated, mutually exclusive: the head never shows more than one goal
// action. Approve outranks Resume outranks Pause; Clear appears only for a
// terminal snapshot.
function resolveHeadAction({
  snapshot,
  onPause,
  onResume,
  onApprove,
  onClear,
}: SessionGoalHeadActionProps): HeadAction | null {
  if (!snapshot) return null;
  if (snapshot.live && snapshot.run_status === "needs-approval" && onApprove) {
    return {
      action: "approve",
      label: "Approve goal",
      icon: <CheckCircle2 aria-hidden="true" className="size-3" />,
      variant: "primary",
      onAct: onApprove,
    };
  }
  if (
    snapshot.live &&
    (snapshot.status === "paused" || snapshot.run_status === "paused") &&
    onResume
  ) {
    return {
      action: "resume",
      label: "Resume goal",
      icon: <Play aria-hidden="true" className="size-3" />,
      variant: "ghost",
      onAct: onResume,
    };
  }
  if (snapshot.live && snapshot.status === "active" && onPause) {
    return {
      action: "pause",
      label: "Pause goal",
      icon: <Pause aria-hidden="true" className="size-3" />,
      variant: "ghost",
      onAct: onPause,
    };
  }
  if (!snapshot.live && onClear) {
    return {
      action: "clear",
      label: "Clear goal",
      icon: <X aria-hidden="true" className="size-3" />,
      variant: "ghost",
      className: "text-muted hover:bg-danger-tint hover:text-danger",
      onAct: onClear,
    };
  }
  return null;
}

/** The one state-gated goal action riding the session window head. */
export function SessionGoalHeadAction(props: SessionGoalHeadActionProps) {
  const resolved = resolveHeadAction(props);
  if (!resolved) return null;
  const pending = props.pendingAction === resolved.action;
  return (
    <Button
      type="button"
      size="sm"
      variant={resolved.variant}
      className={resolved.className}
      disabled={props.pendingAction !== undefined}
      onClick={resolved.onAct}
      data-testid={`goal-head-${resolved.action}`}
    >
      {pending ? <Spinner className="size-3" /> : resolved.icon}
      {resolved.label}
    </Button>
  );
}
