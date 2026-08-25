import { startTransition, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useChatRuntime } from "@assistant-ui/react-ai-sdk";

import { authorizeStreamFetchInput } from "@/lib/gateway-stream-auth";
import { reportGatewayResponse } from "@/lib/gateway-access-signal";
import { modelCatalogKeys } from "@/systems/model-catalog";

import type { SessionPromptDispatchStore } from "@/components/assistant-ui/session-prompt-dispatch-store";
import { sessionKeys } from "../lib/query-keys";
import { invalidateSessionMutationQueries } from "../lib/session-query-invalidation";
import { createGoalAwareFetch } from "../lib/session-goal-chat-transport";
import { createSessionPromptChatTransport } from "../lib/session-prompt-chat-transport";
import { SessionPromptRecovery } from "../lib/session-prompt-recovery";
import { sessionStore } from "../stores/session-store";
import type { SessionPromptRuntimeSnapshot } from "../contexts/session-prompt-runtime-context-value";
import { getSessionPromptRuntimeSnapshot } from "./use-session-prompt-runtime";
import { useSessionAttachmentAdapter } from "./use-session-attachment-adapter";
import { useOptionalSessionPromptRuntimeContext } from "./use-session-prompt-runtime-context";
import { loopsKeys } from "@/systems/loops";

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

function buildSessionRuntimeConfig(
  queryClient: QueryClient,
  workspaceId: string,
  sessionId: string,
  promptDispatch: SessionPromptDispatchStore,
  promptRecovery: SessionPromptRecovery,
  getRuntimeSnapshot?: () => SessionPromptRuntimeSnapshot | null,
  idempotencyKeys?: Map<string, string>
) {
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
    const controller = new AbortController();
    const upstreamSignal = init?.signal;
    const abortFromUpstream = () => controller.abort(upstreamSignal?.reason);
    if (upstreamSignal?.aborted) {
      abortFromUpstream();
    } else {
      upstreamSignal?.addEventListener("abort", abortFromUpstream, { once: true });
    }
    promptDispatch.trigger.requestStarted({ controller });
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
      await queryClient.cancelQueries({ queryKey: modelCatalogKeys.all });
      // The prompt POST streams its response, so a remote gateway session
      // authenticates it with a fresh single-use ticket like any other stream.
      const target = await authorizeStreamFetchInput(input, controller.signal);
      const response = await goalAwareFetch(target, { ...init, signal: controller.signal });
      await reportGatewayResponse(response);
      if (response.ok) promptRecovery.acknowledge();
      responseOwnsCompletion = true;
      return completeWhenResponseBodySettles(response, completeRequest);
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
      onPromptPrepared: messages => promptRecovery.stage(messages),
    }),
    onFinish: () => {
      startTransition(() => {
        void invalidateSessionMutationQueries(queryClient, workspaceId, sessionId);
      });
    },
  };
}

export function useSessionChatRuntime({
  sessionId,
  workspaceId,
  promptDispatch,
  promptRecovery,
}: {
  sessionId: string;
  workspaceId: string;
  promptDispatch: SessionPromptDispatchStore;
  promptRecovery: SessionPromptRecovery;
}) {
  const queryClient = useQueryClient();
  const promptRuntime = useOptionalSessionPromptRuntimeContext();
  const [idempotencyKeys] = useState(() => new Map<string, string>());
  const attachmentAdapter = useSessionAttachmentAdapter(workspaceId, sessionId);
  const runtimeConfig = buildSessionRuntimeConfig(
    queryClient,
    workspaceId,
    sessionId,
    promptDispatch,
    promptRecovery,
    promptRuntime ? () => getSessionPromptRuntimeSnapshot(promptRuntime) : undefined,
    idempotencyKeys
  );

  return useChatRuntime({
    transport: runtimeConfig.transport,
    onError: () => {
      promptRecovery.recover(attachmentAdapter.recoverSentFiles());
    },
    onFinish: ({ isError }) => {
      if (!isError) {
        promptRecovery.acknowledge();
        attachmentAdapter.acknowledgeSentFiles();
      }
      runtimeConfig.onFinish();
    },
    adapters: { attachments: attachmentAdapter },
  });
}
