import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  cancelQueuedSessionPrompt,
  clearSessionConversation,
  createSession,
  type CreateSessionParams,
  deleteSession,
  repairSession,
  resumeSession,
  SessionApiError,
  sendSessionPrompt,
  steerSessionPrompt,
  stopSession,
} from "../adapters/session-api";
import { useActiveWorkspace } from "@/systems/workspace";
import { useSessionStore } from "./use-session-store";
import { sessionKeys } from "../lib/query-keys";
import {
  invalidateSessionMutationQueries,
  invalidateWorkspaceSessionCatalog,
} from "../lib/session-query-invalidation";
import type {
  SessionPayload,
  SessionPromptPayload,
  SessionPromptRequest,
  SessionPromptResult,
  SessionRepairQuery,
} from "../types";

function requireWorkspace(workspaceId: string | null | undefined): string {
  if (!workspaceId) {
    throw new SessionApiError("No active workspace selected", 400);
  }
  return workspaceId;
}

interface UseSessionWorkspaceOptions {
  workspaceId?: string | null;
}

function resolveWorkspaceId(
  workspaceId: string | null | undefined,
  activeWorkspaceId: string | null | undefined
): string | null {
  return workspaceId ?? activeWorkspaceId ?? null;
}

export function useCreateSession() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (params: CreateSessionParams) => createSession(params),
    onSuccess: session => {
      const workspaceId = requireWorkspace(session.workspace_id);
      queryClient.setQueryData(sessionKeys.detail(workspaceId, session.id), session);
      queryClient.setQueryData(sessionKeys.byId(session.id), session);
      void queryClient.invalidateQueries({ queryKey: sessionKeys.detail(workspaceId, session.id) });
      void invalidateWorkspaceSessionCatalog(queryClient, workspaceId);
    },
  });
}

export function useStopSession(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, activeWorkspaceId);

  return useMutation({
    mutationFn: (id: string) => stopSession(requireWorkspace(workspaceId), id),
    onSettled: (_data, _error, id) => {
      if (workspaceId) void invalidateSessionMutationQueries(queryClient, workspaceId, id);
    },
  });
}

export function useDeleteSession(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, activeWorkspaceId);

  return useMutation({
    mutationFn: (id: string) => deleteSession(requireWorkspace(workspaceId), id),
    onSuccess: (_data, id) => {
      const successWorkspaceId = requireWorkspace(workspaceId);
      useSessionStore.getState().clearDraft(id);
      queryClient.removeQueries({ queryKey: sessionKeys.detail(successWorkspaceId, id) });
      queryClient.removeQueries({ queryKey: sessionKeys.byId(id), exact: true });

      return invalidateWorkspaceSessionCatalog(queryClient, successWorkspaceId);
    },
  });
}

export function useResumeSession(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, activeWorkspaceId);

  return useMutation({
    mutationFn: (id: string) => resumeSession(requireWorkspace(workspaceId), id),
    onSettled: (_data, _error, id) => {
      if (workspaceId) void invalidateSessionMutationQueries(queryClient, workspaceId, id);
    },
  });
}

export interface RepairSessionParams extends SessionRepairQuery {
  id: string;
}

export function useRepairSession(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, activeWorkspaceId);

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
  const { activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, activeWorkspaceId);

  return useMutation({
    mutationKey: sessionKeys.clearConversation(workspaceId ?? ""),
    mutationFn: (id: string) => clearSessionConversation(requireWorkspace(workspaceId), id),
    onMutate: async (id): Promise<ClearConversationSnapshot> => {
      const mutateWorkspaceId = requireWorkspace(workspaceId);
      await queryClient.cancelQueries({ queryKey: sessionKeys.detail(mutateWorkspaceId, id) });
      await queryClient.cancelQueries({ queryKey: sessionKeys.history(mutateWorkspaceId, id) });
      await queryClient.cancelQueries({ queryKey: sessionKeys.transcript(mutateWorkspaceId, id) });

      const snapshot: ClearConversationSnapshot = {
        session: queryClient.getQueryData<SessionPayload>(
          sessionKeys.detail(mutateWorkspaceId, id)
        ),
        transcript: queryClient.getQueryData(sessionKeys.transcript(mutateWorkspaceId, id)),
        history: queryClient.getQueryData(sessionKeys.history(mutateWorkspaceId, id)),
      };

      queryClient.removeQueries({
        queryKey: sessionKeys.transcript(mutateWorkspaceId, id),
        exact: true,
      });
      queryClient.setQueryData(sessionKeys.history(mutateWorkspaceId, id), []);

      return snapshot;
    },
    onError: (_error, id, snapshot) => {
      if (!snapshot) {
        return;
      }

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
      const workspaceId = requireWorkspace(session.workspace_id);
      queryClient.setQueryData(sessionKeys.detail(workspaceId, id), session);
      queryClient.removeQueries({
        queryKey: sessionKeys.transcript(workspaceId, id),
        exact: true,
      });
      queryClient.setQueryData(sessionKeys.history(workspaceId, id), []);
    },
    onSettled: (_data, _error, id) => {
      if (workspaceId) void invalidateSessionMutationQueries(queryClient, workspaceId, id);
    },
  });
}

export interface SessionPromptActionParams {
  id: string;
  message: string;
}

export interface SendSessionPromptParams extends SessionPromptActionParams {
  mode?: SessionPromptRequest["mode"];
}

export interface CancelQueuedSessionPromptParams {
  id: string;
  queueEntryId: string;
}

export function useSendSessionPrompt(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, activeWorkspaceId);

  return useMutation<SessionPromptResult, Error, SendSessionPromptParams>({
    mutationFn: ({ id, message, mode }) =>
      sendSessionPrompt(requireWorkspace(workspaceId), id, { message, mode }),
    onSettled: (_data, _error, params) => {
      if (workspaceId) void invalidateSessionMutationQueries(queryClient, workspaceId, params.id);
    },
  });
}

export function useQueueSessionPrompt(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, activeWorkspaceId);

  return useMutation<SessionPromptResult, Error, SessionPromptActionParams>({
    mutationFn: ({ id, message }) =>
      sendSessionPrompt(requireWorkspace(workspaceId), id, { message, mode: "queue" }),
    onSettled: (_data, _error, params) => {
      if (workspaceId) void invalidateSessionMutationQueries(queryClient, workspaceId, params.id);
    },
  });
}

export function useInterruptSessionPrompt(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, activeWorkspaceId);

  return useMutation<SessionPromptResult, Error, SessionPromptActionParams>({
    mutationFn: ({ id, message }) =>
      sendSessionPrompt(requireWorkspace(workspaceId), id, { message, mode: "interrupt" }),
    onSettled: (_data, _error, params) => {
      if (workspaceId) void invalidateSessionMutationQueries(queryClient, workspaceId, params.id);
    },
  });
}

export function useSteerSessionPrompt(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, activeWorkspaceId);

  return useMutation<SessionPromptPayload, Error, SessionPromptActionParams>({
    mutationFn: ({ id, message }) => steerSessionPrompt(requireWorkspace(workspaceId), id, message),
    onSettled: (_data, _error, params) => {
      if (workspaceId) void invalidateSessionMutationQueries(queryClient, workspaceId, params.id);
    },
  });
}

export function useCancelQueuedSessionPrompt(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, activeWorkspaceId);

  return useMutation<SessionPromptPayload, Error, CancelQueuedSessionPromptParams>({
    mutationFn: ({ id, queueEntryId }) =>
      cancelQueuedSessionPrompt(requireWorkspace(workspaceId), id, queueEntryId),
    onSettled: (_data, _error, params) => {
      if (workspaceId) void invalidateSessionMutationQueries(queryClient, workspaceId, params.id);
    },
  });
}
