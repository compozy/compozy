import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef } from "react";

import { createClientId } from "@/lib/client-id";
import {
  archiveSession,
  cancelQueuedSessionPrompt,
  clearSessionConversation,
  createSession,
  type CreateSessionParams,
  deleteSession,
  repairSession,
  renameSession,
  resumeSession,
  SessionApiError,
  sendSessionPrompt,
  steerSessionPrompt,
  stopSession,
  unarchiveSession,
} from "../adapters/session-api";

import { sessionStore } from "../stores/session-store";
import { sessionKeys } from "../lib/query-keys";
import {
  invalidateSessionMutationQueries,
  invalidateWorkspaceSessionCatalog,
} from "../lib/session-query-invalidation";
import type {
  SessionPayload,
  SessionPromptPayload,
  SessionPromptAttachment,
  SessionPromptRequest,
  SessionPromptResult,
  SessionRepairQuery,
} from "../types";
import type { SessionPromptRuntimeSnapshot } from "../contexts/session-prompt-runtime-context-value";
import { useActiveWorkspace } from "@/systems/workspace";
import { createdInProfileToast, useProfileReadScope } from "@/systems/profiles";
import { notifyUser } from "@/lib/user-feedback";

function requireWorkspace(workspaceId: string | null | undefined): string {
  if (!workspaceId) {
    throw new SessionApiError("No active workspace selected", 400);
  }
  return workspaceId;
}

interface UseSessionWorkspaceOptions {
  workspaceId?: string | null;
}

export interface RenameSessionParams {
  id: string;
  name: string;
}

export function useRenameSession(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, runtimeWorkspaceId);
  const request = useAbortableMutationRequest();

  return useMutation({
    mutationFn: ({ id, name }: RenameSessionParams) =>
      request(signal => renameSession(requireWorkspace(workspaceId), id, { name }, signal)),
    onSuccess: session => {
      const successWorkspaceId = requireWorkspace(session.workspace_id);
      queryClient.setQueryData(sessionKeys.detail(successWorkspaceId, session.id), session);
    },
    onSettled: (_data, _error, variables) => {
      if (workspaceId) {
        void invalidateSessionMutationQueries(queryClient, workspaceId, variables.id);
      }
    },
  });
}

function resolveWorkspaceId(
  workspaceId: string | null | undefined,
  runtimeWorkspaceId: string | null | undefined
): string | null {
  return workspaceId ?? runtimeWorkspaceId ?? null;
}

function useAbortableMutationRequest() {
  const controllersRef = useRef(new Set<AbortController>());

  useEffect(
    () => () => {
      for (const controller of controllersRef.current) controller.abort();
      controllersRef.current.clear();
    },
    []
  );

  return async <T>(request: (signal: AbortSignal) => Promise<T>): Promise<T> => {
    const controller = new AbortController();
    controllersRef.current.add(controller);
    return Promise.resolve()
      .then(() => request(controller.signal))
      .finally(() => controllersRef.current.delete(controller));
  };
}

export function useCreateSession() {
  const queryClient = useQueryClient();
  // The destination is the acting profile — `default` while the aggregate is on,
  // which is exactly what the destination chip promised (ADR-005).
  const { aggregate, destination } = useProfileReadScope();

  return useMutation({
    mutationFn: (params: CreateSessionParams) => createSession(params, destination),
    onSuccess: session => {
      // Only under the aggregate: a scoped view already shows whose session this
      // is, so announcing it there would be noise. Here it is the guardrail that
      // makes a misfile visible immediately (US-012.AC-2).
      if (aggregate) {
        notifyUser({ message: createdInProfileToast(session.profile_name), tone: "success" });
      }
      const workspaceId = requireWorkspace(session.workspace_id);
      queryClient.setQueryData(sessionKeys.detail(workspaceId, session.id), session);
      void queryClient.invalidateQueries({ queryKey: sessionKeys.detail(workspaceId, session.id) });
      void invalidateWorkspaceSessionCatalog(queryClient, workspaceId);
    },
  });
}

export function useStopSession(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, runtimeWorkspaceId);

  return useMutation({
    mutationFn: (id: string) => stopSession(requireWorkspace(workspaceId), id),
    onSettled: (_data, _error, id) => {
      if (workspaceId) void invalidateSessionMutationQueries(queryClient, workspaceId, id);
    },
  });
}

export function useDeleteSession(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, runtimeWorkspaceId);

  return useMutation({
    mutationFn: (id: string) => deleteSession(requireWorkspace(workspaceId), id),
    onSuccess: (_data, id) => {
      const successWorkspaceId = requireWorkspace(workspaceId);
      sessionStore.trigger.sessionInteractionRemoved({ sessionId: id });
      queryClient.removeQueries({ queryKey: sessionKeys.detail(successWorkspaceId, id) });

      return invalidateWorkspaceSessionCatalog(queryClient, successWorkspaceId);
    },
  });
}

export function useArchiveSession(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, runtimeWorkspaceId);
  const request = useAbortableMutationRequest();

  return useMutation({
    mutationFn: (id: string) =>
      request(signal => archiveSession(requireWorkspace(workspaceId), id, signal)),
    onSettled: (_data, _error, id) => {
      if (workspaceId) void invalidateSessionMutationQueries(queryClient, workspaceId, id);
    },
  });
}

export function useUnarchiveSession(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, runtimeWorkspaceId);
  const request = useAbortableMutationRequest();

  return useMutation({
    mutationFn: (id: string) =>
      request(signal => unarchiveSession(requireWorkspace(workspaceId), id, signal)),
    onSettled: (_data, _error, id) => {
      if (workspaceId) void invalidateSessionMutationQueries(queryClient, workspaceId, id);
    },
  });
}

export function useResumeSession(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, runtimeWorkspaceId);

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
  attachments?: SessionPromptAttachment[];
  expectedTurnId?: string;
  idempotencyKey?: string;
  id: string;
  messageId?: string;
  message: string;
  runtime?: SessionPromptRuntimeSnapshot;
}

export interface SendSessionPromptParams extends SessionPromptActionParams {
  mode?: SessionPromptRequest["mode"];
}

export interface CancelQueuedSessionPromptParams {
  id: string;
  queueEntryId: string;
}

function createPromptIdentity(): { idempotencyKey: string; messageId: string } {
  return {
    idempotencyKey: createClientId(),
    messageId: createClientId(),
  };
}

function requireExpectedTurnId(expectedTurnId: string | undefined): string {
  const value = expectedTurnId?.trim() ?? "";
  if (value.length === 0) {
    throw new Error("A session steer or interrupt action requires a non-empty expected_turn_id");
  }
  return value;
}

const actionPromptIdentities = new WeakMap<
  SessionPromptActionParams,
  {
    idempotencyKey: string;
    messageId: string;
  }
>();

function promptIdentityForAction(params: SessionPromptActionParams): {
  idempotencyKey: string;
  messageId: string;
} {
  if (params.messageId !== undefined || params.idempotencyKey !== undefined) {
    if (
      typeof params.messageId !== "string" ||
      params.messageId.trim().length === 0 ||
      typeof params.idempotencyKey !== "string" ||
      params.idempotencyKey.trim().length === 0
    ) {
      throw new Error(
        "A session prompt action requires both non-empty message_id and idempotency_key"
      );
    }
    return { idempotencyKey: params.idempotencyKey, messageId: params.messageId };
  }
  const existing = actionPromptIdentities.get(params);
  if (existing) return existing;
  const identity = createPromptIdentity();
  actionPromptIdentities.set(params, identity);
  return identity;
}

function promptRequestFromAction(
  params: SessionPromptActionParams,
  mode?: SendSessionPromptParams["mode"]
): SessionPromptRequest {
  const identity = promptIdentityForAction(params);
  return {
    ...(params.attachments?.length ? { attachments: params.attachments } : {}),
    idempotency_key: identity.idempotencyKey,
    message_id: identity.messageId,
    messages: [
      {
        id: identity.messageId,
        parts: [{ text: params.message, type: "text" }],
        role: "user",
      },
    ],
    ...(mode ? { mode } : {}),
    ...(params.expectedTurnId ? { expected_turn_id: params.expectedTurnId } : {}),
    ...(params.runtime ? { runtime: params.runtime } : {}),
  };
}

export function useSendSessionPrompt(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, runtimeWorkspaceId);

  return useMutation<SessionPromptResult, Error, SendSessionPromptParams>({
    mutationFn: params =>
      sendSessionPrompt(
        requireWorkspace(workspaceId),
        params.id,
        promptRequestFromAction(params, params.mode)
      ),
    onSettled: (_data, _error, params) => {
      if (workspaceId) void invalidateSessionMutationQueries(queryClient, workspaceId, params.id);
    },
  });
}

export function useQueueSessionPrompt(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, runtimeWorkspaceId);

  return useMutation<SessionPromptResult, Error, SessionPromptActionParams>({
    mutationFn: params =>
      sendSessionPrompt(
        requireWorkspace(workspaceId),
        params.id,
        promptRequestFromAction(params, "queue")
      ),
    onSettled: (_data, _error, params) => {
      if (workspaceId) void invalidateSessionMutationQueries(queryClient, workspaceId, params.id);
    },
  });
}

export function useInterruptSessionPrompt(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, runtimeWorkspaceId);

  return useMutation<SessionPromptResult, Error, SessionPromptActionParams>({
    mutationFn: params => {
      const expectedTurnId = requireExpectedTurnId(params.expectedTurnId);
      return sendSessionPrompt(
        requireWorkspace(workspaceId),
        params.id,
        promptRequestFromAction({ ...params, expectedTurnId }, "interrupt")
      );
    },
    onSettled: (_data, _error, params) => {
      if (workspaceId) void invalidateSessionMutationQueries(queryClient, workspaceId, params.id);
    },
  });
}

export function useSteerSessionPrompt(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, runtimeWorkspaceId);

  return useMutation<SessionPromptPayload, Error, SessionPromptActionParams>({
    mutationFn: params => {
      const expectedTurnId = requireExpectedTurnId(params.expectedTurnId);
      const identity = promptIdentityForAction(params);
      return steerSessionPrompt(requireWorkspace(workspaceId), params.id, {
        expected_turn_id: expectedTurnId,
        idempotency_key: identity.idempotencyKey,
        message_id: identity.messageId,
        text: params.message,
      });
    },
    onSettled: (_data, _error, params) => {
      if (workspaceId) void invalidateSessionMutationQueries(queryClient, workspaceId, params.id);
    },
  });
}

export function useCancelQueuedSessionPrompt(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, runtimeWorkspaceId);

  return useMutation<SessionPromptPayload, Error, CancelQueuedSessionPromptParams>({
    mutationFn: ({ id, queueEntryId }) =>
      cancelQueuedSessionPrompt(requireWorkspace(workspaceId), id, queueEntryId),
    onSettled: (_data, _error, params) => {
      if (workspaceId) void invalidateSessionMutationQueries(queryClient, workspaceId, params.id);
    },
  });
}
