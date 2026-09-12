import { useSecondClock } from "@/hooks/use-second-clock";
import { lastSettledTurn } from "@/components/assistant-ui/session-thread-status.logic";
import { toTimelineParts } from "@/components/assistant-ui/timeline-message-parts";
import { useSessionTranscriptThreadMessages } from "./use-session-transcript-thread-messages";
import {
  agentCountLabel,
  deriveWorkingStatus,
  formatFrozenDuration,
} from "../lib/session-working-status";
import { isAgentEventPayload } from "../lib/message-parts";
import { isRuntimeActivityEvent } from "../components/runtime-activity-notice.logic";
import type { SessionActivityView } from "../components/session-activity-section";
import type { SessionGoalSnapshot, SessionPayload } from "../types";

export interface SessionContextActivitySource {
  session: SessionPayload;
  running: boolean;
  live: boolean;
  queued: number | undefined;
  goal: SessionGoalSnapshot | null;
}

export function useSessionContextActivity(
  session: SessionPayload,
  running: boolean,
  live: boolean,
  queued: number | undefined,
  goal: SessionGoalSnapshot | null
): SessionActivityView {
  const messages = useSessionTranscriptThreadMessages();
  const nowMs = useSecondClock(running && live);
  const lastTurn = running ? null : lastSettledTurn(messages, session);
  const status = deriveWorkingStatus({ session, running, thinking: false, lastTurn, nowMs });
  // The compiler caches this pure calculation by message identity, outside the leaf clock.
  const { toolCount, thoughts, warning } = activityTranscriptSummary(messages);
  let statusText: string | undefined;
  if (status.kind === "working")
    statusText = `${status.elapsed ? `Working for ${status.elapsed}` : "Working"}${status.activity ? ` · ${status.activity}` : ""}`;
  if (status.kind === "stopped")
    statusText = `Stopped ${status.byYou ? "by you " : ""}after ${status.duration}${status.detail ? ` · ${status.detail}` : ""}`;
  if (status.kind === "failed")
    statusText = `Failed after ${status.duration}${status.cause ? ` · ${status.cause}` : ""}`;
  if (status.kind === "hidden" && lastTurn?.startedAtMs != null && lastTurn.endedAtMs != null)
    statusText = `Worked for ${formatFrozenDuration(lastTurn.endedAtMs - lastTurn.startedAtMs)}`;
  return {
    status: statusText,
    agents:
      status.kind === "working" && status.agentCount > 0
        ? agentCountLabel(status.agentCount)
        : undefined,
    tools: toolCount > 0 ? `${toolCount} tool${toolCount === 1 ? "" : "s"}` : undefined,
    thoughts: thoughts > 0 ? `${thoughts} thought${thoughts === 1 ? "" : "s"}` : undefined,
    queued:
      queued != null && queued > 0 && session.state !== "stopped" ? `${queued} queued` : undefined,
    goal:
      goal && session.state !== "stopped"
        ? `Goal · turn ${goal.turns_used}/${goal.turn_limit} · ${goal.status}`
        : undefined,
    warning,
  };
}

function activityTranscriptSummary(
  messages: ReturnType<typeof useSessionTranscriptThreadMessages>
) {
  const parts = messages.flatMap(message => toTimelineParts(message));
  const latestWarning = [...parts]
    .reverse()
    .find(
      part =>
        part.kind === "data" &&
        isAgentEventPayload(part.data) &&
        part.data.type === "runtime_warning" &&
        isRuntimeActivityEvent(part.data)
    );
  return {
    toolCount: parts.filter(part => part.kind === "tool").length,
    thoughts: parts.filter(part => part.kind === "reasoning").length,
    warning:
      latestWarning?.kind === "data" && isAgentEventPayload(latestWarning.data)
        ? latestWarning.data.text || latestWarning.data.runtime?.last_activity_detail
        : undefined,
  };
}
