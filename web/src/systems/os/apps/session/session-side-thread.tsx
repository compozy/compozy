import { AuiConfig, AuiProvider } from "@assistant-ui/react";

import { useSessionPageControls } from "@/hooks/routes/use-session-page-controls";
import {
  canPromptSession,
  getSessionPromptRuntimeSnapshot,
  SessionChatRuntimeProvider,
  SessionPromptRuntimeProvider,
  SessionPromptRuntimeSelector,
  SessionSideThreadBoundary,
  type SessionPayload,
  useSessionCommands,
  useSessionPromptRuntimeContext,
} from "@/systems/session";
import { SessionThread } from "./session-thread-lazy";

/**
 * A side chat's own conversation inside the side panel: its own runtime, prompt controls and
 * command catalog, the same composer as any session (`_uiux.md` S3). It is a hidden session, not a
 * window, so it hosts no `/side` interceptor: the daemon refuses a side chat inside a side chat.
 */
function SessionSideThreadContent({
  side,
  workspaceId,
}: {
  side: SessionPayload;
  workspaceId: string;
}) {
  const promptRuntime = useSessionPromptRuntimeContext();
  const controls = useSessionPageControls(side.id, side, {
    getRuntimeSnapshot: () => getSessionPromptRuntimeSnapshot(promptRuntime),
    liveDataEnabled: true,
    workspaceId: side.workspace_id,
  });
  const commands = useSessionCommands(workspaceId, side.id, { enabled: true });
  return (
    <SessionThread
      liveDataEnabled
      sessionId={side.id}
      workspaceId={workspaceId}
      agentName={side.agent_name}
      acpSessionId={side.runtime.acp_session_id}
      sessionState={side.state}
      failure={side.failure}
      statusSession={side}
      stopCompletionNote={controls.stopCompletionNote}
      canPrompt={controls.canPrompt}
      onCancelPrompt={controls.handleCancelPrompt}
      onQueuePrompt={controls.handleQueuePrompt}
      onInterruptPrompt={controls.handleInterruptPrompt}
      onSteerPrompt={controls.handleSteerPrompt}
      isBusyInputPending={controls.isBusyInputPending}
      isSessionRunning={controls.isSessionRunning}
      stopPhase={controls.stopPhase}
      allowBusyInput={controls.allowBusyInput}
      busyInputDefaultMode={controls.busyInputDefaultMode}
      busyInputSteerDelivery={controls.busyInputSteerDelivery}
      queuedPrompts={controls.queuedPrompts}
      onRemoveQueuedPrompt={controls.handleRemoveQueuedPrompt}
      onReplaceQueuedPrompt={controls.handleReplaceQueuedPrompt}
      onSteerQueuedPrompt={controls.handleSteerQueuedPrompt}
      onClearQueue={controls.handleClearQueue}
      queueCap={controls.queueCap}
      unconfirmedSends={controls.unconfirmedSends}
      onRetryUnconfirmedSend={controls.handleRetryUnconfirmedSend}
      onDiscardUnconfirmedSend={controls.handleDiscardUnconfirmedSend}
      messageTrail={false}
      // The side chat's own runtime: a change applies to this side chat only (B-011).
      runtimeControl={<SessionPromptRuntimeSelector canPrompt={controls.canPrompt} />}
      commandCatalog={commands.catalog}
      commandCatalogStatus={commands.isPending ? "loading" : "ready"}
      onCommandCatalogOpen={() => {
        void commands.refetch();
      }}
    />
  );
}

/**
 * The side chat transcript and composer. A fresh assistant-ui scope keeps it apart from the main
 * thread; the boundary drops the window's fork, continue, clear and report actions (S6).
 */
export function SessionSideThread({
  side,
  workspaceId,
}: {
  side: SessionPayload;
  workspaceId: string;
}) {
  return (
    <AuiProvider extends={null} config={AuiConfig({})}>
      <SessionSideThreadBoundary>
        <SessionPromptRuntimeProvider canPrompt={canPromptSession(side)} session={side}>
          <SessionChatRuntimeProvider
            key={side.id}
            sessionId={side.id}
            workspaceId={workspaceId}
            liveTailEnabled
          >
            <SessionSideThreadContent side={side} workspaceId={workspaceId} />
          </SessionChatRuntimeProvider>
        </SessionPromptRuntimeProvider>
      </SessionSideThreadBoundary>
    </AuiProvider>
  );
}
