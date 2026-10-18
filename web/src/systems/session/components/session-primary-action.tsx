import { Button, Spinner, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";
import { RotateCcw, Square } from "lucide-react";

interface SessionPrimaryActionProps {
  showUnarchiveAction: boolean;
  showStopAction: boolean;
  controlsBusy: boolean;
  isUnarchiving: boolean;
  isStopping: boolean;
  onUnarchive: () => void;
  onStop: () => void;
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
  const action = primaryAction(props);
  if (!action) return null;
  const Glyph = action.busy ? Spinner : action.Glyph;
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            className="focus-visible:shadow-focus-inset"
            onClick={action.onClick}
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
  );
}
