import { ReadonlyThreadProvider, ThreadPrimitive, useAuiState } from "@assistant-ui/react";
import type { VirtualItem } from "@tanstack/react-virtual";
import { type ComponentProps, type ContextType, use, useEffect } from "react";

import {
  SessionContinueDivider,
  SessionContinueEmptyChild,
  SessionLoadOlderButton,
  SessionOriginContext,
} from "@/systems/session";

import {
  recordSessionDebugEvent,
  SESSION_DEBUG_EVENTS,
} from "@/systems/session/lib/session-observability";
import { toTimelineParts } from "@/systems/session/lib/timeline-message-parts";
import {
  sessionMessageOrigin,
  sessionReplyMeta,
} from "@/systems/session/lib/session-message-payload";
import { SubagentOriginContext } from "@/systems/session/contexts/session-subagents-context-value";
import { SubagentOriginDivider } from "@/systems/session/components/subagents/subagent-origin-divider";
import { AssistantMessage } from "./session-assistant-message";
import { deriveSessionRows } from "./session-timeline.logic";
import { ThreadStatePane } from "./session-thread-states";
import { UserMessage } from "./session-user-message";
import { SessionAgentMessage, SessionReplyMessage } from "./session-agent-message";
import {
  type SessionFailurePayload,
  type SessionState,
  useSessionTranscriptThreadState,
  useSessionTransportState,
} from "@/systems/session";

function SessionThreadMessage() {
  const role = useAuiState(state => state.message.role);
  const metadata = useAuiState(state => state.message.metadata);

  if (role === "user") {
    // Another session sent this turn (S1): its card, not the operator bubble.
    const origin = sessionMessageOrigin(metadata);
    return origin ? <SessionAgentMessage origin={origin} /> : <UserMessage />;
  }
  // A reply that came back (S2) is the one synthetic wake with a visible card.
  const reply = sessionReplyMeta(metadata);
  if (reply) {
    return <SessionReplyMessage reply={reply} />;
  }
  if (role === "assistant") {
    return <AssistantMessage />;
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
  const origin = use(SessionOriginContext);
  const subagentOrigin = use(SubagentOriginContext);
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
            className="w-full"
          >
            {/* The derived child's own transcript starts here; nothing older is its own. */}
            {origin && messageIndex === 0 && leadingItemCount === 0 ? (
              <SessionContinueDivider onOpenSource={origin.onOpenSource} origin={origin.origin} />
            ) : null}
            {/* A delegated subagent's transcript opens with its parent (S4). */}
            {subagentOrigin && messageIndex === 0 && leadingItemCount === 0 ? (
              <SubagentOriginDivider
                parent={subagentOrigin.parent}
                onOpenParent={subagentOrigin.onOpenParent}
              />
            ) : null}
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

// Status events (hook dispatches, usage, progress ticks) are transcript messages that
// render nothing, so a transcript holding only those has said nothing yet.
function hasNarrativeMessage(
  messages: ReturnType<typeof useSessionTranscriptThreadState>["messages"]
): boolean {
  return messages.some(
    message =>
      message.role === "user" ||
      sessionReplyMeta(message.metadata) !== null ||
      deriveSessionRows(toTimelineParts(message)).length > 0
  );
}

// A derived child's placeholder shows only once the transcript settled with no
// sync, startup, or failure state that the status pane must explain instead.
function isDerivedChildReady({
  transcriptStatus,
  syncFailure,
  sessionState,
  startupFailed,
  failure,
}: {
  transcriptStatus: ReturnType<typeof useSessionTranscriptThreadState>["status"];
  syncFailure: unknown;
  sessionState?: SessionState;
  startupFailed: boolean;
  failure?: SessionFailurePayload | null;
}): boolean {
  return (
    transcriptStatus === "success" &&
    syncFailure === null &&
    sessionState !== "starting" &&
    !(startupFailed && failure)
  );
}

// Before its first message a derived child shows only where it came from;
// while that first prompt runs, the divider stands alone above the status row.
function DerivedChildPlaceholder({
  origin,
  isSessionRunning,
}: {
  origin: NonNullable<ContextType<typeof SessionOriginContext>>;
  isSessionRunning: boolean;
}) {
  return isSessionRunning ? (
    <SessionContinueDivider onOpenSource={origin.onOpenSource} origin={origin.origin} />
  ) : (
    <SessionContinueEmptyChild onOpenSource={origin.onOpenSource} origin={origin.origin} />
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
  const transport = useSessionTransportState();
  const origin = use(SessionOriginContext);
  const syncFailure =
    transport.phase === "failed" && transport.failure !== null
      ? { attempts: transport.failure.attempts, retry: transport.retry }
      : null;
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

  // Nothing said yet: no messages, or only status events that render no row. With
  // older history still unloaded the window cannot claim the session is empty.
  const saidNothing =
    messageCount === 0 || (!showLoadOlder && !hasNarrativeMessage(transcriptMessages));
  if (
    origin !== null &&
    saidNothing &&
    isDerivedChildReady({ transcriptStatus, syncFailure, sessionState, startupFailed, failure })
  ) {
    return <DerivedChildPlaceholder origin={origin} isSessionRunning={isSessionRunning} />;
  }

  if (saidNothing) {
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
          syncFailure={syncFailure}
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
