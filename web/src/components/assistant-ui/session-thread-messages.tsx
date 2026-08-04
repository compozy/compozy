import {
  MessageByIndexProvider,
  ReadonlyThreadProvider,
  type ThreadMessage,
  useAuiState,
} from "@assistant-ui/react";
import { useEffect } from "react";

import type { SessionFailurePayload, SessionState } from "@/systems/session";
import { useSessionTranscriptThreadState } from "@/systems/session";
import {
  recordSessionDebugEvent,
  SESSION_DEBUG_EVENTS,
} from "@/systems/session/lib/session-observability";
import { AssistantMessage } from "./session-assistant-message";
import { ThreadStatePane } from "./session-thread-states";
import { UserMessage } from "./session-user-message";

function SessionThreadMessage() {
  const role = useAuiState(state => state.message.role);

  if (role === "user") {
    return <UserMessage />;
  }
  if (role === "assistant") {
    return <AssistantMessage />;
  }
  return null;
}

function ThreadMessageRows({
  messageCount,
  transcriptMessages,
}: {
  messageCount: number;
  transcriptMessages: readonly ThreadMessage[];
}) {
  const committedMessageCount = useAuiState(state => state.thread.messages.length);
  const renderableCount = Math.min(messageCount, committedMessageCount);

  return (
    <div className="w-full" data-testid="thread-messages">
      {Array.from({ length: renderableCount }, (_, index) => (
        <div
          key={transcriptMessages[index]?.id ?? index}
          data-message-id={transcriptMessages[index]?.id}
          data-message-index={index}
          data-testid="thread-message-row"
          className="w-full [content-visibility:auto] [contain-intrinsic-size:auto_120px]"
        >
          <MessageByIndexProvider index={index}>
            <SessionThreadMessage />
          </MessageByIndexProvider>
        </div>
      ))}
    </div>
  );
}

export function ThreadMessages({
  agentName,
  sessionId,
  isSessionRunning,
  messageCount,
  transcriptStatus,
  transcriptError,
  retryTranscript,
  transcriptMessages,
  sessionState,
  failure,
  startupFailed,
}: {
  agentName: string;
  sessionId: string;
  isSessionRunning: boolean;
  messageCount: number;
  transcriptStatus: ReturnType<typeof useSessionTranscriptThreadState>["status"];
  transcriptError: ReturnType<typeof useSessionTranscriptThreadState>["error"];
  retryTranscript: () => void;
  transcriptMessages: ReturnType<typeof useSessionTranscriptThreadState>["messages"];
  sessionState?: SessionState;
  failure?: SessionFailurePayload | null;
  startupFailed: boolean;
}) {
  const emptyWhileActive = messageCount === 0 && transcriptStatus === "success" && isSessionRunning;
  useEffect(() => {
    if (!emptyWhileActive) {
      return;
    }
    recordSessionDebugEvent(SESSION_DEBUG_EVENTS.threadEmptyWhileActive, {
      agent_name: agentName,
      message_count: messageCount,
      session_id: sessionId,
      transcript_status: transcriptStatus,
    });
  }, [agentName, emptyWhileActive, messageCount, sessionId, transcriptStatus]);

  if (messageCount === 0) {
    return (
      <ThreadStatePane
        status={transcriptStatus}
        agentName={agentName}
        error={transcriptError}
        onRetry={retryTranscript}
        sessionState={sessionState}
        failure={failure}
        startupFailed={startupFailed}
        isSessionRunning={isSessionRunning}
      />
    );
  }

  return (
    <ReadonlyThreadProvider messages={transcriptMessages}>
      <ThreadMessageRows messageCount={messageCount} transcriptMessages={transcriptMessages} />
    </ReadonlyThreadProvider>
  );
}
