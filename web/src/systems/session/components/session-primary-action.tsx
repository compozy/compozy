import { useState } from "react";
import {
  Button,
  ConfirmDialog,
  Spinner,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";
import { RotateCcw, Square } from "@compozy/ui/icons";

import type { SessionProcessPolicy } from "../adapters/session-processes-api";
import { processPortsSentence, sessionProcessChoiceCopy } from "../lib/session-process-choice";

interface SessionPrimaryActionProps {
  showUnarchiveAction: boolean;
  showStopAction: boolean;
  controlsBusy: boolean;
  isUnarchiving: boolean;
  isStopping: boolean;
  onUnarchive: () => void;
  /** `processes` answers what happens to running processes; absent when none run. */
  onStop: (processes?: SessionProcessPolicy) => void;
  hasLiveGoal?: boolean;
  /** Processes the session left running; above zero the stop asks about them. */
  runningProcesses?: number;
  processPorts?: readonly number[];
}

function primaryAction(props: SessionPrimaryActionProps) {
  if (props.showUnarchiveAction)
    return {
      onClick: props.onUnarchive,
      disabled: props.controlsBusy,
      busy: props.isUnarchiving,
      testId: "unarchive-button",
      label: "Unarchive session",
      Glyph: RotateCcw,
    };
  if (props.showStopAction)
    return {
      onClick: () => props.onStop(),
      disabled: props.controlsBusy && !props.isStopping,
      busy: props.isStopping,
      testId: "stop-button",
      label: "Stop session",
      Glyph: Square,
    };
  return null;
}

export function SessionPrimaryAction(props: SessionPrimaryActionProps) {
  const action = primaryAction(props);
  return action ? (
    <SessionPrimaryActionControl key={action.testId} {...props} action={action} />
  ) : null;
}

function SessionPrimaryActionControl({
  action,
  ...props
}: SessionPrimaryActionProps & { action: NonNullable<ReturnType<typeof primaryAction>> }) {
  const [confirmStop, setConfirmStop] = useState(false);
  const Glyph = action.busy ? Spinner : action.Glyph;
  const runningProcesses = props.runningProcesses ?? 0;
  const processChoice =
    runningProcesses > 0 ? sessionProcessChoiceCopy("stop", 1, runningProcesses) : null;
  const stop = (processes?: SessionProcessPolicy) => {
    setConfirmStop(false);
    props.onStop(processes);
  };
  return (
    <>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              className="focus-visible:shadow-focus-inset"
              onClick={() => {
                if (
                  props.showStopAction &&
                  !props.isStopping &&
                  (props.hasLiveGoal !== false || processChoice !== null)
                )
                  setConfirmStop(true);
                else action.onClick();
              }}
              disabled={action.disabled}
              data-testid={action.testId}
              aria-label={action.label}
            />
          }
        >
          <Glyph aria-hidden="true" className="size-3.5" />
        </TooltipTrigger>
        <TooltipContent>{action.label}</TooltipContent>
      </Tooltip>
      <ConfirmDialog
        open={confirmStop && props.showStopAction}
        onOpenChange={setConfirmStop}
        title="Stop this session?"
        description={[
          props.hasLiveGoal === true
            ? "The current work will be interrupted and this session's goal will be paused."
            : props.hasLiveGoal === false
              ? "The current work will be interrupted."
              : "The current work will be interrupted. Any active goal will be paused.",
          processChoice?.description,
          processPortsSentence(props.processPorts ?? []),
        ]
          .filter(Boolean)
          .join(" ")}
        note={
          props.hasLiveGoal === true
            ? "Sending a message reopens the session. The goal stays paused until you choose Resume goal."
            : "Sending a message reopens the session."
        }
        confirmLabel={processChoice?.confirmLabel ?? "Stop session"}
        cancelLabel={processChoice ? "Cancel" : "Keep working"}
        tone="warning"
        icon={Square}
        className={processChoice ? "sm:max-w-lg" : undefined}
        contentProps={{ "data-testid": "stop-session-dialog" }}
        confirmButtonProps={{ "data-testid": "stop-session-confirm" }}
        alternateAction={
          processChoice
            ? {
                label: processChoice.keepLabel,
                onSelect: () => stop("keep"),
                buttonProps: { "data-testid": "stop-session-keep-processes" },
              }
            : undefined
        }
        onConfirm={() => stop(processChoice ? "terminate" : undefined)}
      />
    </>
  );
}
