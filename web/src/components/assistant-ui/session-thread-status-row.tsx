import {
  deriveThinkingState,
  SessionQuietStatusRow,
  SessionThinkingRow,
  type SessionQuietWarning,
  type SessionWorkingStatusInput,
  useSessionTranscriptThreadState,
  useSessionTransportState,
} from "@/systems/session";

import { useSessionPromptDispatch } from "./hooks/use-session-prompt-dispatch";
import { useThinkingGuardElapsed } from "./hooks/use-thinking-guard-elapsed";
import {
  activeReplyHasContent,
  lastSettledTurn,
  type SessionStopFacts,
} from "@/systems/session/lib/session-thread-status";

// Thinking needs nothing from the session resource; a surface without one
// (read-only rows, stories) still reads the pending reply truthfully.
const NO_STATUS_SESSION: SessionWorkingStatusInput["session"] = {
  activity: null,
  pending_interactions: [],
  supervision: null,
};

export type SessionThreadStatusSession = SessionWorkingStatusInput["session"] & SessionStopFacts;

export interface SessionThreadStatusRowProps {
  /** The session resource the status reads activity, signals, decisions and stop facts from. */
  session: SessionThreadStatusSession | null;
  /** The daemon answered the last turn stop with `nothing-in-flight` (US-009.EC-2). */
  stopCompletionNote?: boolean;
  /** The turn is in flight as the thread sees it (daemon or local stream). */
  running: boolean;
  stopping?: boolean;
  /** The daemon's quiet episode outranks every other status (task_05). */
  quietWarning: SessionQuietWarning | null;
  liveDataEnabled: boolean;
  reducedMotion: boolean;
}

/**
 * The one status line between the transcript and the composer (S3): the
 * quiet warning row while the daemon reports one, otherwise the thinking /
 * working / stopped / failed row derived from the session resource and the
 * transcript's own record of the last turn.
 */
export function SessionThreadStatusRow({
  session,
  running,
  quietWarning,
  liveDataEnabled,
  reducedMotion,
  stopCompletionNote = false,
  stopping = false,
}: SessionThreadStatusRowProps) {
  const { messages } = useSessionTranscriptThreadState();
  const transport = useSessionTransportState();
  const dispatch = useSessionPromptDispatch();
  const guardElapsed = useThinkingGuardElapsed(running ? dispatch.pendingSinceMs : null);

  if (quietWarning && !stopping) {
    return <SessionQuietStatusRow liveDataEnabled={liveDataEnabled} warning={quietWarning} />;
  }

  const hasContent = activeReplyHasContent(messages);
  // Inside the guard the clock reads the send instant itself (nothing elapsed);
  // past it the guard no longer applies — no wall clock is read during render.
  const sentAtMs = guardElapsed ? null : dispatch.pendingSinceMs;
  const thinking = deriveThinkingState({
    running,
    hasContent,
    failed: false,
    sentAtMs,
    nowMs: sentAtMs ?? 0,
  });
  if (thinking === "pending" && !stopping) return null;

  return (
    <SessionThinkingRow
      session={session ?? NO_STATUS_SESSION}
      running={running}
      stopping={stopping}
      thinking={thinking === "thinking"}
      lastTurn={running ? null : lastSettledTurn(messages, session ?? {})}
      liveDataEnabled={liveDataEnabled}
      pausedAtMs={liveDataEnabled ? null : transport.lastLiveAt}
      reducedMotion={reducedMotion}
      stopCompletionNote={stopCompletionNote}
    />
  );
}
