import { MessagePrimitive, useAuiState } from "@assistant-ui/react";
import { AlertCircle } from "@compozy/ui/icons";

import { cn } from "@/lib/utils";
import { Marker } from "@compozy/ui";
import { isGoalCommandFailureGuidance } from "@/systems/session/lib/session-goal-chat-transport";
import { assistantMessageHasContent } from "@/systems/session/lib/session-thinking-state";
import { useAssistantMessagePlacement } from "./hooks/use-assistant-message-placement";
import { MessageActions } from "./message-actions";
import { formatMessageError, providerDiagnosticOwnsError } from "./session-thread-error";
import { SessionThreadErrorBoundary } from "./session-thread-error-boundary";
import { AssistantMessageTimeline } from "./session-timeline-render";

// Message-level failure as a one-line danger marker. Goal-command failures are
// owned by SessionGoalCommandErrorNotice in the composer zone, and a provider
// diagnostic already rendered by the timeline owns its own failure, never here.
function SessionMessageErrorNotice() {
  const error = useAuiState(state => {
    const status = state.message.status;
    if (status?.type !== "incomplete" || status.reason !== "error") {
      return null;
    }
    const message = formatMessageError(status.error);
    if (message !== null && providerDiagnosticOwnsError(state.message.content, message)) {
      return null;
    }
    return message;
  });

  if (error === null || isGoalCommandFailureGuidance(error)) {
    return null;
  }

  return (
    <Marker
      role="alert"
      data-testid="session-message-error"
      tone="destructive"
      icon={<AlertCircle />}
    >
      {error}
    </Marker>
  );
}

/**
 * One assistant turn: the derived timeline, its error marker, and hover meta.
 * No shell exists before content (ADR-006 rule 3): while the turn is only
 * running, the status row under the scroller reads "Thinking…" and this
 * message renders nothing — no bubble, no floating copy action. The turn
 * mounts with its first token, tool, or rich event; a failure before any
 * content mounts the error marker alone.
 */
export function AssistantMessage() {
  const { traceCount, renders, hasRenderableContent, joinsPrevious, joinsNext } =
    useAssistantMessagePlacement();
  const hasContent = useAuiState(state => assistantMessageHasContent(state.message.content));
  const isRunning = useAuiState(state => {
    const status = state.message.status;
    return status?.type === "running" || status?.type === "incomplete";
  });

  if (!renders) {
    return null;
  }

  // One turn split across messages (a steer delivered mid-turn) keeps the turn's
  // inline rhythm between its pieces; only the turn's last piece opens the gap.
  return (
    <MessagePrimitive.Root
      data-testid="assistant-message"
      className={cn(
        "group/message flex w-full min-w-0",
        joinsPrevious ? null : "pt-1",
        joinsNext
          ? "pb-transcript-inline-gap"
          : isRunning
            ? "pb-transcript-message-y"
            : "pb-transcript-turn-gap"
      )}
    >
      <div className="flex min-w-0 flex-1 flex-col gap-transcript-inline-gap">
        {hasRenderableContent ? (
          <SessionThreadErrorBoundary>
            <AssistantMessageTimeline queueTraceCount={traceCount} />
          </SessionThreadErrorBoundary>
        ) : null}
        <SessionMessageErrorNotice />
        {hasContent ? (
          <MessageActions
            align="start"
            copyLabel="Copy message"
            testId="assistant-message-actions"
          />
        ) : null}
      </div>
    </MessagePrimitive.Root>
  );
}
