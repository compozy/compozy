import { startTransition, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useChatRuntime } from "@assistant-ui/react-ai-sdk";
import type { UIMessage } from "ai";

import { authorizeStreamFetchInput } from "@/lib/gateway-stream-auth";
import { reportGatewayResponse } from "@/lib/gateway-access-signal";
import { modelCatalogKeys } from "@/systems/model-catalog";

import type { SessionPromptDispatchStore } from "@/components/assistant-ui/session-prompt-dispatch-store";
import { sessionKeys } from "../lib/query-keys";
import { invalidateSessionMutationQueries } from "../lib/session-query-invalidation";
import { createGoalAwareFetch } from "../lib/session-goal-chat-transport";
import {
  createSessionPromptChatTransport,
  type SessionPromptRequestBody,
} from "../lib/session-prompt-chat-transport";
import {
  isSendAcknowledgmentLost,
  type SessionSendEnvelope,
} from "../lib/session-unconfirmed-send";
import { discardSessionTerminalQuote } from "../lib/session-terminal-quote";
import { applyTerminalQuoteToPromptMessage } from "../lib/session-terminal-quote-prompt";
import type { SessionPromptRecovery } from "../lib/session-prompt-recovery";
import {
  notifySessionPromptNotSent,
  sessionPromptRefusalReason,
} from "../lib/session-prompt-refusal";
import { sessionStore } from "../stores/session-store";
import type { SessionPromptRuntimeSnapshot } from "../contexts/session-prompt-runtime-context-value";
import { getSessionPromptRuntimeSnapshot } from "./use-session-prompt-runtime";
import { useSessionAttachmentAdapter } from "./use-session-attachment-adapter";
import { useOptionalSessionPromptRuntimeContext } from "./use-session-prompt-runtime-context";
import { loopsKeys } from "@/systems/loops";
import { fetchSessionTranscript } from "../adapters/session-api";
import { sessionTranscriptOptions } from "../lib/session-transcript-options";
import {
  reconcileAdmittedPrompt,
  type SessionTranscriptData,
} from "../lib/session-transcript-query";

type QueryClient = ReturnType<typeof useQueryClient>;

function completeWhenResponseBodySettles(response: Response, onComplete: () => void): Response {
  if (response.body === null) {
    onComplete();
    return response;
  }

  const reader = response.body.getReader();
  let completed = false;
  const complete = () => {
    if (completed) return;
    completed = true;
    onComplete();
  };
  const body = new ReadableStream<Uint8Array>({
    async pull(controller) {
      try {
        const chunk = await reader.read();
        if (chunk.done) {
          complete();
          controller.close();
          return;
        }
        controller.enqueue(chunk.value);
      } catch (error) {
        complete();
        controller.error(error);
      }
    },
    async cancel(reason) {
      try {
        await reader.cancel(reason);
      } finally {
        complete();
      }
    },
  });
  return new Response(body, {
    headers: response.headers,
    status: response.status,
    statusText: response.statusText,
  });
}

/**
 * Settles the composer draft a prompt POST took. The draft is consumed only
 * once the daemon accepts the prompt; a refusal, or a request that never got
 * an answer, gives it back to its session.
 */
interface SessionPromptOutcome {
  accepted: (messageId: string | null) => void;
  canceled: (messageId: string | null) => void;
  /** The prompt never left this browser; its draft returns without a notice. */
  returned: (messageId: string | null) => void;
  /** The daemon refused it, or never answered: the draft returns with the reason. */
  notSent: (messageId: string | null, reason: string | null) => void;
}

/** The durable identity a prompt POST carries; every outcome settles only that prompt's draft. */
function promptRequestMessageId(body: BodyInit | null | undefined): string | null {
  if (typeof body !== "string") return null;
  try {
    const parsed: unknown = JSON.parse(body);
    if (typeof parsed !== "object" || parsed === null || !("message_id" in parsed)) return null;
    return typeof parsed.message_id === "string" ? parsed.message_id : null;
  } catch {
    return null;
  }
}

function promptRuntimeClosedError(): DOMException {
  return new DOMException("The session runtime closed before the daemon answered.", "AbortError");
}

/**
 * Runtime teardown (navigation, a profile switch) stops the chat while a POST
 * may still be waiting for admission. The daemon keeps executing it either
 * way, so the request runs on detached and its answer decides the draft.
 */
function settleDetachedPrompt(
  request: Promise<Response>,
  messageId: string | null,
  outcome: SessionPromptOutcome
): void {
  void request.then(
    async response => {
      if (response.ok) {
        outcome.accepted(messageId);
        await response.body?.cancel().catch(() => undefined);
        return;
      }
      if (response.status >= 500) {
        // A 5xx may follow durable acceptance: not a refusal. With no composer left to hold
        // a retryable pending-ack row, the draft returns as unconfirmed instead of being lost.
        await response.body?.cancel().catch(() => undefined);
        outcome.notSent(messageId, null);
        return;
      }
      outcome.notSent(messageId, sessionPromptRefusalReason(await response.text().catch(() => "")));
    },
    () => outcome.notSent(messageId, null)
  );
}

/**
 * The identity and content of the prompt on the wire, retained so a POST that
 * loses its acknowledgment mid-turn stays retryable by the same identity.
 */
function envelopeFromPromptBody(
  body: SessionPromptRequestBody,
  runtime: SessionPromptRuntimeSnapshot | null
): SessionSendEnvelope {
  const text: string[] = [];
  for (const message of body.messages) {
    for (const part of message.parts) {
      if (part.type === "text" && part.text.length > 0) text.push(part.text);
    }
  }
  return {
    action: "prompt",
    attachments: body.attachments ?? [],
    expectedTurnId: null,
    identity: { idempotencyKey: body.idempotency_key, messageId: body.message_id },
    runtime,
    text: text.join("\n"),
  };
}

function buildSessionRuntimeConfig(
  queryClient: QueryClient,
  workspaceId: string,
  sessionId: string,
  promptDispatch: SessionPromptDispatchStore,
  promptRecovery: SessionPromptRecovery,
  promptOutcome: SessionPromptOutcome,
  getRuntimeSnapshot?: () => SessionPromptRuntimeSnapshot | null,
  idempotencyKeys?: Map<string, string>,
  preparedUserMessages?: Map<string, UIMessage>,
  readOnly = false
) {
  const recoveryScope = { workspaceId, sessionId };
  const goalAwareFetch = createGoalAwareFetch({
    onRequest: () => {
      sessionStore.trigger.goalErrorAcknowledged({ sessionId });
    },
    onResult: (result, requestText) => {
      sessionStore.trigger.goalCommandReported({
        sessionId,
        result,
        ...(requestText === null || requestText === undefined ? {} : { command: requestText }),
      });
      if (result.snapshot !== null || result.outcome === "cleared") {
        queryClient.setQueryData(sessionKeys.goal(workspaceId, sessionId), {
          goal: result.snapshot,
        });
      }
      const runId = result.snapshot?.run_id;
      if (runId) {
        void queryClient.invalidateQueries({
          queryKey: loopsKeys.runDetail(workspaceId, runId),
        });
      }
      void queryClient.invalidateQueries({
        queryKey: loopsKeys.runsByWorkspace(workspaceId),
      });
    },
  });
  const trackedFetch: typeof globalThis.fetch = async (input, init) => {
    if (readOnly) throw new Error("This session history is read-only");
    const controller = new AbortController();
    const upstreamSignal = init?.signal;
    const messageId = promptRequestMessageId(init?.body);
    let answered = false;
    let detached = false;
    let request: Promise<Response> | null = null;
    let detach = () => {};
    const runtimeClosed = new Promise<never>((_resolve, reject) => {
      detach = () => {
        detached = true;
        reject(promptRuntimeClosedError());
      };
    });
    // Observed only through the races below; teardown after the answer must not surface it.
    runtimeClosed.catch(() => undefined);
    const abortFromUpstream = () => {
      if (answered) {
        controller.abort(upstreamSignal?.reason);
        return;
      }
      detach();
    };
    if (upstreamSignal?.aborted) {
      promptOutcome.returned(messageId);
      throw promptRuntimeClosedError();
    }
    upstreamSignal?.addEventListener("abort", abortFromUpstream, { once: true });
    promptDispatch.trigger.requestStarted({ at: Date.now(), controller });
    let responseOwnsCompletion = false;
    let requestCompleted = false;
    const completeRequest = () => {
      if (requestCompleted) return;
      requestCompleted = true;
      upstreamSignal?.removeEventListener("abort", abortFromUpstream);
      promptDispatch.trigger.requestCompleted({ controller });
    };
    try {
      // Local HTTP/1.1 browsers have a small per-origin connection pool. The
      // desktop already owns several long-lived SSE connections, so a slow
      // provider discovery can otherwise queue the prompt behind the model
      // catalog. Prompt dispatch is the user-driven operation, so release any
      // optional catalog reads first; the selector's explicit refresh remains
      // available when the operator next needs discovery.
      await Promise.race([
        queryClient.cancelQueries({ queryKey: modelCatalogKeys.all }),
        runtimeClosed,
      ]);
      // The prompt POST streams its response, so a remote gateway session
      // authenticates it with a fresh single-use ticket like any other stream.
      const target = await Promise.race([
        authorizeStreamFetchInput(input, controller.signal),
        runtimeClosed,
      ]);
      request = goalAwareFetch(target, { ...init, signal: controller.signal });
      const response = await Promise.race([request, runtimeClosed]);
      answered = true;
      promptDispatch.trigger.promptAnswered({ status: response.status });
      await reportGatewayResponse(response);
      if (response.ok) {
        promptOutcome.accepted(messageId);
        // The POST streams the assistant, not the daemon's verified user-input
        // metadata. Read that input once while the parallel transcript SSE is
        // suspended, retaining its cursor and every streaming assistant message.
        if (messageId !== null) {
          void fetchSessionTranscript(workspaceId, sessionId, {}, controller.signal)
            .then(async transcript => {
              // A cold composer may send before its first history read finishes.
              // Join that canonical read before merging, so its older snapshot
              // cannot discard the admitted input and its verified directives.
              await queryClient.ensureInfiniteQueryData(
                sessionTranscriptOptions(workspaceId, sessionId)
              );
              if (controller.signal.aborted) return;
              queryClient.setQueryData<SessionTranscriptData>(
                sessionKeys.transcript(workspaceId, sessionId),
                current => reconcileAdmittedPrompt(current, transcript, messageId)
              );
            })
            .catch(error => {
              if (!controller.signal.aborted) {
                console.warn(
                  "Could not reconcile the admitted prompt; the live tail will retry",
                  error
                );
              }
            });
        }
      }
      responseOwnsCompletion = true;
      return completeWhenResponseBodySettles(response, completeRequest);
    } catch (error) {
      if (detached) {
        if (request) settleDetachedPrompt(request, messageId, promptOutcome);
        else promptOutcome.returned(messageId);
      } else if (!answered && controller.signal.aborted) {
        // An operator cancel aborts the request before any answer: nothing comes back to restore.
        promptOutcome.canceled(messageId);
      }
      throw error;
    } finally {
      if (!responseOwnsCompletion) {
        completeRequest();
      }
    }
  };
  return {
    transport: createSessionPromptChatTransport({
      api: `/api/workspaces/${encodeURIComponent(workspaceId)}/sessions/${encodeURIComponent(sessionId)}/prompt`,
      fetch: trackedFetch,
      ...(getRuntimeSnapshot ? { getRuntimeSnapshot } : {}),
      ...(idempotencyKeys ? { idempotencyKeys } : {}),
      ...(preparedUserMessages ? { preparedUserMessages } : {}),
      onPromptPrepared: request => {
        promptRecovery.stage(recoveryScope, request.body.message_id, request);
        promptDispatch.trigger.promptPrepared({
          envelope: envelopeFromPromptBody(request.body, getRuntimeSnapshot?.() ?? null),
        });
        discardSessionTerminalQuote(sessionId);
      },
      prepareUserMessage: message => applyTerminalQuoteToPromptMessage(sessionId, message),
    }),
    onFinish: () => {
      startTransition(() => {
        void invalidateSessionMutationQueries(queryClient, workspaceId, sessionId);
      });
    },
  };
}

/** Connects session dispatch and recovery to chat streaming and session-scoped completion events. */
export function useSessionChatRuntime({
  sessionId,
  workspaceId,
  promptDispatch,
  promptRecovery,
  readOnly = false,
}: {
  sessionId: string;
  workspaceId: string;
  promptDispatch: SessionPromptDispatchStore;
  promptRecovery: SessionPromptRecovery;
  readOnly?: boolean;
}) {
  const queryClient = useQueryClient();
  const promptRuntime = useOptionalSessionPromptRuntimeContext();
  const [idempotencyKeys] = useState(() => new Map<string, string>());
  const [preparedUserMessages] = useState(() => new Map<string, UIMessage>());
  const attachmentAdapter = useSessionAttachmentAdapter(workspaceId, sessionId);
  const recoveryScope = { workspaceId, sessionId };
  const consumeDraft = (messageId: string | null) => {
    if (messageId !== null) promptRecovery.acknowledge(recoveryScope, messageId);
    attachmentAdapter.acknowledgeSentFiles();
  };
  const restoreDraft = (messageId: string | null) => {
    const files = attachmentAdapter.recoverSentFiles();
    if (messageId !== null) promptRecovery.recover(recoveryScope, messageId, files);
  };
  const promptOutcome: SessionPromptOutcome = {
    accepted: consumeDraft,
    canceled: consumeDraft,
    returned: restoreDraft,
    notSent: (messageId, reason) => {
      restoreDraft(messageId);
      notifySessionPromptNotSent(reason);
    },
  };
  const runtimeConfig = buildSessionRuntimeConfig(
    queryClient,
    workspaceId,
    sessionId,
    promptDispatch,
    promptRecovery,
    promptOutcome,
    promptRuntime ? () => getSessionPromptRuntimeSnapshot(promptRuntime) : undefined,
    idempotencyKeys,
    preparedUserMessages,
    readOnly
  );

  const onError = (error: Error) => {
    // A POST with no authoritative answer — no response at all, or a 5xx that
    // may have followed durable acceptance — may still have been recorded: the
    // identity is retained as pending-ack and the tail reopens at once (Part II
    // topology). A 2xx accepted the turn (it continues detached if the body
    // breaks later; the draft was already consumed) and a 4xx refused it (the
    // draft comes back): both are outcomes, never unconfirmed.
    const prompt = promptDispatch.getSnapshot().context.prompt;
    const answered = prompt?.answeredStatus ?? null;
    const messageId = prompt?.envelope.identity.messageId ?? null;
    const unconfirmed =
      prompt !== null && (answered === null || answered >= 500) && isSendAcknowledgmentLost(error);
    promptDispatch.trigger.promptFailed();
    if (unconfirmed) {
      // The app-scoped owner keeps the identity for Retry even when this
      // session's page is gone, as it is for a detached edit send.
      promptRecovery.retainUnconfirmed(recoveryScope, prompt.envelope);
      consumeDraft(messageId);
      return;
    }
    if (answered !== null && answered >= 400 && answered < 500) {
      promptOutcome.notSent(
        messageId,
        sessionPromptRefusalReason(error instanceof Error ? error.message : "")
      );
      return;
    }
    restoreDraft(messageId);
  };

  const runtime = useChatRuntime({
    transport: runtimeConfig.transport,
    onError,
    /**
     * Emits a draft handoff only for the matching successful operator prompt.
     * The draft itself settles with the POST's answer (see trackedFetch): an
     * abort here can be a runtime teardown whose prompt is still awaiting one.
     */
    onFinish: ({ isError, isAbort, isDisconnect, finishReason, message, messages }) => {
      const prompt = promptDispatch.getSnapshot().context.prompt;
      const submitted = messages.filter(item => item.role === "user").at(-1);
      if (
        !isError &&
        !isAbort &&
        !isDisconnect &&
        finishReason === "stop" &&
        prompt?.answeredStatus === 200 &&
        submitted?.id === prompt.envelope.identity.messageId &&
        /^\/goal\s+draft\s+\S/.test(prompt.envelope.text.trim())
      ) {
        // Earlier text parts may describe tool work; only the final answer is the proposal.
        const proposal = message.parts
          .filter(part => part.type === "text")
          .at(-1)
          ?.text.trim();
        if (proposal) {
          sessionStore.trigger.goalDraftCompleted({ sessionId, text: `/goal ${proposal}` });
        }
      }
      runtimeConfig.onFinish();
    },
    adapters: { attachments: attachmentAdapter },
  });
  return {
    runtime,
    // Capture the transport, not the UI thread proxy which disappears on navigation.
    sendDetachedPrompt: async (message: UIMessage) => {
      try {
        const stream = await runtimeConfig.transport.sendMessages({
          abortSignal: undefined,
          chatId: sessionId,
          messageId: undefined,
          messages: [message],
          trigger: "submit-message",
        });
        await stream.cancel();
        runtimeConfig.onFinish();
      } catch (error) {
        onError(error instanceof Error ? error : new Error(String(error)));
        throw error;
      }
    },
  };
}
