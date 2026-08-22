import { lazy, Suspense, use, useRef } from "react";
import { toast } from "sonner";

import { loadSessionThread } from "./session-window-module-loader";
import { useSessionWindowController } from "./use-session-window-controller";
import { WorktreeDialogActionsContext } from "../../contexts/worktree-dialog-actions-context";
import { sessionPromptCapability } from "@/systems/session/lib/session-prompt-capability";
import {
  type SessionPayload,
  SessionEnvironmentControl,
  type SessionEnvironmentControlHandle,
  SessionPromptRuntimeSelector,
  SessionResumeFailure,
  SessionRuntimeRecoveryNotice,
  SessionSidebar,
  hasUnrecoverableRuntime,
  useCreateSession,
} from "@/systems/session";

const SessionThread = lazy(() =>
  loadSessionThread().then(module => ({ default: module.SessionThread }))
);
const SessionClearDialog = lazy(() =>
  import("./session-window-dialogs").then(module => ({ default: module.SessionClearDialog }))
);
const SessionDeleteDialog = lazy(() =>
  import("@/systems/session/components/session-delete-dialog").then(module => ({
    default: module.SessionDeleteDialog,
  }))
);
const SessionRenameDialog = lazy(() =>
  import("@/systems/session/components/session-rename-dialog").then(module => ({
    default: module.SessionRenameDialog,
  }))
);
const SessionInspector = lazy(() =>
  import("@/systems/session/components/session-inspector").then(module => ({
    default: module.SessionInspector,
  }))
);

function workingStartedAt(value: string | null | undefined): number | undefined {
  if (!value) return undefined;
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? parsed : undefined;
}

export function SessionWindowContent({
  windowId,
  agentName,
  sessionId,
  session,
  workspaceId,
  onDeleteSuccess,
  liveDataEnabled,
}: {
  windowId: string;
  agentName: string;
  sessionId: string;
  session: SessionPayload;
  workspaceId: string;
  onDeleteSuccess: () => void;
  liveDataEnabled: boolean;
}) {
  const worktreeDialogs = use(WorktreeDialogActionsContext);
  const page = useSessionWindowController({
    windowId,
    sessionId,
    workspaceId,
    session,
    onDeleteSuccess,
    liveDataEnabled,
    onOpenWorktreeContext: worktreeDialogs?.requestContextWorktree,
    onResolveMissingWorktree: worktreeDialogs?.requestResolveMissingWorktree,
  });
  const {
    controls,
    inspector,
    sidebar,
    inspectorMemory,
    inspectorUsage,
    sessionVault,
    deleteDialog,
    renameDialog,
    clearDialog,
    commandCatalog,
    commandCatalogStatus,
    refreshCommandCatalog,
    promptRuntimeSnapshot,
    worktreeBinding,
  } = page;
  const environmentControl = useRef<SessionEnvironmentControlHandle>(null);
  const forkSession = useCreateSession();
  const promptImageCapability = sessionPromptCapability(
    session,
    "prompt_image",
    promptRuntimeSnapshot
  );
  const promptEmbeddedContextCapability = sessionPromptCapability(
    session,
    "prompt_embedded_context",
    promptRuntimeSnapshot
  );

  const handleForkDeadSession = () => {
    forkSession.mutate(
      {
        agent_name: session.agent_name,
        parent_session_id: sessionId,
        workspace: workspaceId,
      },
      {
        onError: error => {
          toast.error(error instanceof Error ? error.message : "Failed to fork session.");
        },
        onSuccess: sidebar.onSelectSession,
      }
    );
  };

  return (
    <div className="flex min-h-0 min-w-0 flex-1 overflow-hidden">
      <SessionSidebar
        open={sidebar.open}
        sessions={sidebar.sessions}
        disconnected={sidebar.disconnected}
        collapsedThreadIds={sidebar.collapsedThreadIds}
        view={sidebar.view}
        currentSessionId={sessionId}
        onToggleThread={sidebar.onToggleThread}
        onSelectSession={sidebar.onSelectSession}
        onNewSession={sidebar.onNewSession}
        sessionActions={sidebar.sessionActions}
      />
      <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        {session.runtime.status === "recovering" ? (
          <SessionRuntimeRecoveryNotice
            attempt={session.runtime.recovery?.attempt}
            maxAttempts={session.runtime.recovery?.max_attempts}
          />
        ) : controls.resumeFailure ? (
          <SessionResumeFailure
            agentName={controls.resumeFailure.providerUnavailable?.agentName ?? agentName}
            isRetrying={controls.isResuming}
            message={controls.resumeFailure.message}
            missingProvider={controls.resumeFailure.providerUnavailable?.missingProvider ?? null}
            onDismiss={controls.handleDismissResumeFailure}
            onRetry={controls.handleResume}
            sessionId={sessionId}
          />
        ) : hasUnrecoverableRuntime(session) ? (
          <SessionResumeFailure
            agentName={agentName}
            isRetrying={forkSession.isPending}
            message="This provider runtime cannot be resumed. Its original transcript and failure details remain available here."
            missingProvider={null}
            onDismiss={() => undefined}
            onRetry={handleForkDeadSession}
            retryLabel="Fork into a new session"
            sessionId={sessionId}
            showDismiss={false}
            title="Runtime unavailable"
          />
        ) : null}
        <SessionThread
          liveDataEnabled={liveDataEnabled}
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
          environmentControl={
            <SessionEnvironmentControl
              ref={environmentControl}
              binding={worktreeBinding}
              sessionId={sessionId}
              sessionTitle={session.name ?? sessionId}
              workspaceId={workspaceId}
              workspaceName={session.workspace_path ?? workspaceId}
            />
          }
          commandCatalog={commandCatalog}
          commandCatalogStatus={commandCatalogStatus}
          onCommandCatalogOpen={refreshCommandCatalog}
          onCommandAction={token => {
            if (token !== "/worktree") return false;
            environmentControl.current?.openFork();
            return true;
          }}
          promptImageCapability={promptImageCapability}
          promptEmbeddedContextCapability={promptEmbeddedContextCapability}
        />
      </div>
      {inspector.open ? (
        <Suspense fallback={null}>
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
        </Suspense>
      ) : null}
      {deleteDialog.open ? (
        <Suspense fallback={null}>
          <SessionDeleteDialog
            open
            onOpenChange={deleteDialog.setOpen}
            session={session}
            isDeleting={controls.isDeleting}
            onConfirm={deleteDialog.confirmDelete}
          />
        </Suspense>
      ) : null}
      {renameDialog.open ? (
        <Suspense fallback={null}>
          <SessionRenameDialog
            open
            onOpenChange={renameDialog.setOpen}
            session={session}
            isRenaming={controls.isRenaming}
            requestError={renameDialog.error}
            onConfirm={renameDialog.confirmRename}
          />
        </Suspense>
      ) : null}
      {sidebar.rowDeleteDialog.open && sidebar.rowDeleteDialog.session ? (
        <Suspense fallback={null}>
          <SessionDeleteDialog
            open
            onOpenChange={sidebar.rowDeleteDialog.onOpenChange}
            session={sidebar.rowDeleteDialog.session}
            isDeleting={sidebar.rowDeleteDialog.isDeleting}
            onConfirm={sidebar.rowDeleteDialog.onConfirm}
          />
        </Suspense>
      ) : null}
      {sidebar.rowRenameDialog.open && sidebar.rowRenameDialog.session ? (
        <Suspense fallback={null}>
          <SessionRenameDialog
            open
            onOpenChange={sidebar.rowRenameDialog.onOpenChange}
            session={sidebar.rowRenameDialog.session}
            isRenaming={sidebar.rowRenameDialog.isRenaming}
            onConfirm={sidebar.rowRenameDialog.onConfirm}
          />
        </Suspense>
      ) : null}
      {clearDialog.open ? (
        <Suspense fallback={null}>
          <SessionClearDialog
            open
            onOpenChange={clearDialog.setOpen}
            isClearing={controls.isClearing}
            onConfirm={clearDialog.confirmClear}
          />
        </Suspense>
      ) : null}
    </div>
  );
}
