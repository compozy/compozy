import { useSelector } from "@xstate/store-react";
import { useAui, useAuiState } from "@assistant-ui/react";
import { toast } from "sonner";

import { useStoreBinding } from "@/hooks/use-store-binding";
import {
  cancelSessionPrompt,
  useClearSessionConversation,
  useDeleteSession,
  useRenameSession,
  useResumeSession,
  useStopSession,
  useSessionTranscriptThreadMessages,
  canPromptSession,
  isSessionRunning,
  isUserControllableSession,
  useUnarchiveSession,
  type SessionPayload,
  type SessionPromptRuntimeSnapshot,
} from "@/systems/session";
import {
  createSessionPageControlsLogic,
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
      execute: () => cancelSessionPrompt(workspaceId, sessionId),
      failureMessage: "Failed to stop the current prompt.",
    });
  };

  const isStopping = controlsState.stop.phase === "pending";
  const isResuming = controlsState.resume.phase === "pending";
  const isUnarchiving = unarchiveMutation.isPending;
  const isDeleting = deleteMutation.isPending;
  const isRenaming = renameMutation.isPending;
  const isClearing = clearMutation.isPending;
  const busyInputPending = busyInput.pending;
  const controlsBusy =
    isStopping ||
    isResuming ||
    isUnarchiving ||
    isDeleting ||
    isRenaming ||
    isClearing ||
    busyInputPending;
  const hasConversationContent = messages.length > 0 || transcriptMessages.length > 0;
  const canClear = userControllable && hasConversationContent && !controlsBusy && !effectiveRunning;

  const handleStop = () => {
    if (controlsBusy || !userControllable) {
      return;
    }

    store.trigger.stopRequested({
      execute: () => stopMutation.mutateAsync(sessionId),
      failureMessage: null,
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
    allowBusyInput: canPrompt,
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
    isStopping,
    isUnarchiving,
    messages,
    queuedPrompts: busyInput.queuedPrompts,
    resumeFailure: controlsState.resume.failure,
  };
}
