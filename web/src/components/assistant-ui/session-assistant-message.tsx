import { MessagePrimitive, useAuiState } from "@assistant-ui/react";
import { AlertCircle } from "lucide-react";

import { cn } from "@/lib/utils";
import { Marker } from "@compozy/ui";
import { isGoalCommandFailureGuidance } from "@/systems/session/lib/session-goal-chat-transport";
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

/** One assistant turn: the derived timeline, its error marker, and hover meta. */
export function AssistantMessage() {
  const isRunning = useAuiState(state => {
    const status = state.message.status;
    return status?.type === "running" || status?.type === "incomplete";
  });

  return (
    <MessagePrimitive.Root
      className={cn("group/message flex w-full min-w-0 pt-1", isRunning ? "pb-[7px]" : "pb-[18px]")}
    >
      <div className="flex min-w-0 flex-1 flex-col gap-[7px]">
        <SessionThreadErrorBoundary>
          <AssistantMessageTimeline />
        </SessionThreadErrorBoundary>
        <SessionMessageErrorNotice />
        <MessageActions align="start" copyLabel="Copy message" testId="assistant-message-actions" />
      </div>
    </MessagePrimitive.Root>
  );
}
