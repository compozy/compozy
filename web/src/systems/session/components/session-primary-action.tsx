import { useState } from "react";
import {
  Button,
  ConfirmDialog,
  Spinner,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";
import { RotateCcw, Square } from "lucide-react";

interface SessionPrimaryActionProps {
  showUnarchiveAction: boolean;
  showStopAction: boolean;
  controlsBusy: boolean;
  isUnarchiving: boolean;
  isStopping: boolean;
  onUnarchive: () => void;
  onStop: () => void;
  hasLiveGoal?: boolean;
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
      onClick: props.onStop,
      disabled: props.controlsBusy && !props.isStopping,
      busy: props.isStopping,
      testId: "stop-button",
      label: "Stop session",
      Glyph: Square,
    };
  return null;
}

export function SessionPrimaryAction(props: SessionPrimaryActionProps) {
  const [confirmStop, setConfirmStop] = useState(false);
  const action = primaryAction(props);
  if (!action) return null;
  const Glyph = action.busy ? Spinner : action.Glyph;
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
                if (props.showStopAction && props.hasLiveGoal && !props.isStopping)
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
        description="The current work will be interrupted and this session's goal will be paused."
        note="Sending a message reopens the session. The goal stays paused until you choose Resume goal."
        confirmLabel="Stop session"
        cancelLabel="Keep working"
        tone="warning"
        icon={Square}
        onConfirm={() => {
          setConfirmStop(false);
          props.onStop();
        }}
      />
    </>
  );
}
