import { SessionThread } from "@/components/assistant-ui/session-thread";
import {
  SessionInspector,
  SessionPromptRuntimeSelector,
  SessionResumeFailure,
  type SessionPayload,
} from "@/systems/session";

import { SessionClearDialog, SessionDeleteDialog } from "./session-window-dialogs";
import { useSessionWindowController } from "./use-session-window-controller";

function workingStartedAt(value: string | null | undefined): number | undefined {
  if (!value) return undefined;
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? parsed : undefined;
}

export function SessionWindowContent({
  agentName,
  sessionId,
  session,
  workspaceId,
  onDeleteSuccess,
}: {
  agentName: string;
  sessionId: string;
  session: SessionPayload;
  workspaceId: string;
  onDeleteSuccess: () => void;
}) {
  const page = useSessionWindowController({ sessionId, session, onDeleteSuccess });
  const {
    controls,
    inspector,
    inspectorMemory,
    inspectorUsage,
    sessionVault,
    deleteDialog,
    clearDialog,
  } = page;

  return (
    <div className="flex min-h-0 min-w-0 flex-1 overflow-hidden">
      <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        {controls.resumeFailure ? (
          <SessionResumeFailure
            agentName={controls.resumeFailure.providerUnavailable?.agentName ?? agentName}
            isRetrying={controls.isResuming}
            message={controls.resumeFailure.message}
            missingProvider={controls.resumeFailure.providerUnavailable?.missingProvider ?? null}
            onDismiss={controls.handleDismissResumeFailure}
            onRetry={controls.handleResume}
            sessionId={sessionId}
          />
        ) : null}
        <SessionThread
          sessionId={sessionId}
          workspaceId={workspaceId}
          agentName={agentName}
          acpSessionId={session.runtime.acp_session_id}
          sessionState={session.state}
          failure={session.failure}
          workingStartedAt={
            controls.isSessionRunning
              ? workingStartedAt(session.activity?.turn_started_at)
              : undefined
          }
          canPrompt={controls.canPrompt}
          onCancelPrompt={controls.handleStop}
          onQueuePrompt={controls.handleQueuePrompt}
          onInterruptPrompt={controls.handleInterruptPrompt}
          onSteerPrompt={controls.handleSteerPrompt}
          isBusyInputPending={controls.isBusyInputPending}
          isSessionRunning={controls.isSessionRunning}
          allowBusyInput={controls.allowBusyInput}
          busyInputFenceAvailable={Boolean(session.activity?.turn_id?.trim())}
          queuedPrompts={controls.queuedPrompts}
          onRemoveQueuedPrompt={controls.handleRemoveQueuedPrompt}
          onReplaceQueuedPrompt={controls.handleReplaceQueuedPrompt}
          onSteerQueuedPrompt={controls.handleSteerQueuedPrompt}
          runtimeControl={<SessionPromptRuntimeSelector canPrompt={controls.canPrompt} />}
        />
      </div>
      {inspector.open ? (
        <SessionInspector
          messages={controls.messages}
          sessionId={sessionId}
          usage={inspectorUsage}
          memory={inspectorMemory}
          vaultSecrets={sessionVault.data ?? []}
          vaultIsLoading={sessionVault.isLoading}
          vaultError={sessionVault.error}
          drawerOpen
          onDrawerOpenChange={open => {
            if (!open) {
              inspector.close();
            }
          }}
        />
      ) : null}
      <SessionDeleteDialog
        open={deleteDialog.open}
        onOpenChange={deleteDialog.setOpen}
        session={session}
        isDeleting={controls.isDeleting}
        onConfirm={deleteDialog.confirmDelete}
      />
      <SessionClearDialog
        open={clearDialog.open}
        onOpenChange={clearDialog.setOpen}
        isClearing={controls.isClearing}
        onConfirm={clearDialog.confirmClear}
      />
    </div>
  );
}
