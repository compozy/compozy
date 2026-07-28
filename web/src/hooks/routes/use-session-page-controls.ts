import { useSelector } from "@xstate/store-react";
import { useLayoutEffect } from "react";
import { useAui, useAuiState } from "@assistant-ui/react";
import { toast } from "sonner";

import type { QueuedPrompt } from "@/components/assistant-ui/session-composer-queued-prompts";
import { awaitStoreRequest } from "@/lib/store-request";
import { useStoreBinding } from "@/hooks/use-store-binding";
import {
  cancelSessionPrompt,
  useCancelQueuedSessionPrompt,
  useClearSessionConversation,
  useDeleteSession,
  useInterruptSessionPrompt,
  useQueueSessionPrompt,
  useResumeSession,
  useSteerSessionPrompt,
  useStopSession,
  useSessionTranscriptThreadMessages,
  isSessionRunning,
  isUserControllableSession,
  type SessionPayload,
} from "@/systems/session";
import {
  createSessionPageControlsLogic,
  type SessionBusyInputSettlement,
  type SessionPageControlsStore,
  type ResumeProviderUnavailableDetail,
  type SessionResumeFailure,
} from "./session-page-controls-store";

interface UseSessionPageControlsOptions {
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
  const cancelQueuedPromptMutation = useCancelQueuedSessionPrompt({ workspaceId });
  const activeTurnId = session.activity?.turn_id?.trim() ?? "";

  const daemonRunning = isSessionRunning(session);
  const userControllable = isUserControllableSession(session);
  const effectiveRunning = isRunning || daemonRunning;
  const promptControlsAvailable = effectiveRunning && userControllable;
  const canPrompt = session.state === "active" && userControllable;
  const queueScope = effectiveRunning ? `running:${activeTurnId}` : "settled";
  const bindingKey = `${workspaceId}\u0000${sessionId}`;
  const { store } = useStoreBinding(bindingKey, () =>
    createSessionPageControlsLogic(queueScope).createStore()
  );
  const controlsState = useSelector(store, snapshot => snapshot.context);
  const queuedPrompts = controlsState.queuedPrompts;

  useLayoutEffect(() => {
    store.trigger.queueScopeChanged({ queueScope });
  }, [queueScope, store]);

  // Queued prompts are a client-tracked optimistic mirror of the daemon's durable
  // busy-input queue: every row carries the real `queue_entry_id` the queue mutation
  // returned, so delete/steer act on real entries. When the turn settles the daemon
  // drains the queue, so drop the local rows — never render a queued prompt that the
  // runtime no longer holds (truthful UI). It may briefly under-show a second queued
  // prompt across a multi-dispatch settle, which is safe; it never over-shows.
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
  const queuedSteerPending = Object.values(controlsState.pendingQueueEdits).some(
    edit => edit.kind === "steer"
  );
  const isBusyInputPending = controlsState.busyInput.phase === "pending" || queuedSteerPending;
  const controlsBusy = isStopping || isResuming || isDeleting || isClearing || isBusyInputPending;
  const hasConversationContent = messages.length > 0 || transcriptMessages.length > 0;
  const canClear = hasConversationContent && !controlsBusy && !effectiveRunning;

  const handleQueuePrompt = (message: string) => {
    const text = message.trim();
    if (!promptControlsAvailable || busyInputIsPending(store) || text.length === 0) {
      return;
    }

    return requestBusyInput(store, () =>
      store.trigger.busyInputRequested({
        execute: () => queuePromptMutation.mutateAsync({ id: sessionId, message: text }),
        kind: "queue",
        message: text,
      })
    );
  };

  const handleInterruptPrompt = (message: string) => {
    const text = message.trim();
    if (!promptControlsAvailable || busyInputIsPending(store) || text.length === 0) {
      return;
    }

    return requestBusyInput(store, () =>
      store.trigger.busyInputRequested({
        execute: () => interruptPromptMutation.mutateAsync({ id: sessionId, message: text }),
        kind: "interrupt",
        message: text,
      })
    );
  };

  const handleSteerPrompt = (message: string) => {
    const text = message.trim();
    if (!promptControlsAvailable || busyInputIsPending(store) || text.length === 0) {
      return;
    }

    return requestBusyInput(store, () =>
      store.trigger.busyInputRequested({
        execute: () => steerPromptMutation.mutateAsync({ id: sessionId, message: text }),
        kind: "steer",
        message: text,
      })
    );
  };

  const handleRemoveQueuedPrompt = (queueEntryId: string) => {
    store.trigger.queuedPromptRemovalRequested({
      execute: () =>
        new Promise<void>((resolve, reject) => {
          cancelQueuedPromptMutation.mutate(
            { id: sessionId, queueEntryId },
            { onError: error => reject(error), onSuccess: () => resolve() }
          );
        }),
      queueEntryId,
    });
  };

  const handleSteerQueuedPrompt = (prompt: QueuedPrompt) => {
    if (!promptControlsAvailable || busyInputIsPending(store)) {
      return;
    }
    store.trigger.queuedPromptSteerRequested({
      cancel: () =>
        new Promise<void>((resolve, reject) => {
          cancelQueuedPromptMutation.mutate(
            { id: sessionId, queueEntryId: prompt.id },
            { onError: error => reject(error), onSuccess: () => resolve() }
          );
        }),
      prompt,
      steer: () =>
        new Promise<void>((resolve, reject) => {
          steerPromptMutation.mutate(
            { id: sessionId, message: prompt.text },
            { onError: error => reject(error), onSuccess: () => resolve() }
          );
        }),
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
    store.trigger.resumeFailureDismissed();
  };

  const handleDelete = () => {
    if (controlsBusy) {
      return;
    }

    deleteMutation.mutate(sessionId, {
      onSuccess: () => {
        aui.thread().reset();
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
        aui.thread().reset();
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
    handleResume,
    handleSteerPrompt,
    handleSteerQueuedPrompt,
    handleStop,
    isBusyInputPending,
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
  const context = store.getSnapshot().context;
  return (
    context.busyInput.phase === "pending" ||
    Object.values(context.pendingQueueEdits).some(edit => edit.kind === "steer")
  );
}
