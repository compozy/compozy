import { useId, useState } from "react";
import { CheckCircle2, Pause, Play, Square, X } from "lucide-react";

import {
  Button,
  ConfirmDialog,
  Spinner,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";

import type { SessionGoalSnapshot } from "./goal-status-types";
import type { GoalControlAction } from "@/systems/loops";

export interface SessionGoalControlsProps {
  snapshot: SessionGoalSnapshot | null;
  pendingAction?: GoalControlAction;
  continuationDisabledReason?: string;
  onPause?: () => void;
  onResume?: () => void;
  onApprove?: () => void;
  onStop?: () => void;
  onClear?: () => void;
}

function primaryAction({
  snapshot,
  onPause,
  onResume,
  onApprove,
  onClear,
}: SessionGoalControlsProps) {
  if (!snapshot) return null;
  if (!snapshot.live) {
    return onClear
      ? {
          action: "clear" as const,
          label: "Clear goal",
          pending: "Clearing…",
          Icon: X,
          onAct: onClear,
          hint: "Hide this goal. Its run history is kept.",
        }
      : null;
  }
  if (snapshot.run_status === "needs-approval") {
    return onApprove
      ? {
          action: "approve" as const,
          label: "Approve goal",
          pending: "Approving…",
          Icon: CheckCircle2,
          onAct: onApprove,
          hint: "Approve the requested continuation of this goal.",
        }
      : null;
  }
  if (snapshot.status === "paused" || snapshot.run_status === "paused") {
    return onResume
      ? {
          action: "resume" as const,
          label: "Resume goal",
          pending: "Resuming…",
          Icon: Play,
          onAct: onResume,
          hint: "Continue this goal from its saved progress.",
        }
      : null;
  }
  if (snapshot.pause_requested && onPause)
    return {
      action: "pause" as const,
      label: "Pausing…",
      pending: "Pausing…",
      Icon: Pause,
      onAct: onPause,
      hint: "The current goal turn is being interrupted. No further goal turns will start.",
    };
  return snapshot.status === "active" && onPause
    ? {
        action: "pause" as const,
        label: "Pause goal",
        pending: "Pausing…",
        Icon: Pause,
        onAct: onPause,
        hint: "Interrupt the current goal turn. No further goal turns run until you resume.",
      }
    : null;
}

/** Goal controls share their state and objective; stopping retains the run until explicitly cleared. */
export function SessionGoalControls(props: SessionGoalControlsProps) {
  const [stopRunId, setStopRunId] = useState<string | null>(null);
  const [tooltipOpen, setTooltipOpen] = useState(false);
  const tooltipId = useId();
  const primary = primaryAction(props);
  const stoppable = props.snapshot?.live === true && props.onStop !== undefined;
  if (!primary && !stoppable) return null;
  const pending = props.pendingAction !== undefined;
  const disabledReason =
    primary?.action === "resume" || primary?.action === "approve"
      ? props.continuationDisabledReason
      : undefined;
  return (
    <div className="flex shrink-0 items-center gap-1" aria-label="Goal controls" role="group">
      {primary ? (
        <Tooltip open={tooltipOpen} onOpenChange={setTooltipOpen}>
          <TooltipTrigger
            render={
              <Button
                type="button"
                size="sm"
                variant={primary.action === "clear" ? "quiet" : "secondary"}
                aria-disabled={disabledReason ? true : undefined}
                aria-describedby={tooltipOpen ? tooltipId : undefined}
                className="aria-disabled:text-subtle aria-disabled:hover:bg-surface-2"
                disabled={
                  pending || (primary.action === "pause" && props.snapshot?.pause_requested)
                }
                onClick={() => {
                  if (!disabledReason) primary.onAct();
                }}
                data-testid={`goal-control-${primary.action}`}
              />
            }
          >
            {props.pendingAction === primary.action ? (
              <Spinner className="size-3" />
            ) : (
              <primary.Icon aria-hidden="true" className="size-3" />
            )}
            {props.pendingAction === primary.action ? primary.pending : primary.label}
          </TooltipTrigger>
          <TooltipContent role="tooltip" id={tooltipId}>
            {disabledReason ?? primary.hint}
          </TooltipContent>
        </Tooltip>
      ) : null}
      {stoppable ? (
        <Button
          type="button"
          size="sm"
          variant="quiet"
          disabled={pending}
          onClick={() => setStopRunId(props.snapshot?.run_id ?? null)}
          data-testid="goal-control-stop"
        >
          {props.pendingAction === "stop" ? (
            <Spinner className="size-3" />
          ) : (
            <Square aria-hidden="true" className="size-3" />
          )}
          {props.pendingAction === "stop" ? "Stopping…" : "Stop goal"}
        </Button>
      ) : null}
      <ConfirmDialog
        open={stopRunId !== null && stopRunId === props.snapshot?.run_id && stoppable}
        onOpenChange={open => {
          if (!open) setStopRunId(null);
        }}
        title="Stop this goal?"
        description="This interrupts the current goal turn and ends the goal. You can start a new goal later; this one cannot be resumed."
        note="The session and run history are kept. Use Pause goal if you want to continue later."
        confirmLabel="Stop goal"
        cancelLabel="Keep goal"
        icon={Square}
        onConfirm={() => {
          setStopRunId(null);
          props.onStop?.();
        }}
      />
    </div>
  );
}
