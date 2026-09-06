import { ThreadPrimitive, useAuiState } from "@assistant-ui/react";
import { useStore } from "@xstate/store-react";

import { SessionGoalHeaderContainer } from "@/systems/session/components/goal/session-goal-header-container";
import { SessionGoalCommandErrorNotice } from "@/systems/session/components/goal/session-goal-command-error-notice";

import { SessionComposer, type SessionComposerProps } from "./session-composer";
import { SessionComposerPrefillProvider } from "./session-composer-prefill-context";
import { SessionThreadLiveDataProvider } from "./session-thread-live-data-provider";
import { SessionThreadReadOnlyProvider } from "./session-thread-read-only-provider";
import { useSessionThreadState } from "./hooks/use-session-thread-state";
import { ThreadContentRail } from "./session-thread-content-rail";
import { SESSION_THREAD_CONTENT_INSET_DEFAULT } from "./session-thread-content-rail-constants";
import {
  SessionThreadStatusRow,
  type SessionThreadStatusSession,
} from "./session-thread-status-row";
import { ThreadViewport } from "./session-thread-viewport";
import { ThreadScrollStoreContext } from "./hooks/thread-scroll-context";
import { SteerProvenanceProvider } from "@/systems/session/lib/steer-provenance-context";
import { SessionTurnOutcomesProvider } from "@/systems/session/lib/session-turn-outcomes-context";
import { threadScrollLogic } from "./hooks/thread-scroll-store";
import {
  SessionDecisionDock,
  SessionTerminalQuoteSlot,
  SessionTransportFailureNotice,
  useSessionTranscriptThreadMessages,
  SessionTransportHistoryResetNotice,
  type SessionBusyInputHandler,
  type SessionFailurePayload,
  type SessionQuietWarning,
  type SessionState,
} from "@/systems/session";

const EMPTY_QUEUED_PROMPTS: NonNullable<SessionComposerProps["queuedPrompts"]> = [];

function LoadedTransportFailureNotice() {
  const transcript = useSessionTranscriptThreadMessages();
  const hasLiveMessages = useAuiState(state => state.thread.messages.length > 0);
  return transcript.length > 0 || hasLiveMessages ? <SessionTransportFailureNotice /> : null;
}

interface SessionThreadProps extends Omit<
  SessionComposerProps,
  "onCancelPrompt" | "quoteSlot" | "sessionId"
> {
  /**
   * Absent in read-only mode, where the composer that owns cancellation is not
   * rendered — there is no run this surface could cancel.
   */
  onCancelPrompt?: SessionComposerProps["onCancelPrompt"];
  sessionId: string;
  agentName: string;
  workspaceId?: string;
  acpSessionId?: string;
  sessionState?: SessionState;
  failure?: SessionFailurePayload | null;
  /**
   * The session resource the status row reads (S3): durable turn start,
   * current tool, work signals (children), pending decisions. Absent for
   * surfaces without a session (read-only rows, stories): no status row.
   */
  statusSession?: SessionThreadStatusSession | null;
  /** The daemon answered the operator's turn stop with `nothing-in-flight` (US-009.EC-2). */
  stopCompletionNote?: boolean;
  /**
   * The daemon's quiet episode (US-014.EC-2). While present the status row
   * reads the quiet clock instead of "Working for": the daemon has no work
   * signal for this session, so a working timer would contradict it.
   */
  quietWarning?: SessionQuietWarning | null;
  liveDataEnabled?: boolean;
  /**
   * Renders the transcript with no way to act on it.
   *
   * The composer and the decision dock are omitted rather than disabled: this
   * mode exists for a session the operator is allowed to read but not to touch,
   * and a greyed-out send button still asserts that sending is this surface's
   * business. Nothing here is a permission check — the daemon owns that.
   */
  readOnly?: boolean;
}

/**
 * The session surface composition root: pinned goal zone above the transcript
 * scroller, the viewport itself, and the composer zone (goal-command notice,
 * decision dock, and composer with the quote chip in its stack) below it.
 */
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
  stopPhase = "idle",
  allowBusyInput = true,
  busyInputDefaultMode,
  busyInputSteerDelivery,
  queuedPrompts = EMPTY_QUEUED_PROMPTS,
  onRemoveQueuedPrompt,
  onReplaceQueuedPrompt,
  onSteerQueuedPrompt,
  onClearQueue,
  queueCap,
  unconfirmedSends,
  onRetryUnconfirmedSend,
  onDiscardUnconfirmedSend,
  contentInset = SESSION_THREAD_CONTENT_INSET_DEFAULT,
  acpSessionId,
  sessionState,
  failure,
  statusSession = null,
  stopCompletionNote = false,
  quietWarning = null,
  liveDataEnabled = true,
  runtimeControl,
  environmentControl,
  commandCatalog,
  commandCatalogStatus,
  onCommandCatalogOpen,
  onCommandAction,
  promptImageCapability = "unknown",
  promptEmbeddedContextCapability = "unknown",
  readOnly = false,
}: SessionThreadProps) {
  const thread = useSessionThreadState({
    acpSessionId,
    canPrompt,
    isSessionRunning,
    onCancelPrompt,
    readOnly,
    sessionFailure: failure,
    sessionId,
    sessionState,
  });
  // One scroll owner for the viewport and the rows that change height (ADR-007).
  const scrollStore = useStore(threadScrollLogic);
  // Steering an already-streaming turn snaps the viewport to the end instead
  // of animating (US-024.EC-3); a steer on an idle turn anchors like a send.
  const handleSteerPrompt: SessionBusyInputHandler | undefined = onSteerPrompt
    ? draft => {
        scrollStore.trigger.steerRequested({ streaming: thread.runtimeRunning });
        return onSteerPrompt(draft);
      }
    : undefined;
  return (
    <ThreadPrimitive.Root
      className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden"
      data-thread-root=""
    >
      <ThreadScrollStoreContext.Provider value={scrollStore}>
        <SessionTurnOutcomesProvider>
          <SteerProvenanceProvider>
            <SessionThreadReadOnlyProvider readOnly={readOnly}>
              <SessionThreadLiveDataProvider liveDataEnabled={liveDataEnabled}>
                <SessionComposerPrefillProvider setComposerText={thread.composer.prefillComposer}>
                  {workspaceId && sessionState !== "starting" ? (
                    <ThreadContentRail inset={contentInset}>
                      <SessionGoalHeaderContainer
                        enabled={liveDataEnabled}
                        onPrefillComposer={readOnly ? undefined : thread.composer.prefillComposer}
                        sessionId={sessionId}
                        workspaceId={workspaceId}
                      />
                    </ThreadContentRail>
                  ) : null}
                  <ThreadViewport
                    agentName={agentName}
                    sessionId={sessionId}
                    isSessionRunning={thread.runtimeRunning}
                    contentInset={contentInset}
                    sessionState={sessionState}
                    failure={failure}
                    startupFailed={thread.startupFailed}
                  />
                  <ThreadContentRail inset={contentInset} className="pt-2">
                    <SessionGoalCommandErrorNotice sessionId={sessionId} />
                    {/* End-of-transcript truth about the live view (S4): a stated history reset, or a dead stream with Try again. */}
                    <SessionTransportHistoryResetNotice />
                    <LoadedTransportFailureNotice />
                    <SessionThreadStatusRow
                      session={statusSession}
                      stopCompletionNote={stopCompletionNote}
                      stopping={stopPhase === "stopping" || sessionState === "stopping"}
                      running={thread.runtimeRunning}
                      quietWarning={quietWarning}
                      liveDataEnabled={liveDataEnabled}
                      reducedMotion={thread.reducedMotion}
                    />
                  </ThreadContentRail>
                  {readOnly ? null : (
                    <SessionComposer
                      sessionId={sessionId}
                      quoteSlot={
                        readOnly ? null : <SessionTerminalQuoteSlot sessionId={sessionId} />
                      }
                      composerState={thread.renderedComposer}
                      contentInset={contentInset}
                      decisionDock={
                        workspaceId ? (
                          <SessionDecisionDock
                            enabled={liveDataEnabled}
                            sessionId={sessionId}
                            workspaceId={workspaceId}
                          />
                        ) : undefined
                      }
                      canPrompt={thread.lifecycleCanPrompt}
                      onCancelPrompt={thread.handleCancelPrompt}
                      onQueuePrompt={onQueuePrompt}
                      onInterruptPrompt={onInterruptPrompt}
                      onSteerPrompt={handleSteerPrompt}
                      isBusyInputPending={isBusyInputPending}
                      isSessionRunning={thread.runtimeRunning}
                      stopPhase={stopPhase}
                      allowBusyInput={allowBusyInput}
                      busyInputDefaultMode={busyInputDefaultMode}
                      busyInputSteerDelivery={busyInputSteerDelivery}
                      queuedPrompts={queuedPrompts}
                      onRemoveQueuedPrompt={onRemoveQueuedPrompt}
                      onReplaceQueuedPrompt={onReplaceQueuedPrompt}
                      onSteerQueuedPrompt={onSteerQueuedPrompt}
                      onClearQueue={onClearQueue}
                      queueCap={queueCap}
                      unconfirmedSends={unconfirmedSends}
                      onRetryUnconfirmedSend={onRetryUnconfirmedSend}
                      onDiscardUnconfirmedSend={onDiscardUnconfirmedSend}
                      inactivePlaceholder={
                        sessionState === "starting"
                          ? "Session is starting…"
                          : thread.startupFailed
                            ? "Session failed to start"
                            : undefined
                      }
                      runtimeControl={runtimeControl}
                      environmentControl={environmentControl}
                      commandCatalog={commandCatalog}
                      commandCatalogStatus={commandCatalogStatus}
                      onCommandCatalogOpen={onCommandCatalogOpen}
                      onCommandAction={onCommandAction}
                      promptImageCapability={promptImageCapability}
                      promptEmbeddedContextCapability={promptEmbeddedContextCapability}
                    />
                  )}
                </SessionComposerPrefillProvider>
              </SessionThreadLiveDataProvider>
            </SessionThreadReadOnlyProvider>
          </SteerProvenanceProvider>
        </SessionTurnOutcomesProvider>
      </ThreadScrollStoreContext.Provider>
    </ThreadPrimitive.Root>
  );
}
