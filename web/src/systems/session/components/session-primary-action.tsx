import { Button, Spinner } from "@compozy/ui";
import { Play, RotateCcw, Square } from "lucide-react";

interface SessionPrimaryActionProps {
  showUnarchiveAction: boolean;
  showStopAction: boolean;
  canResume: boolean;
  controlsBusy: boolean;
  isUnarchiving: boolean;
  isStopping: boolean;
  isResuming: boolean;
  onUnarchive: () => void;
  onStop: () => void;
  onResume: () => void;
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
  if (props.canResume)
    return {
      onClick: props.onResume,
      disabled: props.controlsBusy && !props.isResuming,
      busy: props.isResuming,
      testId: "resume-button",
      label: "Attach session",
      Glyph: Play,
    };
  return null;
}

export function SessionPrimaryAction(props: SessionPrimaryActionProps) {
  const action = primaryAction(props);
  if (!action) return null;
  const Glyph = action.busy ? Spinner : action.Glyph;
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon-sm"
      className="size-11 focus-visible:shadow-focus-inset"
      onClick={action.onClick}
      disabled={action.disabled}
      data-testid={action.testId}
      aria-label={action.label}
    >
      <Glyph className="size-3" />
    </Button>
  );
}
