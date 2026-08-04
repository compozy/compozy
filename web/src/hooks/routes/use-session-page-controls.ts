import { useSelector } from "@xstate/store-react";
import { useAui, useAuiState } from "@assistant-ui/react";
import { toast } from "sonner";

import { createClientId } from "@/lib/client-id";
import { awaitStoreRequest } from "@/lib/store-request";
import { useStoreBinding } from "@/hooks/use-store-binding";
import {
  cancelSessionPrompt,
  useCancelSessionInput,
  useClearSessionConversation,
  useDeleteSession,
  useInterruptSessionPrompt,
  useQueueSessionPrompt,
  usePromoteSessionInput,
  useReplaceSessionInput,
  useResumeSession,
  useSteerSessionPrompt,
  useStopSession,
  useSessionInputs,
  useSessionTranscriptThreadMessages,
  canPromptSession,
  isSessionRunning,
  isUserControllableSession,
  type QueuedPrompt,
  type SessionPayload,
  type SessionPromptRuntimeSnapshot,
} from "@/systems/session";
import {
  createSessionPageControlsLogic,
  isBusyInputPending,
  type SessionBusyInputSettlement,
  type SessionPageControlsStore,
  type ResumeProviderUnavailableDetail,
  type SessionResumeFailure,
} from "./session-page-controls-store";

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
  const deleteMutation = useDeleteSession({ workspaceId });
  const stopMutation = useStopSession({ workspaceId });
  const resumeMutation = useResumeSession({ workspaceId });
  const clearMutation = useClearSessionConversation({ workspaceId });
  const queuePromptMutation = useQueueSessionPrompt({ workspaceId });
  const interruptPromptMutation = useInterruptSessionPrompt({ workspaceId });
  const steerPromptMutation = useSteerSessionPrompt({ workspaceId });
  const sessionInputs = useSessionInputs(workspaceId, sessionId);
  const cancelSessionInput = useCancelSessionInput(workspaceId, sessionId);
  const replaceSessionInput = useReplaceSessionInput(workspaceId, sessionId);
  const promoteSessionInput = usePromoteSessionInput(workspaceId, sessionId);
  const activeTurnId = session.activity?.turn_id?.trim() ?? "";

  const daemonRunning = isSessionRunning(session);
  const userControllable = isUserControllableSession(session);
  const effectiveRunning = isRunning || daemonRunning;
  const promptControlsAvailable = effectiveRunning && userControllable;
  const canPrompt = canPromptSession(session);
  const bindingKey = `${workspaceId}\u0000${sessionId}`;
  const { store } = useStoreBinding(bindingKey, () =>
    createSessionPageControlsLogic().createStore()
  );
  const controlsState = useSelector(store, snapshot => snapshot.context);
  const queuedPrompts: QueuedPrompt[] =
    sessionInputs.data?.inputs.map(input => ({
      id: input.id,
      mode: input.mode,
      status: input.status,
      text: input.text,
    })) ?? [];
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
  const isDeleting = deleteMutation.isPending;
  const isClearing = clearMutation.isPending;
  const busyInputPending =
    isBusyInputPending(controlsState) ||
    cancelSessionInput.isPending ||
    replaceSessionInput.isPending ||
    promoteSessionInput.isPending;
  const controlsBusy = isStopping || isResuming || isDeleting || isClearing || busyInputPending;
  const hasConversationContent = messages.length > 0 || transcriptMessages.length > 0;
  const canClear = hasConversationContent && !controlsBusy && !effectiveRunning;

  const handleQueuePrompt = (message: string) => {
    const text = message.trim();
    if (!promptControlsAvailable || busyInputIsPending(store) || text.length === 0) {
      return;
    }
    const runtime = getRuntimeSnapshot?.() ?? null;

    return requestBusyInput(store, () =>
      store.trigger.busyInputRequested({
        execute: () =>
          queuePromptMutation.mutateAsync({
            id: sessionId,
            message: text,
            ...(runtime ? { runtime } : {}),
          }),
        kind: "queue",
        message: text,
      })
    );
  };

  const handleInterruptPrompt = (message: string) => {
    const text = message.trim();
    if (
      !promptControlsAvailable ||
      busyInputIsPending(store) ||
      text.length === 0 ||
      activeTurnId.length === 0
    ) {
      return;
    }
    const runtime = getRuntimeSnapshot?.() ?? null;

    return requestBusyInput(store, () =>
      store.trigger.busyInputRequested({
        execute: () =>
          interruptPromptMutation.mutateAsync({
            expectedTurnId: activeTurnId,
            id: sessionId,
            message: text,
            ...(runtime ? { runtime } : {}),
          }),
        kind: "interrupt",
        message: text,
      })
    );
  };

  const handleSteerPrompt = (message: string) => {
    const text = message.trim();
    if (
      !promptControlsAvailable ||
      busyInputIsPending(store) ||
      text.length === 0 ||
      activeTurnId.length === 0
    ) {
      return;
    }

    return requestBusyInput(store, () =>
      store.trigger.busyInputRequested({
        execute: () =>
          steerPromptMutation.mutateAsync({
            expectedTurnId: activeTurnId,
            id: sessionId,
            message: text,
          }),
        kind: "steer",
        message: text,
      })
    );
  };

  const handleRemoveQueuedPrompt = (queueEntryId: string) => {
    cancelSessionInput.mutate(queueEntryId, {
      onError: error => toast.error(error.message || "Couldn't remove queued prompt."),
    });
  };

  const handleSteerQueuedPrompt = (prompt: QueuedPrompt) => {
    if (!promptControlsAvailable || busyInputPending || activeTurnId.length === 0) {
      return;
    }
    promoteSessionInput.mutate(
      {
        queueEntryId: prompt.id,
        request: {
          expected_turn_id: activeTurnId,
          idempotency_key: createClientId(),
          message_id: createClientId(),
          text: prompt.text,
        },
      },
      { onError: error => toast.error(error.message || "Couldn't steer queued prompt.") }
    );
  };

  const handleReplaceQueuedPrompt = (prompt: QueuedPrompt, message: string) => {
    const text = message.trim();
    if (busyInputPending || text.length === 0) {
      return Promise.reject(new Error("Pending input replacement is not available."));
    }
    return replaceSessionInput.mutateAsync({
      queueEntryId: prompt.id,
      request: {
        idempotency_key: createClientId(),
        message_id: createClientId(),
        text,
      },
    });
  };

  const handleStop = () => {
    if (controlsBusy) {
      return;
    }

    if (promptControlsAvailable) {
      handleCancelPrompt();
      return;
    }

    store.trigger.stopRequested({
      execute: () => stopMutation.mutateAsync(sessionId),
      failureMessage: null,
    });
  };

  const handleResume = () => {
    if (controlsBusy) {
      return;
    }

    store.trigger.resumeRequested({
      resumeSession: () => resumeMutation.mutateAsync(sessionId),
      sessionId,
    });
  };

  const handleDismissResumeFailure = () => {
    store.trigger.resumeFailureDismissed({});
  };

  const handleDelete = () => {
    if (controlsBusy) {
      return;
    }

    deleteMutation.mutate(sessionId, {
      onSuccess: () => {
        aui.thread.reset();
        toast.success("Session deleted.");
        onDeleteSuccess?.();
      },
      onError: error => {
        toast.error(error instanceof Error ? error.message : "Failed to delete session");
      },
    });
  };

  const handleClear = () => {
    if (controlsBusy || effectiveRunning) {
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
    allowBusyInput: userControllable,
    handleCancelPrompt,
    handleClear,
    handleDismissResumeFailure,
    handleDelete,
    handleInterruptPrompt,
    handleQueuePrompt,
    handleRemoveQueuedPrompt,
    handleReplaceQueuedPrompt,
    handleResume,
    handleSteerPrompt,
    handleSteerQueuedPrompt,
    handleStop,
    isBusyInputPending: busyInputPending,
    isClearing,
    isDeleting,
    isResuming,
    isSessionRunning: daemonRunning,
    isStopping,
    messages,
    queuedPrompts,
    resumeFailure: controlsState.resume.failure,
  };
}

function requestBusyInput(store: SessionPageControlsStore, request: () => void): Promise<void> {
  return awaitStoreRequest<{ requestId: number }, SessionBusyInputSettlement, void>({
    notAcceptedMessage: "Busy input request was not accepted",
    request,
    resolveSettlement: settlement => {
      if (settlement.outcome === "failed") throw settlement.error;
    },
    subscribeAccepted: listener => store.on("busyInputAccepted", listener),
    subscribeSettled: listener => store.on("busyInputSettled", listener),
  });
}

function busyInputIsPending(store: SessionPageControlsStore): boolean {
  return isBusyInputPending(store.getSnapshot().context);
}
