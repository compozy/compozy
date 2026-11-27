import { useMutation, useQueryClient } from "@tanstack/react-query";

import { useActiveWorkspace } from "@/systems/workspace";

import { clearSessionConversation, repairSession } from "../adapters/session-api";
import { sessionKeys } from "../lib/query-keys";
import { invalidateSessionMutationQueries } from "../lib/session-query-invalidation";
import type { SessionPayload, SessionRepairQuery } from "../types";
import {
  requireWorkspace,
  resolveWorkspaceId,
  type UseSessionWorkspaceOptions,
} from "./session-action-workspace";

export interface RepairSessionParams extends SessionRepairQuery {
  id: string;
}

export function useRepairSession(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, runtimeWorkspaceId);

  return useMutation({
    mutationFn: ({ id, ...query }: RepairSessionParams) =>
      repairSession(requireWorkspace(workspaceId), id, query),
    onSettled: (_data, _error, params) => {
      if (workspaceId) {
        void invalidateSessionMutationQueries(queryClient, workspaceId, params.id);
      }
    },
  });
}

interface ClearConversationSnapshot {
  session: SessionPayload | undefined;
  transcript: unknown;
  history: unknown;
}

export function useClearSessionConversation(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, runtimeWorkspaceId);

  return useMutation({
    mutationKey: sessionKeys.clearConversation(workspaceId ?? ""),
    mutationFn: (id: string) => clearSessionConversation(requireWorkspace(workspaceId), id),
    onMutate: async (id): Promise<ClearConversationSnapshot> => {
      const mutateWorkspaceId = requireWorkspace(workspaceId);
      await queryClient.cancelQueries({ queryKey: sessionKeys.detail(mutateWorkspaceId, id) });
      await queryClient.cancelQueries({ queryKey: sessionKeys.history(mutateWorkspaceId, id) });
      await queryClient.cancelQueries({ queryKey: sessionKeys.transcript(mutateWorkspaceId, id) });

      return {
        session: queryClient.getQueryData<SessionPayload>(
          sessionKeys.detail(mutateWorkspaceId, id)
        ),
        transcript: queryClient.getQueryData(sessionKeys.transcript(mutateWorkspaceId, id)),
        history: queryClient.getQueryData(sessionKeys.history(mutateWorkspaceId, id)),
      };
    },
    onError: (_error, id, snapshot) => {
      if (!snapshot) return;

      if (snapshot.session) {
        const snapshotWorkspaceId = requireWorkspace(snapshot.session.workspace_id);
        queryClient.setQueryData(
          sessionKeys.detail(snapshotWorkspaceId, snapshot.session.id),
          snapshot.session
        );
      }

      const errorWorkspaceId = requireWorkspace(workspaceId);
      if (snapshot.transcript !== undefined) {
        queryClient.setQueryData(sessionKeys.transcript(errorWorkspaceId, id), snapshot.transcript);
      }
      queryClient.setQueryData(sessionKeys.history(errorWorkspaceId, id), snapshot.history);
    },
    onSuccess: (session, id) => {
      const targetWorkspaceId = requireWorkspace(session.workspace_id);
      queryClient.setQueryData(sessionKeys.detail(targetWorkspaceId, id), session);
      queryClient.setQueryData(sessionKeys.transcript(targetWorkspaceId, id), {
        pages: [],
        pageParams: [],
      });
      void queryClient.invalidateQueries({
        queryKey: sessionKeys.transcript(targetWorkspaceId, id),
        exact: true,
      });
      queryClient.setQueryData(sessionKeys.history(targetWorkspaceId, id), []);
    },
    onSettled: (_data, _error, id) => {
      if (workspaceId) void invalidateSessionMutationQueries(queryClient, workspaceId, id);
    },
  });
}
