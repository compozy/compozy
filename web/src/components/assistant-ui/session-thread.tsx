import { ThreadPrimitive, useAui } from "@assistant-ui/react";

import { SessionGoalHeaderContainer } from "@/systems/session/components/goal/session-goal-header-container";
import { SessionGoalCommandErrorNotice } from "@/systems/session/components/goal/session-goal-command-error-notice";

import { useSessionComposerState } from "./hooks/use-session-composer-state";
import { usePrefersReducedMotion } from "./hooks/use-prefers-reduced-motion";
import { SessionComposer, type SessionComposerProps } from "./session-composer";
import { SessionComposerPrefillProvider } from "./session-composer-prefill-context";
import { useSessionPromptDispatch } from "./hooks/use-session-prompt-dispatch";
import {
  SESSION_THREAD_CONTENT_INSET_DEFAULT,
  ThreadContentRail,
} from "./session-thread-content-rail";
import { ThreadViewport } from "./session-thread-viewport";
import { WorkingIndicator } from "./session-working-row";
import {
  SessionDecisionDock,
  useSessionFirstPrompt,
  type SessionFailurePayload,
  type SessionState,
} from "@/systems/session";

const EMPTY_QUEUED_PROMPTS: NonNullable<SessionComposerProps["queuedPrompts"]> = [];

interface SessionThreadProps extends SessionComposerProps {
  sessionId: string;
  agentName: string;
  workspaceId?: string;
  acpSessionId?: string;
  sessionState?: SessionState;
  failure?: SessionFailurePayload | null;
  workingStartedAt?: number;
  liveDataEnabled?: boolean;
}

/**
 * The session surface composition root: pinned goal zone above the transcript
 * scroller, the viewport itself, and the composer zone (goal-command notice,
 * decision dock, composer) below it.
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
  allowBusyInput = true,
  busyInputFenceAvailable = true,
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
}: SessionThreadProps) {
  const aui = useAui();
  const reducedMotion = usePrefersReducedMotion();
  const composerState = useSessionComposerState(sessionId);
  const promptDispatch = useSessionPromptDispatch();
  const promptDispatchPending = promptDispatch.pending;
  const renderedComposerState = promptDispatch.canceled
    ? { ...composerState, isRunning: false }
    : composerState;
  const runtimeRunning =
    isSessionRunning || renderedComposerState.isRunning || promptDispatchPending;
  const startupFailed =
    sessionState === "stopped" && Boolean(failure) && !acpSessionId?.trim().length;
  const lifecycleCanPrompt = canPrompt && sessionState !== "starting" && !startupFailed;
  useSessionFirstPrompt({ canPrompt: lifecycleCanPrompt, sessionId });
  const handleCancelPrompt = () => {
    aui.thread.cancelRun();
    promptDispatch.cancelPending();
    onCancelPrompt();
  };
  return (
    <ThreadPrimitive.Root className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
      <SessionComposerPrefillProvider setComposerText={composerState.prefillComposer}>
        {workspaceId && sessionState !== "starting" ? (
          <ThreadContentRail inset={contentInset}>
            <SessionGoalHeaderContainer
              enabled={liveDataEnabled}
              onPrefillComposer={composerState.prefillComposer}
              sessionId={sessionId}
              workspaceId={workspaceId}
            />
          </ThreadContentRail>
        ) : null}
        <ThreadViewport
          agentName={agentName}
          sessionId={sessionId}
          isSessionRunning={runtimeRunning}
          contentInset={contentInset}
          sessionState={sessionState}
          failure={failure}
          startupFailed={startupFailed}
        />
        <ThreadContentRail inset={contentInset} className="pt-2">
          <SessionGoalCommandErrorNotice sessionId={sessionId} />
          {runtimeRunning ? (
            <WorkingIndicator
              liveDataEnabled={liveDataEnabled}
              startedAt={workingStartedAt}
              reducedMotion={reducedMotion}
            />
          ) : null}
        </ThreadContentRail>
        <SessionComposer
          composerState={renderedComposerState}
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
          canPrompt={lifecycleCanPrompt}
          onCancelPrompt={handleCancelPrompt}
          onQueuePrompt={onQueuePrompt}
          onInterruptPrompt={onInterruptPrompt}
          onSteerPrompt={onSteerPrompt}
          isBusyInputPending={isBusyInputPending}
          isSessionRunning={runtimeRunning}
          allowBusyInput={allowBusyInput}
          busyInputFenceAvailable={busyInputFenceAvailable}
          queuedPrompts={queuedPrompts}
          onRemoveQueuedPrompt={onRemoveQueuedPrompt}
          onReplaceQueuedPrompt={onReplaceQueuedPrompt}
          onSteerQueuedPrompt={onSteerQueuedPrompt}
          inactivePlaceholder={
            sessionState === "starting"
              ? "Session is starting…"
              : startupFailed
                ? "Session failed to start"
                : undefined
          }
          runtimeControl={runtimeControl}
          environmentControl={environmentControl}
          commandCatalog={commandCatalog}
          commandCatalogStatus={commandCatalogStatus}
          onCommandCatalogOpen={onCommandCatalogOpen}
          onCommandAction={onCommandAction}
        />
      </SessionComposerPrefillProvider>
    </ThreadPrimitive.Root>
  );
}
