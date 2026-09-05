import { toast } from "sonner";

import { createClientId } from "@/lib/client-id";
import { awaitStoreRequest } from "@/lib/store-request";
import {
  queuedPromptAttachmentSummary,
  SessionBusyInputRefusalError,
  sessionSendOutcomeFromResult,
  useCancelSessionInput,
  usePromoteSessionInput,
  useReplaceSessionInput,
  useSendSessionPrompt,
  useSessionInputs,
  type QueuedPrompt,
  type SessionBusyInputAction,
  type SessionBusyInputDraft,
  type SessionBusyInputHandler,
  type SessionPromptRuntimeSnapshot,
  type SessionSendOutcome,
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
  const sendMutation = useSendSessionPrompt({ workspaceId });
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

  /**
   * Every busy verb goes through one gate. A gate that cannot honor the send
   * rejects with its reason (US-004.AC-3) — never a silent no-op — and the
   * daemon's answer resolves as the disposition envelope. The known active turn
   * rides along as a strict fence; when the poll has not reported one yet the
   * daemon resolves the live turn itself (invariant 6).
   */
  const submitBusyInput = (
    action: SessionBusyInputAction,
    draft: SessionBusyInputDraft
  ): Promise<SessionSendOutcome | void> => {
    const text = draft.message.trim();
    const attachmentCount = draft.attachments.length;
    if (!promptControlsAvailable) {
      return Promise.reject(
        new SessionBusyInputRefusalError({ attachmentCount, code: "session_not_promptable" })
      );
    }
    if (isBusyInputPending(store.getSnapshot().context)) {
      return Promise.reject(
        new SessionBusyInputRefusalError({ attachmentCount, code: "send_in_flight" })
      );
    }
    if (text.length === 0 && attachmentCount === 0) {
      return Promise.resolve();
    }
    if (action === "steer" && attachmentCount > 0) {
      return Promise.reject(
        new SessionBusyInputRefusalError({ attachmentCount, code: "steer_attachments_unsupported" })
      );
    }
    const runtime = getRuntimeSnapshot?.() ?? null;
    return requestBusyInput(store, () =>
      store.trigger.busyInputRequested({
        execute: () =>
          sendMutation.mutateAsync({
            id: sessionId,
            message: text,
            mode: action,
            ...(activeTurnId.length > 0 ? { expectedTurnId: activeTurnId } : {}),
            ...(attachmentCount > 0 ? { attachments: draft.attachments } : {}),
            ...(runtime ? { runtime } : {}),
          }),
        kind: action,
        message: text,
      })
    );
  };

  const handleQueuePrompt: SessionBusyInputHandler = draft => submitBusyInput("queue", draft);
  const handleSteerPrompt: SessionBusyInputHandler = draft => submitBusyInput("steer", draft);
  const handleInterruptPrompt: SessionBusyInputHandler = draft =>
    submitBusyInput("interrupt", draft);

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
      if (!promptControlsAvailable || pending || prompt.attachments) {
        return;
      }
      promoteInput.mutate(
        {
          queueEntryId: prompt.id,
          request: {
            ...(activeTurnId.length > 0 ? { expected_turn_id: activeTurnId } : {}),
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

function requestBusyInput(
  store: SessionPageControlsStore,
  request: () => void
): Promise<SessionSendOutcome | void> {
  return awaitStoreRequest<
    { requestId: number },
    SessionBusyInputSettlement,
    SessionSendOutcome | void
  >({
    notAcceptedMessage: "Busy input request was not accepted",
    request,
    resolveSettlement: settlement => {
      if (settlement.outcome === "failed") throw settlement.error;
      return sessionSendOutcomeFromResult(settlement.result) ?? undefined;
    },
    subscribeAccepted: listener => store.on("busyInputAccepted", listener),
    subscribeSettled: listener => store.on("busyInputSettled", listener),
  });
}
