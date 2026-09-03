import { toast } from "sonner";

import { createClientId } from "@/lib/client-id";
import { awaitStoreRequest } from "@/lib/store-request";
import {
  queuedPromptAttachmentSummary,
  useCancelSessionInput,
  useInterruptSessionPrompt,
  usePromoteSessionInput,
  useQueueSessionPrompt,
  useReplaceSessionInput,
  useSessionInputs,
  useSteerSessionPrompt,
  type QueuedPrompt,
  type SessionBusyInputHandler,
  type SessionPromptRuntimeSnapshot,
} from "@/systems/session";

import {
  isBusyInputPending,
  type SessionBusyInputSettlement,
  type SessionPageControlsStore,
} from "./session-page-controls-store";

interface UseSessionBusyInputControlsOptions {
  activeTurnId: string;
  getRuntimeSnapshot?: () => SessionPromptRuntimeSnapshot | null;
  promptControlsAvailable: boolean;
  sessionId: string;
  store: SessionPageControlsStore;
  workspaceId: string;
}

export function useSessionBusyInputControls({
  activeTurnId,
  getRuntimeSnapshot,
  promptControlsAvailable,
  sessionId,
  store,
  workspaceId,
}: UseSessionBusyInputControlsOptions) {
  const queueMutation = useQueueSessionPrompt({ workspaceId });
  const interruptMutation = useInterruptSessionPrompt({ workspaceId });
  const steerMutation = useSteerSessionPrompt({ workspaceId });
  const inputs = useSessionInputs(workspaceId, sessionId);
  const cancelInput = useCancelSessionInput(workspaceId, sessionId);
  const replaceInput = useReplaceSessionInput(workspaceId, sessionId);
  const promoteInput = usePromoteSessionInput(workspaceId, sessionId);
  const queuedPrompts: QueuedPrompt[] =
    inputs.data?.inputs.map(input => {
      const attachments = queuedPromptAttachmentSummary(input.attachments, workspaceId, sessionId);
      return {
        id: input.id,
        mode: input.mode,
        status: input.status,
        text: input.text,
        ...(attachments ? { attachments } : {}),
      };
    }) ?? [];
  const pending =
    isBusyInputPending(store.getSnapshot().context) ||
    cancelInput.isPending ||
    replaceInput.isPending ||
    promoteInput.isPending;

  const handleQueuePrompt: SessionBusyInputHandler = draft => {
    const text = draft.message.trim();
    if (
      !promptControlsAvailable ||
      isBusyInputPending(store.getSnapshot().context) ||
      (text.length === 0 && draft.attachments.length === 0)
    ) {
      return;
    }
    const runtime = getRuntimeSnapshot?.() ?? null;
    return requestBusyInput(store, () =>
      store.trigger.busyInputRequested({
        execute: () =>
          queueMutation.mutateAsync({
            id: sessionId,
            message: text,
            ...(draft.attachments.length > 0 ? { attachments: draft.attachments } : {}),
            ...(runtime ? { runtime } : {}),
          }),
        kind: "queue",
        message: text,
      })
    );
  };

  const handleInterruptPrompt: SessionBusyInputHandler = draft => {
    const text = draft.message.trim();
    if (
      !promptControlsAvailable ||
      isBusyInputPending(store.getSnapshot().context) ||
      (text.length === 0 && draft.attachments.length === 0) ||
      activeTurnId.length === 0
    ) {
      return;
    }
    const runtime = getRuntimeSnapshot?.() ?? null;
    return requestBusyInput(store, () =>
      store.trigger.busyInputRequested({
        execute: () =>
          interruptMutation.mutateAsync({
            expectedTurnId: activeTurnId,
            id: sessionId,
            message: text,
            ...(draft.attachments.length > 0 ? { attachments: draft.attachments } : {}),
            ...(runtime ? { runtime } : {}),
          }),
        kind: "interrupt",
        message: text,
      })
    );
  };

  const handleSteerPrompt: SessionBusyInputHandler = draft => {
    const text = draft.message.trim();
    if (
      !promptControlsAvailable ||
      isBusyInputPending(store.getSnapshot().context) ||
      draft.attachments.length > 0 ||
      text.length === 0 ||
      activeTurnId.length === 0
    ) {
      return;
    }
    return requestBusyInput(store, () =>
      store.trigger.busyInputRequested({
        execute: () =>
          steerMutation.mutateAsync({
            expectedTurnId: activeTurnId,
            id: sessionId,
            message: text,
          }),
        kind: "steer",
        message: text,
      })
    );
  };

  return {
    handleInterruptPrompt,
    handleQueuePrompt,
    handleRemoveQueuedPrompt: (queueEntryId: string) => {
      cancelInput.mutate(queueEntryId, {
        onError: () => {
          console.error("Failed to remove a queued prompt");
          toast.error("Couldn't remove queued prompt.");
        },
      });
    },
    handleReplaceQueuedPrompt: (prompt: QueuedPrompt, message: string) => {
      const text = message.trim();
      if (pending || text.length === 0) {
        return Promise.reject(new Error("Pending input replacement is not available."));
      }
      return replaceInput.mutateAsync({
        queueEntryId: prompt.id,
        request: { idempotency_key: createClientId(), message_id: createClientId(), text },
      });
    },
    handleSteerPrompt,
    handleSteerQueuedPrompt: (prompt: QueuedPrompt) => {
      if (!promptControlsAvailable || pending || activeTurnId.length === 0 || prompt.attachments) {
        return;
      }
      promoteInput.mutate(
        {
          queueEntryId: prompt.id,
          request: {
            expected_turn_id: activeTurnId,
            idempotency_key: createClientId(),
            message_id: createClientId(),
            text: prompt.text,
          },
        },
        {
          onError: error => {
            console.error("Failed to steer a queued prompt", error);
            toast.error("Couldn't steer queued prompt.");
          },
        }
      );
    },
    pending,
    queuedPrompts,
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
