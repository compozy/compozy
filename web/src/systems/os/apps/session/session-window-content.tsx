import { SessionContextControl } from "@/systems/session";
import { lazy, Suspense, use, useRef } from "react";
import { toast } from "sonner";

import { ThreadContentRail } from "@/components/assistant-ui/session-thread-content-rail";
import { SESSION_THREAD_CONTENT_INSET_DEFAULT } from "@/components/assistant-ui/session-thread-content-rail-constants";
import { SessionThread } from "./session-thread-lazy";
import { useSessionWindowController } from "./use-session-window-controller";
import { WorktreeDialogActionsContext } from "../../contexts/worktree-dialog-actions-context";
import { sessionPromptCapability } from "@/systems/session/lib/session-prompt-capability";
import {
  type SessionPayload,
  SessionEnvironmentControl,
  type SessionEnvironmentControlHandle,
  SessionPromptRuntimeSelector,
  type SessionQuietWarning,
  SessionQuietWarningNotice,
  SessionResumeFailure,
  SessionRuntimeRecoveryNotice,
  SessionSidebar,
  SessionStopAttentionNotice,
  hasUnrecoverableRuntime,
  sessionQuietWarning,
  useCreateSession,
} from "@/systems/session";

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

type SessionWindowControls = ReturnType<typeof useSessionWindowController>["controls"];

/**
 * The one session-level notice above the transcript, on the transcript's inset
 * rail so it aligns with the messages below instead of running edge to edge.
 * Precedence: a recovering runtime, then an unverified stop, then an attach
 * failure, then a runtime that can only be forked, then the daemon's quiet
 * warning (US-014.EC-2) — every actionable failure outranks a warning about
 * work that merely paused.
 */
type SessionWindowNoticeProps = {
  agentName: string;
  controls: SessionWindowControls;
  isForking: boolean;
  onFork: () => void;
  quietWarning: SessionQuietWarning | null;
  session: SessionPayload;
  sessionId: string;
};

function SessionWindowNotice(props: SessionWindowNoticeProps) {
  return (
    <ThreadContentRail
      data-testid="session-window-notice-rail"
      inset={SESSION_THREAD_CONTENT_INSET_DEFAULT}
    >
      <SessionWindowNoticeContent {...props} />
    </ThreadContentRail>
  );
}

function SessionWindowNoticeContent({
  agentName,
  controls,
  isForking,
  onFork,
  quietWarning,
  session,
  sessionId,
}: SessionWindowNoticeProps) {
  if (session.runtime.status === "recovering") {
    return (
      <SessionRuntimeRecoveryNotice
        attempt={session.runtime.recovery?.attempt}
        maxAttempts={session.runtime.recovery?.max_attempts}
      />
    );
  }
  if (controls.stopAttention !== null) {
    return (
      <SessionStopAttentionNotice
        isRetrying={controls.isStopRetrying}
        onRetry={controls.canRetryStop ? controls.handleStop : undefined}
      />
    );
  }
  if (controls.resumeFailure) {
    return (
      <SessionResumeFailure
        agentName={controls.resumeFailure.providerUnavailable?.agentName ?? agentName}
        isRetrying={controls.isResuming}
        message={controls.resumeFailure.message}
        missingProvider={controls.resumeFailure.providerUnavailable?.missingProvider ?? null}
        onDismiss={controls.handleDismissResumeFailure}
        onRetry={controls.handleResume}
        sessionId={sessionId}
      />
    );
  }
  if (hasUnrecoverableRuntime(session)) {
    return (
      <SessionResumeFailure
        agentName={agentName}
        isRetrying={isForking}
        message="This provider runtime cannot be resumed. Its original transcript and failure details remain available here."
        missingProvider={null}
        onDismiss={() => undefined}
        onRetry={onFork}
        retryLabel="Fork into a new session"
        sessionId={sessionId}
        showDismiss={false}
        title="Runtime unavailable"
      />
    );
  }
  if (quietWarning) {
    return (
      <SessionQuietWarningNotice
        isStopping={controls.isStopping}
        onStop={controls.canRetryStop ? controls.handleStop : undefined}
        warning={quietWarning}
      />
    );
  }
  return null;
}

/** Compose the active session window and the sidebar-owned single or batch lifecycle dialogs. */
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
    sessionContext,
    sessionUsageTurns,
    activityGoal,
    inspectorUsage,
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

  const quietWarning = sessionQuietWarning(session);

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
        <SessionWindowNotice
          agentName={agentName}
          controls={controls}
          isForking={forkSession.isPending}
          onFork={handleForkDeadSession}
          quietWarning={quietWarning}
          session={session}
          sessionId={sessionId}
        />
        <SessionThread
          liveDataEnabled={liveDataEnabled}
          sessionId={sessionId}
          workspaceId={workspaceId}
          agentName={agentName}
          acpSessionId={session.runtime.acp_session_id}
          sessionState={session.state}
          failure={session.failure}
          statusSession={session}
          stopCompletionNote={controls.stopCompletionNote}
          quietWarning={quietWarning}
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
          contextControl={
            <SessionContextControl
              context={sessionContext.context}
              open={inspector.open}
              onOpen={() => inspector.setOpen(true)}
            />
          }
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
            activitySource={{
              session,
              running: controls.isSessionRunning,
              live: liveDataEnabled,
              queued: controls.queuedPrompts.length,
              goal: activityGoal,
            }}
            context={sessionContext.context}
            turns={sessionUsageTurns.data}
            turnsUnavailable={sessionUsageTurns.isError}
            usage={inspectorUsage}
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
            sessions={sidebar.rowDeleteDialog.sessions}
            results={sidebar.rowDeleteDialog.results}
            onRetry={sidebar.rowDeleteDialog.onRetry}
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
