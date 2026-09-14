import type { SessionGoalSnapshot } from "../types";
import {
  agentCountLabel,
  formatFrozenDuration,
  type SessionLastTurn,
  type SessionWorkingStatus,
} from "./session-working-status";

/** Rows of the Activity section; a field is absent when the client has no such signal. */
export interface SessionActivityView {
  status?: string;
  agents?: string;
  tools?: string;
  thoughts?: string;
  queued?: string;
  goal?: string;
  warning?: string;
}

/** "Working for 49m 20s · Running shell" · "Stopped by you after 1h 12m" · "Worked for 4m 12s". */
export function describeActivityStatus(
  status: SessionWorkingStatus,
  lastTurn: SessionLastTurn | null
): string | undefined {
  switch (status.kind) {
    case "working":
      return `${status.elapsed ? `Working for ${status.elapsed}` : "Working"}${status.activity ? ` · ${status.activity}` : ""}`;
    case "stopped":
      return `Stopped ${status.byYou ? "by you " : ""}after ${status.duration}${status.detail ? ` · ${status.detail}` : ""}`;
    case "failed":
      return `Failed after ${status.duration}${status.cause ? ` · ${status.cause}` : ""}`;
    case "hidden":
      return lastTurn?.startedAtMs != null && lastTurn.endedAtMs != null
        ? `Worked for ${formatFrozenDuration(lastTurn.endedAtMs - lastTurn.startedAtMs)}`
        : undefined;
    default:
      return undefined;
  }
}

function countLabel(count: number, noun: string): string | undefined {
  return count > 0 ? `${count} ${noun}${count === 1 ? "" : "s"}` : undefined;
}

export interface SessionActivityInput {
  status: SessionWorkingStatus;
  lastTurn: SessionLastTurn | null;
  toolCount: number;
  thoughts: number;
  warning: string | undefined;
  queued: number | undefined;
  goal: SessionGoalSnapshot | null;
  /** A stopped session has no queue and no live goal to report. */
  stopped: boolean;
}

export function deriveSessionActivityView(input: SessionActivityInput): SessionActivityView {
  const { status, queued, goal, stopped } = input;
  return {
    status: describeActivityStatus(status, input.lastTurn),
    agents:
      status.kind === "working" && status.agentCount > 0
        ? agentCountLabel(status.agentCount)
        : undefined,
    tools: countLabel(input.toolCount, "tool"),
    thoughts: countLabel(input.thoughts, "thought"),
    queued: queued != null && queued > 0 && !stopped ? `${queued} queued` : undefined,
    goal:
      goal && !stopped
        ? `Goal · turn ${goal.turns_used}/${goal.turn_limit} · ${goal.status}`
        : undefined,
    warning: input.warning,
  };
}
