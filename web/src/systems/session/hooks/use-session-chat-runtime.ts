import { startTransition } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { AssistantChatTransport, useChatRuntime } from "@assistant-ui/react-ai-sdk";

import { loopsKeys } from "@/systems/loops";
import { sessionKeys } from "../lib/query-keys";
import { invalidateSessionMutationQueries } from "../lib/session-query-invalidation";
import { createGoalAwareFetch } from "../lib/session-goal-chat-transport";
import { useSessionStore } from "./use-session-store";

type QueryClient = ReturnType<typeof useQueryClient>;

function buildSessionRuntimeConfig(
  queryClient: QueryClient,
  workspaceId: string,
  sessionId: string
) {
  const goalAwareFetch = createGoalAwareFetch({
    onRequest: () => {
      useSessionStore.getState().dismissGoalError(sessionId);
    },
    onResult: (result, requestText) => {
      useSessionStore.getState().setGoalResult(sessionId, result, requestText ?? undefined);
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
  return {
    transport: new AssistantChatTransport({
      api: `/api/workspaces/${encodeURIComponent(workspaceId)}/sessions/${encodeURIComponent(sessionId)}/prompt`,
      fetch: goalAwareFetch,
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
}: {
  sessionId: string;
  workspaceId: string;
}) {
  const queryClient = useQueryClient();
  const runtimeConfig = buildSessionRuntimeConfig(queryClient, workspaceId, sessionId);

  return useChatRuntime({
    transport: runtimeConfig.transport,
    onFinish: runtimeConfig.onFinish,
  });
}
