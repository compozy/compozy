import {
  type DataMessagePartProps,
  MessagePrimitive,
  type TextMessagePartProps,
  ThreadPrimitive,
  ReadonlyThreadProvider,
  type ThreadMessage,
  useAui,
  useAuiState,
} from "@assistant-ui/react";
import { ArrowDown } from "lucide-react";
import { type ComponentPropsWithoutRef, useEffect, useLayoutEffect, useRef } from "react";

import { cn } from "@/lib/utils";
import { SessionGoalHeaderContainer } from "@/systems/session/components/goal/session-goal-header-container";
import { SessionGoalCommandErrorNotice } from "@/systems/session/components/goal/session-goal-command-error-notice";
import { SessionLoadOlderButton } from "@/systems/session/components/session-load-older-button";
import { useSessionTranscriptThreadState } from "@/systems/session";
import type { SessionFailurePayload, SessionState } from "@/systems/session";
import { isGoalCommandFailureGuidance } from "@/systems/session/lib/session-goal-chat-transport";
import {
  recordSessionDebugEvent,
  SESSION_DEBUG_EVENTS,
} from "@/systems/session/lib/session-observability";
import { SessionComposer, type SessionComposerProps } from "./session-composer";
import {
  SESSION_THREAD_CONTENT_INSET_DEFAULT,
  ThreadContentRail,
  type SessionThreadContentInset,
} from "./session-thread-content-rail";
import { threadProviderIdentity } from "./hooks/use-thread-provider-identity";
import { useThreadScrollController } from "./hooks/use-thread-scroll-controller";
import { useSessionComposerState } from "./hooks/use-session-composer-state";
import { MessageActions } from "./message-actions";
import { SessionComposerPrefillProvider } from "./session-composer-prefill-context";
import { SessionDataEventCard, SessionMessageText } from "./session-message-parts";
import { formatMessageError } from "./session-thread-error";
import { SessionThreadErrorBoundary } from "./session-thread-error-boundary";
import { AssistantMessageTimeline } from "./session-timeline-render";
import { ThreadStatePane } from "./session-thread-states";

const EMPTY_QUEUED_PROMPTS: NonNullable<SessionComposerProps["queuedPrompts"]> = [];

interface SessionThreadProps extends SessionComposerProps {
  sessionId: string;
  agentName: string;
  workspaceId?: string;
  acpSessionId?: string;
  sessionState?: SessionState;
  failure?: SessionFailurePayload | null;
}

function SessionTextPart({ text, state }: { text: string; state?: { type: string } }) {
  return <SessionMessageText text={text} streaming={state?.type === "running"} />;
}

function SessionDataPart(part: DataMessagePartProps<unknown>) {
  return <SessionDataEventCard name={part.name} data={part.data} />;
}

function SessionMessageErrorNotice() {
  const error = useAuiState(state => {
    const status = state.message.status;
    if (status?.type !== "incomplete" || status.reason !== "error") {
      return null;
    }
    return formatMessageError(status.error);
  });

  if (error === null || isGoalCommandFailureGuidance(error)) {
    return null;
  }

  return (
    <div
      role="alert"
      data-testid="session-message-error"
      className={cn(
        "rounded-md border px-3 py-2 text-small-body",
        "border-danger/30 bg-danger/8",
        "text-danger"
      )}
    >
      {error}
    </div>
  );
}

function UserMessage() {
  return (
    <MessagePrimitive.Root className="group/message flex w-full min-w-0 justify-end pb-4 pt-1">
      <div className="flex min-w-0 max-w-[min(80%,42rem)] flex-col items-end gap-1">
        <div className={cn("w-full rounded-xl border px-4 py-3", "border-line bg-canvas-soft")}>
          <MessagePrimitive.Parts
            components={{
              Text: ({ text, status }: TextMessagePartProps) => (
                <SessionTextPart text={text} state={status} />
              ),
              data: {
                Fallback: SessionDataPart,
              },
            }}
          />
        </div>
        <MessageActions align="end" copyLabel="Copy message" testId="user-message-actions" />
      </div>
    </MessagePrimitive.Root>
  );
}

function AssistantMessage() {
  const isRunning = useAuiState(state => {
    const status = state.message.status;
    return status?.type === "running" || status?.type === "incomplete";
  });

  return (
    <MessagePrimitive.Root
      className={cn("group/message flex w-full min-w-0 pt-1", isRunning ? "pb-2" : "pb-4")}
    >
      <div className="flex min-w-0 flex-1 flex-col gap-3">
        <SessionThreadErrorBoundary>
          <AssistantMessageTimeline />
        </SessionThreadErrorBoundary>
        <SessionMessageErrorNotice />
        <MessageActions
          align="start"
          copyLabel="Copy message"
          goalPrefill
          testId="assistant-message-actions"
        />
      </div>
    </MessagePrimitive.Root>
  );
}

type ThreadViewportProps = ComponentPropsWithoutRef<typeof ThreadPrimitive.Viewport>;

export function ScrollToBottomPill({
  visible,
  onClick,
}: {
  visible: boolean;
  onClick: () => void;
}) {
  // Floating scroll-to-bottom affordance: a neutral `size-8` disc (no
  // glass/backdrop-blur) that fades + drifts in with the shared disclosure
  // motion and stays mounted so its exit animates too.
  return (
    <div
      className={cn(
        "pointer-events-none absolute inset-x-0 bottom-3 z-20 flex justify-center",
        "transition-[transform,opacity] duration-base ease-out motion-reduce:transition-none",
        visible ? "translate-y-0 opacity-100" : "translate-y-1 opacity-0"
      )}
      aria-hidden={!visible}
    >
      <button
        type="button"
        data-testid="scroll-to-bottom-pill"
        data-visible={visible}
        aria-label="Scroll to latest"
        onClick={onClick}
        tabIndex={visible ? 0 : -1}
        className={cn(
          "flex size-8 items-center justify-center rounded-full",
          "border border-line bg-canvas-soft text-muted shadow-[var(--shadow-overlay)]",
          "transition-colors hover:bg-hover hover:text-fg",
          "focus-visible:outline-none focus-visible:shadow-focus-ring",
          visible ? "pointer-events-auto" : "pointer-events-none"
        )}
      >
        <ArrowDown className="size-3.5" aria-hidden="true" />
      </button>
    </div>
  );
}

function ThreadViewport({
  agentName,
  sessionId,
  isSessionRunning,
  contentInset,
  sessionState,
  failure,
  startupFailed,
  className,
  ...props
}: ThreadViewportProps & {
  agentName: string;
  sessionId: string;
  isSessionRunning: boolean;
  contentInset: SessionThreadContentInset;
  sessionState?: SessionState;
  failure?: SessionFailurePayload | null;
  startupFailed: boolean;
}) {
  const viewportRef = useRef<HTMLDivElement | null>(null);
  const {
    messages: transcriptMessages,
    status: transcriptStatus,
    error: transcriptError,
    hasOlder,
    isFetchingOlder,
    loadOlder,
    retry: retryTranscript,
  } = useSessionTranscriptThreadState();
  const olderAnchorRef = useRef<{ messageId: string; offsetTop: number } | null>(null);
  const messageCount = transcriptMessages.length;
  const { showScrollToBottom, scrollToEnd } = useThreadScrollController(
    viewportRef,
    transcriptMessages
  );
  const loadOlderWithAnchor = () => {
    const viewport = viewportRef.current;
    if (viewport) {
      const viewportTop = viewport.getBoundingClientRect().top;
      const messageRows = [...viewport.querySelectorAll<HTMLElement>("[data-message-id]")];
      const anchorRow =
        messageRows.find(row => row.getBoundingClientRect().bottom >= viewportTop) ??
        messageRows[0];
      const messageId = anchorRow?.dataset.messageId;
      olderAnchorRef.current =
        anchorRow && messageId
          ? { messageId, offsetTop: anchorRow.getBoundingClientRect().top - viewportTop }
          : null;
    }
    loadOlder();
  };

  useLayoutEffect(() => {
    const viewport = viewportRef.current;
    const anchor = olderAnchorRef.current;
    if (!viewport || !anchor || isFetchingOlder) return;
    const anchorRow = [...viewport.querySelectorAll<HTMLElement>("[data-message-id]")].find(
      row => row.dataset.messageId === anchor.messageId
    );
    if (anchorRow) {
      const nextOffset =
        anchorRow.getBoundingClientRect().top - viewport.getBoundingClientRect().top;
      viewport.scrollTop += nextOffset - anchor.offsetTop;
    }
    olderAnchorRef.current = null;
  }, [isFetchingOlder, messageCount]);

  return (
    <div className="relative flex min-h-0 min-w-0 flex-1 flex-col">
      <ThreadPrimitive.Viewport
        {...props}
        ref={viewportRef}
        className={cn("min-h-0 flex-1 overflow-y-auto", className)}
        data-testid="chat-view"
      >
        <ThreadContentRail inset={contentInset} className="min-h-full">
          {hasOlder || isFetchingOlder ? (
            <SessionLoadOlderButton isLoading={isFetchingOlder} onClick={loadOlderWithAnchor} />
          ) : null}
          <ThreadMessages
            agentName={agentName}
            sessionId={sessionId}
            isSessionRunning={isSessionRunning}
            messageCount={messageCount}
            transcriptStatus={transcriptStatus}
            transcriptError={transcriptError}
            retryTranscript={retryTranscript}
            transcriptMessages={transcriptMessages}
            sessionState={sessionState}
            failure={failure}
            startupFailed={startupFailed}
          />
        </ThreadContentRail>
      </ThreadPrimitive.Viewport>
      <ScrollToBottomPill visible={showScrollToBottom} onClick={scrollToEnd} />
    </div>
  );
}

const SESSION_MESSAGE_COMPONENTS = {
  UserMessage,
  AssistantMessage,
};

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
          <ThreadPrimitive.MessageByIndex index={index} components={SESSION_MESSAGE_COMPONENTS} />
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

  const transcriptIdentity = threadProviderIdentity(transcriptMessages);

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
      />
    );
  }

  // `ReadonlyThreadProvider` applies changed `messages` in a passive effect, but
  // appended/removed message ids change which indexes are valid immediately.
  // Keying only on the id sequence rebuilds that provider with the correct core
  // size synchronously while leaving the outer virtualizer and its scroll machine
  // alive across the remount.
  return (
    <ReadonlyThreadProvider key={transcriptIdentity} messages={transcriptMessages}>
      <ThreadMessageRows messageCount={messageCount} transcriptMessages={transcriptMessages} />
    </ReadonlyThreadProvider>
  );
}

export function SessionThread({
  sessionId,
  agentName,
  workspaceId,
  canPrompt,
  onCancelPrompt,
  onQueuePrompt,
  onInterruptPrompt,
  onSteerPrompt,
  isBusyInputPending = false,
  isSessionRunning = false,
  allowBusyInput = true,
  queuedPrompts = EMPTY_QUEUED_PROMPTS,
  onRemoveQueuedPrompt,
  onSteerQueuedPrompt,
  contentInset = SESSION_THREAD_CONTENT_INSET_DEFAULT,
  acpSessionId,
  sessionState,
  failure,
}: SessionThreadProps) {
  const aui = useAui();
  const composerState = useSessionComposerState(sessionId);
  const startupFailed =
    sessionState === "stopped" && Boolean(failure) && !acpSessionId?.trim().length;
  const lifecycleCanPrompt = canPrompt && sessionState !== "starting" && !startupFailed;
  const handleCancelPrompt = () => {
    aui.thread().cancelRun();
    onCancelPrompt();
  };
  return (
    <ThreadPrimitive.Root className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
      <SessionComposerPrefillProvider setComposerText={composerState.prefillComposer}>
        {workspaceId && sessionState !== "starting" ? (
          <SessionGoalHeaderContainer
            onPrefillComposer={composerState.prefillComposer}
            sessionId={sessionId}
            workspaceId={workspaceId}
          />
        ) : null}
        <ThreadViewport
          agentName={agentName}
          sessionId={sessionId}
          isSessionRunning={isSessionRunning}
          contentInset={contentInset}
          sessionState={sessionState}
          failure={failure}
          startupFailed={startupFailed}
        />
        <ThreadContentRail inset={contentInset} className="pt-2">
          <SessionGoalCommandErrorNotice sessionId={sessionId} />
        </ThreadContentRail>
        <SessionComposer
          composerState={composerState}
          contentInset={contentInset}
          canPrompt={lifecycleCanPrompt}
          onCancelPrompt={handleCancelPrompt}
          onQueuePrompt={onQueuePrompt}
          onInterruptPrompt={onInterruptPrompt}
          onSteerPrompt={onSteerPrompt}
          isBusyInputPending={isBusyInputPending}
          isSessionRunning={isSessionRunning}
          allowBusyInput={allowBusyInput}
          queuedPrompts={queuedPrompts}
          onRemoveQueuedPrompt={onRemoveQueuedPrompt}
          onSteerQueuedPrompt={onSteerQueuedPrompt}
          inactivePlaceholder={
            sessionState === "starting"
              ? "Session is starting…"
              : startupFailed
                ? "Session failed to start"
                : undefined
          }
        />
      </SessionComposerPrefillProvider>
    </ThreadPrimitive.Root>
  );
}
