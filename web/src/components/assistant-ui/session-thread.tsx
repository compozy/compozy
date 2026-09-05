import { ThreadPrimitive } from "@assistant-ui/react";

import { SessionGoalHeaderContainer } from "@/systems/session/components/goal/session-goal-header-container";
import { SessionGoalCommandErrorNotice } from "@/systems/session/components/goal/session-goal-command-error-notice";

import { SessionComposer, type SessionComposerProps } from "./session-composer";
import { SessionComposerPrefillProvider } from "./session-composer-prefill-context";
import { SessionThreadReadOnlyProvider } from "./session-thread-read-only-provider";
import { useSessionThreadState } from "./hooks/use-session-thread-state";
import { ThreadContentRail } from "./session-thread-content-rail";
import { SESSION_THREAD_CONTENT_INSET_DEFAULT } from "./session-thread-content-rail-constants";
import { ThreadViewport } from "./session-thread-viewport";
import { WorkingIndicator } from "./session-working-row";
import {
  SessionDecisionDock,
  SessionTerminalQuoteSlot,
  type SessionFailurePayload,
  type SessionState,
} from "@/systems/session";

const EMPTY_QUEUED_PROMPTS: NonNullable<SessionComposerProps["queuedPrompts"]> = [];

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
  workingStartedAt?: number;
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
  contentInset = SESSION_THREAD_CONTENT_INSET_DEFAULT,
  acpSessionId,
  sessionState,
  failure,
  workingStartedAt,
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
  return (
    <ThreadPrimitive.Root className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
      <SessionThreadReadOnlyProvider readOnly={readOnly}>
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
            {thread.runtimeRunning ? (
              <WorkingIndicator
                liveDataEnabled={liveDataEnabled}
                startedAt={workingStartedAt}
                reducedMotion={thread.reducedMotion}
              />
            ) : null}
          </ThreadContentRail>
          {readOnly ? null : (
            <SessionComposer
              sessionId={sessionId}
              quoteSlot={readOnly ? null : <SessionTerminalQuoteSlot sessionId={sessionId} />}
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
              onSteerPrompt={onSteerPrompt}
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
      </SessionThreadReadOnlyProvider>
    </ThreadPrimitive.Root>
  );
}
