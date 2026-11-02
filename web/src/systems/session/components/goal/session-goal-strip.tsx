import { useId, useState } from "react";

import { Eyebrow, Pill, Spinner, TranscriptDisclosure, type PillTone } from "@compozy/ui";

import { describeGoalCommandFailure } from "../../lib/session-goal-chat-transport";

import type { GoalComposerAffordance, SessionGoalSnapshot } from "./goal-status-types";
import { SessionGoalControls, type SessionGoalControlsProps } from "./session-goal-controls";
import { SessionGoalStripBody } from "./session-goal-strip-body";

export interface SessionGoalStripProps extends SessionGoalControlsProps {
  snapshot: SessionGoalSnapshot;
  composerAffordance?: GoalComposerAffordance;
  onPrefillComposer?: (text: string) => void;
}

/** Maps durable Goal and run state to the visible status and tone. */
function goalState(snapshot: SessionGoalSnapshot): {
  state: string;
  label: string;
  tone: PillTone;
} {
  if (snapshot.run_status === "canceled")
    return { state: "stopped", label: "Stopped", tone: "neutral" };
  if (snapshot.run_status === "done" || snapshot.status === "complete")
    return { state: "done", label: "Complete", tone: "success" };
  if (snapshot.run_status === "failed")
    return { state: "blocked", label: "Failed", tone: "destructive" };
  if (snapshot.run_status === "needs-approval")
    return { state: "blocked", label: "Needs approval", tone: "warning" };
  if (snapshot.run_status === "paused" || snapshot.status === "paused")
    return { state: "paused", label: "Paused", tone: "warning" };
  if (snapshot.pause_requested) return { state: "pausing", label: "Pausing…", tone: "warning" };
  if (snapshot.status === "waiting" && snapshot.live)
    return { state: "waiting", label: "Waiting", tone: "neutral" };
  if (snapshot.status === "active" && snapshot.live)
    return { state: "active", label: "Active", tone: "brand" };
  if (snapshot.status === "budget-limited")
    return { state: "blocked", label: "Budget reached", tone: "warning" };
  if (snapshot.status === "usage-limited")
    return { state: "blocked", label: "Usage limit reached", tone: "warning" };
  return { state: "blocked", label: "Blocked", tone: "warning" };
}

/** Explains waiting, control transitions, and wait attention causes on the Goal surface. */
function goalStateGuidance(state: string, cause: SessionGoalSnapshot["cause"]): string | null {
  if (cause === "goal_wait_timeout" || cause === "goal_wait_unavailable") {
    return describeGoalCommandFailure(cause);
  }
  switch (state) {
    case "paused":
      return "Goal paused. Resume it explicitly to continue.";
    case "waiting":
      return "Waiting for a registered monitor or delegated work. No new Goal turns start.";
    case "stopped":
      return "Goal ended. Your session and run history are kept.";
    case "pausing":
      return "Interrupting the current goal turn. No further goal turns will start.";
    default:
      return null;
  }
}

const PENDING_LABEL = {
  pause: "Pausing…",
  resume: "Resuming…",
  approve: "Approving…",
  stop: "Stopping…",
  clear: "Clearing…",
};

/** One control surface: explicit state and actions, followed by the objective and its details. */
export function SessionGoalStrip({
  snapshot,
  composerAffordance,
  onPrefillComposer,
  ...controls
}: SessionGoalStripProps) {
  const [open, setOpen] = useState(false);
  const detailsId = useId();
  const state = goalState(snapshot);
  const moved = snapshot.bound_session_id !== snapshot.origin_session_id;
  const facts = `Step ${snapshot.turns_used} of ${snapshot.turn_limit}`;
  const pending = controls.pendingAction !== undefined || state.state === "pausing";
  const guidance = goalStateGuidance(state.state, snapshot.cause);
  const label = controls.pendingAction ? PENDING_LABEL[controls.pendingAction] : state.label;

  return (
    <section
      aria-label="Goal status"
      data-testid="session-goal-strip"
      data-state={state.state}
      data-goal-status={snapshot.status}
      data-run-status={snapshot.run_status}
      data-live={snapshot.live ? "true" : "false"}
      data-goal-node={snapshot.node_id}
      data-goal-cause={snapshot.cause || undefined}
      className="min-w-0 border-b border-line py-2"
    >
      <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-2 px-1 pb-1">
        <div className="flex min-w-0 items-center gap-3">
          <Eyebrow className="shrink-0 text-subtle">Goal</Eyebrow>
          <Pill
            form="plain"
            tone={state.tone}
            aria-live="polite"
            aria-atomic="true"
            data-testid="goal-control-state"
          >
            {pending ? <Spinner className="size-3" /> : <Pill.Dot data-testid="goal-strip-dot" />}
            {label}
          </Pill>
        </div>
        <SessionGoalControls snapshot={snapshot} {...controls} />
      </div>
      <TranscriptDisclosure
        className="w-full min-w-0 gap-2 font-normal"
        icon={null}
        variant="turn"
        aria-controls={open ? detailsId : undefined}
        data-testid="goal-strip-line"
        expanded={open}
        label={
          <span className="block max-w-full truncate text-small-body text-fg">
            {snapshot.objective}
          </span>
        }
        onToggle={() => setOpen(value => !value)}
        trailing={
          <span
            aria-atomic="true"
            aria-live="polite"
            data-testid="goal-strip-facts"
            className="ml-auto shrink-0 text-transcript-meta text-subtle tabular-nums"
          >
            {moved ? `Moved · ${facts}` : facts}
          </span>
        }
      />
      {guidance && (!pending || state.state === "pausing") ? (
        <p className="px-1 pt-1 text-transcript-caption text-subtle">{guidance}</p>
      ) : null}
      {snapshot.context.diagnostic ? (
        <p role="status" className="px-1 pt-1 text-transcript-caption text-subtle">
          Context usage is unavailable. Goal controls remain available.
        </p>
      ) : null}
      {open ? (
        <SessionGoalStripBody
          id={detailsId}
          snapshot={snapshot}
          moved={moved}
          composerAffordance={composerAffordance}
          onPrefillComposer={onPrefillComposer}
        />
      ) : null}
    </section>
  );
}
