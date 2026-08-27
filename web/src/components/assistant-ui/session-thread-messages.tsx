import { ReadonlyThreadProvider, ThreadPrimitive, useAuiState } from "@assistant-ui/react";
import type { VirtualItem } from "@tanstack/react-virtual";
import { type ComponentProps, useEffect } from "react";

import { SessionLoadOlderButton } from "@/systems/session";

import {
  recordSessionDebugEvent,
  SESSION_DEBUG_EVENTS,
} from "@/systems/session/lib/session-observability";
import { AssistantMessage } from "./session-assistant-message";
import { SessionSyntheticMessage } from "./session-synthetic-turn";
import { ThreadStatePane } from "./session-thread-states";
import { UserMessage } from "./session-user-message";
import { readSyntheticTurn } from "@/systems/agent-comms";
import {
  type SessionFailurePayload,
  type SessionState,
  useSessionTranscriptThreadState,
} from "@/systems/session";

function SessionThreadMessage() {
  const message = useAuiState(state => state.message);
  const role = message.role;

  if (role === "user") {
    return <UserMessage />;
  }
  if (role === "assistant") {
    return <AssistantMessage />;
  }
  if (role === "system") {
    const synthetic = readSyntheticTurn(message.metadata);
    return synthetic === null ? null : (
      <SessionSyntheticMessage
        synthetic={synthetic}
        content={message.content}
        timestamp={message.createdAt.toISOString()}
      />
    );
  }
  return null;
}

type MessageComponents = ComponentProps<typeof ThreadPrimitive.Unstable_MessageById>["components"];

const MESSAGE_COMPONENTS: MessageComponents = { Message: SessionThreadMessage };

function ThreadMessageRows({
  messageIds,
  measureVirtualElement,
  virtualItems,
  virtualTotalSize,
  leadingItemCount,
  isFetchingOlder,
  loadOlder,
}: {
  messageIds: readonly string[];
  measureVirtualElement: (element: HTMLDivElement | null) => void;
  virtualItems: VirtualItem[];
  virtualTotalSize: number;
  leadingItemCount: number;
  isFetchingOlder: boolean;
  loadOlder: () => void;
}) {
  const paddingTop = virtualItems[0]?.start ?? 0;
  const paddingBottom = Math.max(0, virtualTotalSize - (virtualItems.at(-1)?.end ?? 0));

  return (
    <div className="w-full" style={{ paddingTop, paddingBottom }} data-testid="thread-messages">
      {virtualItems.map(virtualItem => {
        if (leadingItemCount > 0 && virtualItem.index === 0) {
          return (
            <div
              key={virtualItem.key}
              ref={measureVirtualElement}
              data-index={virtualItem.index}
              className="w-full"
            >
              <SessionLoadOlderButton isLoading={isFetchingOlder} onClick={loadOlder} />
            </div>
          );
        }

        const messageIndex = virtualItem.index - leadingItemCount;
        if (messageIndex < 0 || messageIndex >= messageIds.length) {
          return null;
        }
        const messageId = messageIds[messageIndex];
        if (!messageId) return null;

        return (
          <div
            key={virtualItem.key}
            ref={measureVirtualElement}
            data-index={virtualItem.index}
            data-message-id={messageId}
            data-message-index={messageIndex}
            data-testid="thread-message-row"
            className="w-full [content-visibility:auto] [contain-intrinsic-size:auto_120px]"
          >
            <ThreadPrimitive.Unstable_MessageById
              messageId={messageId}
              components={MESSAGE_COMPONENTS}
            />
          </div>
        );
      })}
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
  measureVirtualElement,
  virtualItems,
  virtualTotalSize,
  leadingItemCount,
  showLoadOlder,
  isFetchingOlder,
  loadOlder,
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
  measureVirtualElement: (element: HTMLDivElement | null) => void;
  virtualItems: VirtualItem[];
  virtualTotalSize: number;
  leadingItemCount: number;
  showLoadOlder: boolean;
  isFetchingOlder: boolean;
  loadOlder: () => void;
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
      <>
        {showLoadOlder ? (
          <SessionLoadOlderButton isLoading={isFetchingOlder} onClick={loadOlder} />
        ) : null}
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
      </>
    );
  }

  return (
    <ReadonlyThreadProvider messages={transcriptMessages}>
      <ThreadMessageRows
        messageIds={transcriptMessages.map(message => message.id)}
        measureVirtualElement={measureVirtualElement}
        virtualItems={virtualItems}
        virtualTotalSize={virtualTotalSize}
        leadingItemCount={leadingItemCount}
        isFetchingOlder={isFetchingOlder}
        loadOlder={loadOlder}
      />
    </ReadonlyThreadProvider>
  );
}
