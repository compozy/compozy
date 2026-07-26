import type { ComponentProps } from "react";
import { CheckCircle2, FilePenLine, Pause, Play, RefreshCw, X } from "lucide-react";

import { Button, Spinner } from "@agh/ui";

import type { GoalControlAction } from "@/systems/loops";
import type { GoalComposerAffordance, GoalStatusChipProps } from "./goal-status-types";

function prefillCommand(affordance: GoalComposerAffordance): string {
  if (affordance.kind === "replace") {
    return `/goal replace ${affordance.expectedRunId} ${affordance.objective}`;
  }
  return `/goal ${affordance.expandedObjective}`;
}

function ActionButton({
  action,
  pendingAction,
  children,
  ...props
}: ComponentProps<typeof Button> & {
  action: GoalControlAction | "prefill";
  pendingAction?: GoalStatusChipProps["pendingAction"];
}) {
  const pending = pendingAction === action;
  return (
    <Button disabled={pendingAction !== undefined} size="sm" {...props}>
      {pending ? <Spinner className="size-3" /> : null}
      {children}
    </Button>
  );
}

export function GoalControls({
  snapshot,
  composerAffordance,
  pendingAction,
  onPause,
  onResume,
  onApprove,
  onClear,
  onPrefillComposer,
}: Pick<
  GoalStatusChipProps,
  | "snapshot"
  | "composerAffordance"
  | "pendingAction"
  | "onPause"
  | "onResume"
  | "onApprove"
  | "onClear"
  | "onPrefillComposer"
>) {
  const showPause = snapshot.live && snapshot.status === "active" && onPause !== undefined;
  const showResume =
    snapshot.live &&
    (snapshot.status === "paused" || snapshot.run_status === "paused") &&
    onResume !== undefined;
  const showApprove =
    snapshot.live && snapshot.run_status === "needs-approval" && onApprove !== undefined;
  const hasControls =
    showPause ||
    showResume ||
    showApprove ||
    onClear !== undefined ||
    (composerAffordance !== undefined && onPrefillComposer !== undefined);

  if (!hasControls) return null;

  return (
    <div className="flex flex-wrap items-center gap-2 border-t border-line-soft px-3 py-2.5 sm:px-4">
      {composerAffordance && onPrefillComposer ? (
        <ActionButton
          action="prefill"
          pendingAction={pendingAction}
          variant="neutral"
          onClick={() => onPrefillComposer(prefillCommand(composerAffordance))}
        >
          {composerAffordance.kind === "replace" ? (
            <RefreshCw data-icon="inline-start" aria-hidden="true" />
          ) : (
            <FilePenLine data-icon="inline-start" aria-hidden="true" />
          )}
          {composerAffordance.kind === "replace" ? "Prefill replacement" : "Prefill draft"}
        </ActionButton>
      ) : null}
      <div className="ml-auto flex flex-wrap items-center gap-2">
        {showPause ? (
          <ActionButton
            action="pause"
            pendingAction={pendingAction}
            variant="neutral"
            onClick={onPause}
          >
            <Pause data-icon="inline-start" aria-hidden="true" />
            Pause goal
          </ActionButton>
        ) : null}
        {showResume ? (
          <ActionButton
            action="resume"
            pendingAction={pendingAction}
            variant="neutral"
            onClick={onResume}
          >
            <Play data-icon="inline-start" aria-hidden="true" />
            Resume goal
          </ActionButton>
        ) : null}
        {showApprove ? (
          <ActionButton
            action="approve"
            pendingAction={pendingAction}
            variant="success"
            onClick={onApprove}
          >
            <CheckCircle2 data-icon="inline-start" aria-hidden="true" />
            Approve goal
          </ActionButton>
        ) : null}
        {onClear ? (
          <ActionButton
            action="clear"
            pendingAction={pendingAction}
            variant="destructive"
            onClick={onClear}
          >
            <X data-icon="inline-start" aria-hidden="true" />
            Clear goal
          </ActionButton>
        ) : null}
      </div>
    </div>
  );
}
