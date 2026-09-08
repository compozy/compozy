import { MessagePrimitive, useAuiState } from "@assistant-ui/react";
import { AlertCircle } from "lucide-react";

import { cn } from "@/lib/utils";
import { Marker } from "@compozy/ui";
import { isGoalCommandFailureGuidance } from "@/systems/session/lib/session-goal-chat-transport";
import {
  assistantMessageHasContent,
  assistantMessageHasRenderableContent,
} from "@/systems/session/lib/session-thinking-state";
import { queueClearTraceCount } from "./session-queue-trace.logic";
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
      tone="danger"
      icon={<AlertCircle strokeWidth={1.8} />}
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
  const traceCount = useAuiState(state =>
    queueClearTraceCount(state.thread.messages, state.message.id)
  );
  const hasRenderableContent = useAuiState(state =>
    assistantMessageHasRenderableContent(state.message.content)
  );
  const hasContent = useAuiState(state => assistantMessageHasContent(state.message.content));
  const failed = useAuiState(state => {
    const status = state.message.status;
    return status?.type === "incomplete" && status.reason === "error";
  });
  const isRunning = useAuiState(state => {
    const status = state.message.status;
    return status?.type === "running" || status?.type === "incomplete";
  });

  if (traceCount === 0 || (!hasRenderableContent && !failed)) {
    return null;
  }

  return (
    <MessagePrimitive.Root
      data-testid="assistant-message"
      className={cn("group/message flex w-full min-w-0 pt-1", isRunning ? "pb-[7px]" : "pb-[18px]")}
    >
      <div className="flex min-w-0 flex-1 flex-col gap-[7px]">
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
