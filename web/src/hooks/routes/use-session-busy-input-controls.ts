import { use, useEffect } from "react";
import { useSelector } from "@xstate/store-react";
import { toast } from "sonner";

import { SessionPromptDispatchContext } from "@/components/assistant-ui/session-prompt-dispatch-store";
import { createClientId } from "@/lib/client-id";
import { awaitStoreRequest } from "@/lib/store-request";
import {
  findUnconfirmedSend,
  queueCapFromInputs,
  queuedPromptsFromInputs,
  SessionBusyInputRefusalError,
  sessionSendOutcomeFromResult,
  useCancelSessionInput,
  useClearSessionInputs,
  usePromoteSessionInput,
  useReplaceSessionInput,
  useSendSessionPrompt,
  useSessionInputs,
  type QueuedPrompt,
  type SessionState,
  type SessionBusyInputAction,
  type SessionBusyInputDraft,
  type SessionBusyInputHandler,
  type SessionPromptRuntimeSnapshot,
  type SessionSendEnvelope,
  type SessionSendOutcome,
  type UnconfirmedSend,
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
  sessionState: SessionState;
  store: SessionPageControlsStore;
  workspaceId: string;
}

export function useSessionBusyInputControls({
  activeTurnId,
  getRuntimeSnapshot,
  promptControlsAvailable,
  sessionId,
  sessionState,
  store,
  workspaceId,
}: UseSessionBusyInputControlsOptions) {
  const sendMutation = useSendSessionPrompt({ workspaceId });
  const inputs = useSessionInputs(workspaceId, sessionId, { sessionState });
  const cancelInput = useCancelSessionInput(workspaceId, sessionId);
  const replaceInput = useReplaceSessionInput(workspaceId, sessionId);
  const promoteInput = usePromoteSessionInput(workspaceId, sessionId);
  const clearInputs = useClearSessionInputs(workspaceId, sessionId);
  // A streaming prompt POST that lost its acknowledgment mid-turn is retained
  // here too: one unconfirmed model for every send this window made.
  const promptDispatch = use(SessionPromptDispatchContext);
  useEffect(() => {
    if (promptDispatch === null) return;
    const subscription = promptDispatch.on("sendUnconfirmed", event => {
      store.trigger.sendUnconfirmedObserved({ send: event.envelope });
    });
    return () => subscription.unsubscribe();
  }, [promptDispatch, store]);
  const unconfirmedSends = useSelector(store, snapshot => snapshot.context.unconfirmedSends);
  const refusalQueueCap = useSelector(store, snapshot => snapshot.context.queueCap);
  // The queue list owns the cap, so "full" reads before any refusal; the cap a
  // `queue_full` refusal named is only the fallback for a daemon without the summary.
  const queueCap = queueCapFromInputs(inputs.data) ?? refusalQueueCap;
  const queuedPrompts: QueuedPrompt[] = queuedPromptsFromInputs(
    inputs.data?.inputs,
    workspaceId,
    sessionId
  );
  const pending =
    isBusyInputPending(store.getSnapshot().context) ||
    cancelInput.isPending ||
    replaceInput.isPending ||
    promoteInput.isPending ||
    clearInputs.isPending;

  /**
   * One send on the wire, identity minted here so the client can retain it: a
   * lost acknowledgment keeps the whole envelope, and Retry replays it byte for
   * byte (invariant 8). `retryOf` names the unconfirmed row being replayed.
   */
  const dispatchEnvelope = (
    envelope: SessionSendEnvelope,
    retryOf?: string
  ): Promise<SessionSendOutcome | void> =>
    requestBusyInput(store, () =>
      store.trigger.busyInputRequested({
        execute: () =>
          sendMutation.mutateAsync({
            id: sessionId,
            idempotencyKey: envelope.identity.idempotencyKey,
            message: envelope.text,
            messageId: envelope.identity.messageId,
            // The idle prompt carries no verb: the daemon resolves it (or replays it) as sent.
            ...(envelope.action === "prompt" ? {} : { mode: envelope.action }),
            ...(envelope.expectedTurnId ? { expectedTurnId: envelope.expectedTurnId } : {}),
            ...(envelope.attachments.length > 0 ? { attachments: envelope.attachments } : {}),
            ...(envelope.runtime ? { runtime: envelope.runtime } : {}),
          }),
        kind: envelope.action,
        message: envelope.text,
        send: envelope,
        ...(retryOf !== undefined ? { retryOf } : {}),
      })
    );

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
    return dispatchEnvelope({
      action,
      attachments: draft.attachments,
      expectedTurnId: activeTurnId.length > 0 ? activeTurnId : null,
      identity: { idempotencyKey: createClientId(), messageId: createClientId() },
      runtime: getRuntimeSnapshot?.() ?? null,
      text,
    });
  };

  /**
   * Retry replays the retained identity exactly as it left: same content,
   * same fence, same runtime. The daemon answers with the recorded outcome
   * (`replayed`) or dispatches it once; either way the row resolves.
   */
  const handleRetryUnconfirmedSend = (id: string): Promise<SessionSendOutcome | void> => {
    const send: UnconfirmedSend | null = findUnconfirmedSend(unconfirmedSends, id);
    if (send === null) {
      return Promise.reject(new Error("There is no unconfirmed send to retry."));
    }
    if (isBusyInputPending(store.getSnapshot().context)) {
      return Promise.reject(
        new SessionBusyInputRefusalError({
          attachmentCount: send.attachments.length,
          code: "send_in_flight",
        })
      );
    }
    return dispatchEnvelope(send, id);
  };

  const handleQueuePrompt: SessionBusyInputHandler = draft => submitBusyInput("queue", draft);
  const handleSteerPrompt: SessionBusyInputHandler = draft => submitBusyInput("steer", draft);
  const handleInterruptPrompt: SessionBusyInputHandler = draft =>
    submitBusyInput("interrupt", draft);

  return {
    /** Explicit clear-all; the strip confirms first and reports failure itself. */
    handleClearQueue: () => clearInputs.mutateAsync(),
    handleDiscardUnconfirmedSend: (id: string) => {
      store.trigger.unconfirmedSendDiscarded({ id });
    },
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
    handleRetryUnconfirmedSend,
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
    queueCap,
    queuedPrompts,
    unconfirmedSends,
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
