import { useEffect } from "react";
import { useSelector } from "@xstate/store-react";
import { useAui, useAuiState } from "@assistant-ui/react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { useStoreBinding } from "@/hooks/use-store-binding";
import {
  cancelSessionPrompt,
  invalidateSessionMutationQueries,
  useClearSessionConversation,
  useDeleteSession,
  useRenameSession,
  useResumeSession,
  useStopSession,
  useSessionTranscriptThreadMessages,
  canPromptSession,
  isSessionRunning,
  isUserControllableSession,
  sessionBusyInputDefaultMode,
  sessionSteerDelivery,
  sessionStopAttention,
  useUnarchiveSession,
  type SessionPayload,
  type SessionPromptRuntimeSnapshot,
} from "@/systems/session";
import {
  createSessionPageControlsLogic,
  isStopRequestActive,
  isStopRetryPending,
  type ResumeProviderUnavailableDetail,
  type SessionResumeFailure,
} from "./session-page-controls-store";
import { useSessionBusyInputControls } from "./use-session-busy-input-controls";

interface UseSessionPageControlsOptions {
  getRuntimeSnapshot?: () => SessionPromptRuntimeSnapshot | null;
  onDeleteSuccess?: () => void;
  workspaceId?: string;
}

export type { ResumeProviderUnavailableDetail, SessionResumeFailure };

export function useSessionPageControls(
  sessionId: string,
  session: SessionPayload,
  options: UseSessionPageControlsOptions = {}
) {
  const aui = useAui();
  const queryClient = useQueryClient();
  const workspaceId = options.workspaceId ?? "";
  const onDeleteSuccess = options.onDeleteSuccess;
  const getRuntimeSnapshot = options.getRuntimeSnapshot;
  const messages = useAuiState(state => state.thread.messages);
  const transcriptMessages = useSessionTranscriptThreadMessages();
  const isRunning = useAuiState(state => state.thread.isRunning);
  const deleteMutation = useDeleteSession({
    workspaceId,
    onDeleteSuccess: () => {
      aui.thread.reset();
      toast.success("Session deleted.");
      onDeleteSuccess?.();
    },
  });
  const stopMutation = useStopSession({ workspaceId });
  const resumeMutation = useResumeSession({ workspaceId });
  const unarchiveMutation = useUnarchiveSession({ workspaceId });
  const renameMutation = useRenameSession({ workspaceId });
  const clearMutation = useClearSessionConversation({ workspaceId });
  const activeTurnId = session.activity?.turn_id?.trim() ?? "";

  const daemonRunning = isSessionRunning(session);
  const userControllable = isUserControllableSession(session);
  const effectiveRunning = isRunning || daemonRunning;
  const canPrompt = canPromptSession(session);
  const promptControlsAvailable = effectiveRunning && canPrompt;
  const bindingKey = `${workspaceId}\u0000${sessionId}`;
  const { store } = useStoreBinding(bindingKey, () =>
    createSessionPageControlsLogic().createStore()
  );
  const controlsState = useSelector(store, snapshot => snapshot.context);
  const sessionState = session.state;
  // The daemon's lifecycle is the only thing that settles an accepted stop:
  // every poll or invalidation result enters the store as evidence.
  useEffect(() => {
    store.trigger.lifecycleObserved({
      running: daemonRunning,
      state: sessionState,
      turnId: activeTurnId,
    });
  }, [activeTurnId, daemonRunning, sessionState, store]);
  const busyInput = useSessionBusyInputControls({
    activeTurnId,
    getRuntimeSnapshot,
    promptControlsAvailable,
    sessionId,
    store,
    workspaceId,
  });
  const handleCancelPrompt = () => {
    if (!promptControlsAvailable) {
      return;
    }

    store.trigger.stopRequested({
      execute: async () => {
        await cancelSessionPrompt(workspaceId, sessionId);
        // The request's outcome is its acceptance. The reread that carries the
        // lifecycle evidence belongs to TanStack Query: a refetch failure lands
        // in query state and is logged, never on the accepted stop.
        invalidateSessionMutationQueries(queryClient, workspaceId, sessionId).catch(error => {
          console.error("Failed to reread the session after cancelling its prompt", error);
        });
      },
      failureMessage: "Failed to stop the current prompt.",
      scope: "turn",
      turnId: activeTurnId,
    });
  };

  const stopRequestActive = isStopRequestActive(controlsState);
  // Stopping reads true from the first activation until the daemon confirms
  // the stop landed; a session the daemon itself reports as `stopping` reads
  // the same way even when no request of ours is in flight (US-009.AC-1/AC-3).
  const isStopping = stopRequestActive || sessionState === "stopping";
  // The daemon could not verify the stop: the session stays `stopping` and
  // carries the attention until the daemon reads `stopped`. Neither
  // acceptance, escalation, nor time clears it here — the read model does.
  const stopAttention = sessionStopAttention(session);
  const isStopRetrying = isStopRetryPending(controlsState);
  const isResuming = controlsState.resume.phase === "pending";
  const isUnarchiving = unarchiveMutation.isPending;
  const isDeleting = deleteMutation.isPending;
  const isRenaming = renameMutation.isPending;
  const isClearing = clearMutation.isPending;
  const busyInputPending = busyInput.pending;
  const controlsBusy =
    stopRequestActive ||
    isResuming ||
    isUnarchiving ||
    isDeleting ||
    isRenaming ||
    isClearing ||
    busyInputPending;
  const hasConversationContent = messages.length > 0 || transcriptMessages.length > 0;
  const canClear = userControllable && hasConversationContent && !controlsBusy && !effectiveRunning;

  // Stop and its retry are one action with one owner. A retry of an
  // unverified stop is the same session stop, waited on (`wait: true`) so the
  // request resolves with the daemon's settled answer; that answer only says
  // this request settled — the session read model still owns the truth.
  const handleStop = () => {
    if (controlsBusy || !userControllable) {
      return;
    }

    const retry = stopAttention !== null;
    store.trigger.stopRequested({
      execute: () => stopMutation.mutateAsync({ id: sessionId, wait: retry }),
      failureMessage: null,
      retry,
      scope: "session",
      turnId: activeTurnId,
    });
  };

  const handleResume = () => {
    if (controlsBusy || !userControllable) {
      return;
    }

    store.trigger.resumeRequested({
      resumeSession: () => resumeMutation.mutateAsync(sessionId),
      sessionId,
    });
  };

  const handleUnarchive = () => {
    if (controlsBusy || !userControllable || session.archived_at === null) {
      return;
    }

    unarchiveMutation.mutate(sessionId, {
      onError: error => {
        console.error("Failed to unarchive session", error);
        toast.error("Couldn't unarchive this session.");
      },
    });
  };

  const handleDismissResumeFailure = () => {
    store.trigger.resumeFailureDismissed({});
  };

  const handleDelete = () => {
    if (controlsBusy || !userControllable) {
      return;
    }
    deleteMutation.mutate(sessionId, {
      onError: error => {
        console.error("Failed to delete session", error);
        toast.error("Couldn't delete this session.");
      },
    });
  };

  const handleRename = async (name: string) => {
    if (!userControllable) {
      return;
    }
    if (controlsBusy) {
      throw new Error("Session controls are busy.");
    }
    try {
      await renameMutation.mutateAsync({ id: sessionId, name });
      toast.success("Session renamed.");
    } catch (error) {
      console.error("Failed to rename session", error);
      toast.error("Couldn't rename this session.");
      throw error;
    }
  };

  const handleClear = () => {
    if (controlsBusy || !userControllable || effectiveRunning) {
      return;
    }

    clearMutation.mutate(sessionId, {
      onSuccess: () => {
        aui.thread.reset();
      },
    });
  };

  return {
    canClear,
    canPrompt,
    canRetryStop: userControllable,
    allowBusyInput: canPrompt,
    busyInputDefaultMode: sessionBusyInputDefaultMode(session),
    busyInputSteerDelivery: sessionSteerDelivery(session),
    handleCancelPrompt,
    handleClear,
    handleDismissResumeFailure,
    handleDelete,
    handleInterruptPrompt: busyInput.handleInterruptPrompt,
    handleQueuePrompt: busyInput.handleQueuePrompt,
    handleRemoveQueuedPrompt: busyInput.handleRemoveQueuedPrompt,
    handleReplaceQueuedPrompt: busyInput.handleReplaceQueuedPrompt,
    handleRename,
    handleResume,
    handleSteerPrompt: busyInput.handleSteerPrompt,
    handleSteerQueuedPrompt: busyInput.handleSteerQueuedPrompt,
    handleStop,
    handleUnarchive,
    isBusyInputPending: busyInputPending,
    isClearing,
    isDeleting,
    isRenaming,
    isResuming,
    isSessionRunning: daemonRunning,
    isStopRetrying,
    isStopping,
    isUnarchiving,
    messages,
    queuedPrompts: busyInput.queuedPrompts,
    resumeFailure: controlsState.resume.failure,
    stopAttention,
    stopPhase: isStopping ? ("stopping" as const) : ("idle" as const),
  };
}
