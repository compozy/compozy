import { useSecondClock } from "@/hooks/use-second-clock";
import { lastSettledTurn } from "@/components/assistant-ui/session-thread-status.logic";
import { toTimelineParts } from "@/components/assistant-ui/timeline-message-parts";
import { useSessionTranscriptThreadMessages } from "./use-session-transcript-thread-messages";
import { deriveWorkingStatus } from "../lib/session-working-status";
import { deriveSessionActivityView, type SessionActivityView } from "../lib/session-activity-view";
import { isAgentEventPayload } from "../lib/message-parts";
import { isRuntimeActivityEvent } from "../components/runtime-activity-notice.logic";
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
  const summary = activityTranscriptSummary(messages);
  return deriveSessionActivityView({
    status,
    lastTurn,
    ...summary,
    queued,
    goal,
    stopped: session.state === "stopped",
  });
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
