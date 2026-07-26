import { useEffect, useRef, useState, type SetStateAction } from "react";
import { useAui, useAuiState } from "@assistant-ui/react";
import { toast } from "sonner";

import type { QueuedPrompt } from "@/components/assistant-ui/session-composer-queued-prompts";
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

interface UseSessionPageControlsOptions {
  onDeleteSuccess?: () => void;
  workspaceId?: string;
}

export interface ResumeProviderUnavailableDetail {
  sessionId: string;
  missingProvider: string;
  agentName?: string;
}

export interface SessionResumeFailure {
  message: string;
  providerUnavailable: ResumeProviderUnavailableDetail | null;
}

const PROVIDER_VALIDATION_PATTERN =
  /validate agent "([^"]+)" with provider "([^"]+)" for session "([^"]+)"/;

function parseProviderUnavailable(
  sessionId: string,
  message: string
): ResumeProviderUnavailableDetail | null {
  const match = message.match(PROVIDER_VALIDATION_PATTERN);
  if (!match) {
    return null;
  }

  const [, agentName, missingProvider, parsedSessionId] = match;
  const providerName = missingProvider?.trim() ?? "";
  if (providerName.length === 0) {
    return null;
  }

  return {
    sessionId: parsedSessionId?.trim().length ? parsedSessionId : sessionId,
    missingProvider: providerName,
    agentName: agentName?.trim().length ? agentName : undefined,
  };
}

function describeResumeError(error: unknown): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return "Failed to attach session.";
}

function describePromptActionError(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return fallback;
}

function restoreQueuedPrompt(
  prompts: QueuedPrompt[],
  prompt: QueuedPrompt,
  originalIndex: number
): QueuedPrompt[] {
  if (prompts.some(item => item.id === prompt.id)) {
    return prompts;
  }
  const restored = [...prompts];
  const insertAt = originalIndex >= 0 ? Math.min(originalIndex, restored.length) : restored.length;
  restored.splice(insertAt, 0, prompt);
  return restored;
}

type QueuedPromptsByScope = Record<string, QueuedPrompt[] | undefined>;

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
  const [isCancellingPrompt, setIsCancellingPrompt] = useState(false);
  const [resumeFailure, setResumeFailure] = useState<SessionResumeFailure | null>(null);
  const activeTurnId = session.activity?.turn_id?.trim() ?? "";

  const daemonRunning = isSessionRunning(session);
  const userControllable = isUserControllableSession(session);
  const effectiveRunning = isRunning || daemonRunning;
  const promptControlsAvailable = effectiveRunning && userControllable;
  const canPrompt = session.state === "active" && userControllable;
  const queueScope = effectiveRunning ? `running:${activeTurnId}` : "settled";
  const [queuedPromptsByScope, setQueuedPromptsByScope] = useState<QueuedPromptsByScope>({});
  const activeQueueScopeRef = useRef(queueScope);
  useEffect(() => {
    activeQueueScopeRef.current = queueScope;
  }, [queueScope]);
  const queuedPrompts = effectiveRunning ? (queuedPromptsByScope[queueScope] ?? []) : [];
  const setQueuedPrompts = (update: SetStateAction<QueuedPrompt[]>) => {
    const updateScope = queueScope;
    setQueuedPromptsByScope(current => {
      if (activeQueueScopeRef.current !== updateScope) {
        return current;
      }
      const scopedPrompts = current[updateScope] ?? [];
      return {
        [updateScope]: typeof update === "function" ? update(scopedPrompts) : update,
      };
    });
  };

  // Queued prompts are a client-tracked optimistic mirror of the daemon's durable
  // busy-input queue: every row carries the real `queue_entry_id` the queue mutation
  // returned, so delete/steer act on real entries. When the turn settles the daemon
  // drains the queue, so drop the local rows — never render a queued prompt that the
  // runtime no longer holds (truthful UI). It may briefly under-show a second queued
  // prompt across a multi-dispatch settle, which is safe; it never over-shows.
  const handleCancelPrompt = () => {
    if (!promptControlsAvailable || isCancellingPrompt) {
      return;
    }

    setIsCancellingPrompt(true);
    void cancelSessionPrompt(workspaceId, sessionId)
      .catch(() => {
        toast.error("Failed to stop the current prompt.");
      })
      .finally(() => {
        setIsCancellingPrompt(false);
      });
  };

  const isStopping = stopMutation.isPending || isCancellingPrompt;
  const isResuming = resumeMutation.isPending;
  const isDeleting = deleteMutation.isPending;
  const isClearing = clearMutation.isPending;
  const isBusyInputPending =
    queuePromptMutation.isPending ||
    interruptPromptMutation.isPending ||
    steerPromptMutation.isPending;
  const controlsBusy = isStopping || isResuming || isDeleting || isClearing || isBusyInputPending;
  const hasConversationContent = messages.length > 0 || transcriptMessages.length > 0;
  const canClear = hasConversationContent && !controlsBusy && !effectiveRunning;

  const handleQueuePrompt = async (message: string) => {
    const text = message.trim();
    if (!promptControlsAvailable || isBusyInputPending || text.length === 0) {
      return;
    }

    const result = await queuePromptMutation.mutateAsync({ id: sessionId, message: text });
    if ("outcome" in result) {
      return;
    }
    const queueEntryId = result.queue_entry_id;
    if (result.queued && queueEntryId) {
      setQueuedPrompts(prev => [...prev, { id: queueEntryId, text }]);
      toast.success("Prompt queued.");
    }
  };

  const handleInterruptPrompt = async (message: string) => {
    const text = message.trim();
    if (!promptControlsAvailable || isBusyInputPending || text.length === 0) {
      return;
    }

    await interruptPromptMutation.mutateAsync({ id: sessionId, message: text });
    // Interrupt advances the busy-input generation, which cancels every pending
    // queued entry on the daemon — clear the local mirror so it can't lie.
    setQueuedPrompts([]);
    toast.success("Prompt interrupted.");
  };

  const handleSteerPrompt = async (message: string) => {
    const text = message.trim();
    if (!promptControlsAvailable || isBusyInputPending || text.length === 0) {
      return;
    }

    await steerPromptMutation.mutateAsync({ id: sessionId, message: text });
    toast.success("Steer staged.");
  };

  const handleRemoveQueuedPrompt = (queueEntryId: string) => {
    const removedIndex = queuedPrompts.findIndex(item => item.id === queueEntryId);
    const removedPrompt = removedIndex >= 0 ? queuedPrompts[removedIndex] : undefined;
    setQueuedPrompts(prev => prev.filter(item => item.id !== queueEntryId));
    cancelQueuedPromptMutation.mutate(
      { id: sessionId, queueEntryId },
      {
        onError: error => {
          if (removedPrompt) {
            setQueuedPrompts(prev => restoreQueuedPrompt(prev, removedPrompt, removedIndex));
          }
          toast.error(describePromptActionError(error, "Couldn't remove queued prompt."));
        },
      }
    );
  };

  const handleSteerQueuedPrompt = (prompt: QueuedPrompt) => {
    if (!promptControlsAvailable || isBusyInputPending) {
      return;
    }
    const removedIndex = queuedPrompts.findIndex(item => item.id === prompt.id);
    setQueuedPrompts(prev => prev.filter(item => item.id !== prompt.id));
    steerPromptMutation.mutate(
      { id: sessionId, message: prompt.text },
      {
        onSuccess: () => {
          // Steer injects the queued text into the live turn now, so drop its
          // durable queue slot to avoid re-running it after the turn ends.
          cancelQueuedPromptMutation.mutate(
            { id: sessionId, queueEntryId: prompt.id },
            {
              onSuccess: () => {
                toast.success("Steer staged.");
              },
              onError: error => {
                setQueuedPrompts(prev => restoreQueuedPrompt(prev, prompt, removedIndex));
                toast.error(
                  describePromptActionError(
                    error,
                    "Steer staged, but couldn't remove queued prompt."
                  )
                );
              },
            }
          );
        },
        onError: error => {
          setQueuedPrompts(prev => restoreQueuedPrompt(prev, prompt, removedIndex));
          toast.error(describePromptActionError(error, "Couldn't steer queued prompt."));
        },
      }
    );
  };

  const handleStop = () => {
    if (controlsBusy) {
      return;
    }

    if (promptControlsAvailable) {
      handleCancelPrompt();
      return;
    }

    stopMutation.mutate(sessionId);
  };

  const handleResume = () => {
    if (controlsBusy) {
      return;
    }

    setResumeFailure(null);
    resumeMutation.mutate(sessionId, {
      onError: error => {
        const message = describeResumeError(error);
        const providerUnavailable = parseProviderUnavailable(sessionId, message);
        setResumeFailure({ message, providerUnavailable });
        if (providerUnavailable === null) {
          toast.error(message);
        }
      },
      onSuccess: () => {
        setResumeFailure(null);
      },
    });
  };

  const handleDismissResumeFailure = () => {
    setResumeFailure(null);
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
    resumeFailure,
  };
}
