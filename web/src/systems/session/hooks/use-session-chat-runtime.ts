import { startTransition, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useChatRuntime } from "@assistant-ui/react-ai-sdk";

import { loopsKeys } from "@/systems/loops";
import type { SessionPromptDispatchStore } from "@/components/assistant-ui/session-prompt-dispatch-store";
import { sessionKeys } from "../lib/query-keys";
import { invalidateSessionMutationQueries } from "../lib/session-query-invalidation";
import { createGoalAwareFetch } from "../lib/session-goal-chat-transport";
import { createSessionPromptChatTransport } from "../lib/session-prompt-chat-transport";
import { sessionStore } from "../stores/session-store";
import type { SessionPromptRuntimeSnapshot } from "../contexts/session-prompt-runtime-context-value";
import { getSessionPromptRuntimeSnapshot } from "./use-session-prompt-runtime";
import { useOptionalSessionPromptRuntimeContext } from "./use-session-prompt-runtime-context";

type QueryClient = ReturnType<typeof useQueryClient>;

function buildSessionRuntimeConfig(
  queryClient: QueryClient,
  workspaceId: string,
  sessionId: string,
  promptDispatch: SessionPromptDispatchStore,
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
    try {
      return await goalAwareFetch(input, { ...init, signal: controller.signal });
    } finally {
      upstreamSignal?.removeEventListener("abort", abortFromUpstream);
      promptDispatch.trigger.requestCompleted({ controller });
    }
  };
  return {
    transport: createSessionPromptChatTransport({
      api: `/api/workspaces/${encodeURIComponent(workspaceId)}/sessions/${encodeURIComponent(sessionId)}/prompt`,
      fetch: trackedFetch,
      ...(getRuntimeSnapshot ? { getRuntimeSnapshot } : {}),
      ...(idempotencyKeys ? { idempotencyKeys } : {}),
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
}: {
  sessionId: string;
  workspaceId: string;
  promptDispatch: SessionPromptDispatchStore;
}) {
  const queryClient = useQueryClient();
  const promptRuntime = useOptionalSessionPromptRuntimeContext();
  const [idempotencyKeys] = useState(() => new Map<string, string>());
  const runtimeConfig = buildSessionRuntimeConfig(
    queryClient,
    workspaceId,
    sessionId,
    promptDispatch,
    promptRuntime ? () => getSessionPromptRuntimeSnapshot(promptRuntime) : undefined,
    idempotencyKeys
  );

  return useChatRuntime({
    transport: runtimeConfig.transport,
    onFinish: runtimeConfig.onFinish,
  });
}
