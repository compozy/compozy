import {
  useIsMutating,
  useMutation,
  useMutationState,
  useQueryClient,
} from "@tanstack/react-query";

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
  SessionApiError,
  SessionGoalCommandError,
  sendSessionPrompt,
  stopSession,
  unarchiveSession,
} from "../adapters/session-api";

import { sessionStore } from "../stores/session-store";
import { sessionKeys } from "../lib/query-keys";
import { discardComposerDraftAttachments } from "../lib/session-composer-attachment-drafts";
import {
  invalidateSessionMutationQueries,
  invalidateWorkspaceSessionCatalog,
} from "../lib/session-query-invalidation";
import type {
  SessionPayload,
  SessionPromptAttachment,
  SessionPromptRequest,
  SessionPromptPayload,
  SessionPromptSendResult,
  SessionRepairQuery,
} from "../types";
import type { SessionPromptRuntimeSnapshot } from "../contexts/session-prompt-runtime-context-value";
import {
  ownerDataScope,
  useActiveWorkspace,
  workspaceBadgeIdentity,
  workspaceSelectorForScope,
} from "@/systems/workspace";
import { useProfileReadScope } from "@/systems/profiles";
import { sessionStartedToast } from "../lib/session-create-target";
import { notifyUser } from "@/lib/user-feedback";
import { useAbortableMutationRequest } from "./use-abortable-mutation-request";
import type { SessionProcessPolicy } from "../adapters/session-processes-api";

/**
 * The session's own workspace path segment. An empty `workspace_id` is a User
 * session, addressed through `@user`; only an absent value has no target.
 */
function requireWorkspace(workspaceId: string | null | undefined): string {
  if (workspaceId === null || workspaceId === undefined) {
    throw new SessionApiError("No active workspace selected", 400);
  }
  return workspaceSelectorForScope(ownerDataScope(workspaceId));
}

interface UseSessionWorkspaceOptions {
  workspaceId?: string | null;
}

export type SessionActionTarget = Pick<SessionPayload, "id" | "workspace_id" | "profile_name">;

export interface RenameSessionParams extends SessionActionTarget {
  name: string;
}

/** Renames the supplied row and invalidates its owning workspace after settlement. */
export function useRenameSession() {
  const queryClient = useQueryClient();
  const request = useAbortableMutationRequest();

  return useMutation({
    mutationFn: ({ id, workspace_id, profile_name, name }: RenameSessionParams) =>
      request(signal =>
        renameSession(requireWorkspace(workspace_id), id, profile_name, { name }, signal)
      ),
    onSuccess: session => {
      const successWorkspaceId = requireWorkspace(session.workspace_id);
      queryClient.setQueryData(sessionKeys.detail(successWorkspaceId, session.id), session);
    },
    onSettled: (_data, _error, variables) => {
      if (variables.workspace_id) {
        void invalidateSessionMutationQueries(queryClient, variables.workspace_id, variables.id);
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

/** A create request plus the profile that owns it; omitted files into the acting profile. */
export type CreateSessionVariables = CreateSessionParams & { profile?: string };

export function useCreateSession() {
  const queryClient = useQueryClient();
  // The default owner is the acting profile — the Profile picker overrides it
  // under the aggregate (ADR-003).
  const { aggregate, destination } = useProfileReadScope();
  const { breadthMode, registeredWorkspaces } = useActiveWorkspace();

  return useMutation({
    mutationFn: ({ profile, ...params }: CreateSessionVariables) =>
      createSession(params, profile ?? destination),
    onSuccess: session => {
      // Only while an axis is aggregated: a scoped view already shows where the
      // session is, so announcing it there would be noise. Here it is the
      // guardrail that makes a misfile visible immediately (ADR-003).
      if (aggregate || breadthMode === "all") {
        const projectLabel = workspaceBadgeIdentity(
          session.workspace_id,
          new Map(registeredWorkspaces.map(workspace => [workspace.id, workspace.name]))
        ).name;
        notifyUser({
          message: sessionStartedToast(projectLabel, session.profile_name),
          tone: "success",
        });
      }
      // A User session reads and lists through `@user`.
      const workspaceId = requireWorkspace(session.workspace_id ?? "");
      queryClient.setQueryData(sessionKeys.detail(workspaceId, session.id), session);
      void queryClient.invalidateQueries({ queryKey: sessionKeys.detail(workspaceId, session.id) });
      void invalidateWorkspaceSessionCatalog(queryClient, workspaceId);
    },
  });
}

export interface StopSessionParams extends Pick<
  SessionPayload,
  "id" | "workspace_id" | "profile_name"
> {
  /** Wait for the settled outcome instead of the 202 acceptance. */
  wait?: boolean;
  /** Stop or keep the processes the session left running; omitted keeps them. */
  processes?: SessionProcessPolicy;
}

export function useStopSession() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, workspace_id, profile_name, wait, processes }: StopSessionParams) =>
      stopSession(requireWorkspace(workspace_id), id, profile_name, {
        wait,
        ...(processes ? { processes } : {}),
      }),
    onSettled: (_data, _error, { id, workspace_id }) => {
      if (workspace_id) void invalidateSessionMutationQueries(queryClient, workspace_id, id);
    },
  });
}

/** Keep remote disappearance recovery from racing an operator-owned deletion. */
export function useSessionDeletionPending(sessionId: string | null): boolean {
  return (
    useIsMutating({
      mutationKey: sessionKeys.deletions(),
      predicate: mutation => {
        const variables = mutation.state.variables;
        return (
          typeof variables === "object" &&
          variables !== null &&
          "id" in variables &&
          variables.id === sessionId
        );
      },
    }) > 0
  );
}

/**
 * True once this client deleted the session: a window still showing it after an
 * operator-owned deletion is retired quietly, never reported as a missing session.
 */
export function useSessionDeletedByOperator(sessionId: string | null): boolean {
  return (
    useMutationState({
      filters: {
        mutationKey: sessionKeys.deletions(),
        status: "success",
        predicate: mutation => {
          const variables = mutation.state.variables;
          return (
            typeof variables === "object" &&
            variables !== null &&
            "id" in variables &&
            variables.id === sessionId
          );
        },
      },
    }).length > 0
  );
}

/** Deletes the captured owner-scoped target and reconciles detail, catalog, and attention caches. */
export interface DeleteSessionParams extends Pick<
  SessionPayload,
  "id" | "workspace_id" | "profile_name"
> {
  /** Stop or keep the processes the session left running; omitted keeps them. */
  processes?: SessionProcessPolicy;
}

export function useDeleteSession(
  options: { onDeleteSuccess?: (session: SessionActionTarget) => void | Promise<void> } = {}
) {
  const queryClient = useQueryClient();

  // Capture ownership with the target, not the active profile/workspace at confirmation time.
  return useMutation({
    mutationKey: sessionKeys.deletions(),
    mutationFn: ({ processes, ...session }: DeleteSessionParams) => {
      const workspaceId = requireWorkspace(session.workspace_id);
      // Omitted keeps the processes, the daemon default, so only an answer is sent.
      return processes
        ? deleteSession(workspaceId, session.id, session.profile_name, undefined, processes)
        : deleteSession(workspaceId, session.id, session.profile_name);
    },
    onMutate: async ({ id, workspace_id }) => {
      const deleteWorkspaceId = requireWorkspace(workspace_id);
      sessionStore.trigger.sessionLiveTailSuspended({ sessionId: id });
      await Promise.all([
        queryClient.cancelQueries({ queryKey: sessionKeys.detail(deleteWorkspaceId, id) }),
        queryClient.cancelQueries({ queryKey: sessionKeys.byIdRoot(id) }),
      ]);
    },
    onSuccess: async (_data, session) => {
      const { id, workspace_id } = session;
      const successWorkspaceId = requireWorkspace(workspace_id);
      await options.onDeleteSuccess?.(session);
      discardComposerDraftAttachments(id);
      sessionStore.trigger.sessionInteractionRemoved({ sessionId: id });
      queryClient.removeQueries({ queryKey: sessionKeys.detail(successWorkspaceId, id) });
      queryClient.removeQueries({ queryKey: sessionKeys.byIdRoot(id) });
      void invalidateWorkspaceSessionCatalog(queryClient, successWorkspaceId);
      void queryClient.invalidateQueries({ queryKey: sessionKeys.workspaceLists("") });
      void queryClient.invalidateQueries({ queryKey: sessionKeys.attentionSummary() });
    },
    onSettled: (_data, error, { id, workspace_id }) => {
      sessionStore.trigger.sessionLiveTailResumed({ sessionId: id });
      if (error && workspace_id) {
        void invalidateSessionMutationQueries(queryClient, workspace_id, id);
      }
    },
  });
}

export interface ArchiveSessionParams extends SessionActionTarget {
  /** Stop a live session through the daemon's normal stop path before archiving it. */
  stop?: boolean;
  /** Stop or keep the processes the session left running; omitted keeps them. */
  processes?: SessionProcessPolicy;
}

/** Archives the supplied row without depending on the currently selected workspace. */
export function useArchiveSession() {
  const queryClient = useQueryClient();
  const request = useAbortableMutationRequest();

  return useMutation({
    mutationFn: ({ id, workspace_id, profile_name, stop, processes }: ArchiveSessionParams) =>
      request(signal =>
        processes
          ? archiveSession(
              requireWorkspace(workspace_id),
              id,
              profile_name,
              signal,
              stop ?? false,
              processes
            )
          : archiveSession(requireWorkspace(workspace_id), id, profile_name, signal, stop ?? false)
      ),
    onSettled: (_data, _error, { id, workspace_id }) => {
      if (workspace_id) void invalidateSessionMutationQueries(queryClient, workspace_id, id);
    },
  });
}

/** Restores the supplied row and refreshes the catalog in its owning workspace. */
export function useUnarchiveSession() {
  const queryClient = useQueryClient();
  const request = useAbortableMutationRequest();

  return useMutation({
    mutationFn: ({ id, workspace_id, profile_name }: SessionActionTarget) =>
      request(signal => unarchiveSession(requireWorkspace(workspace_id), id, profile_name, signal)),
    onSettled: (_data, _error, { id, workspace_id }) => {
      if (workspace_id) void invalidateSessionMutationQueries(queryClient, workspace_id, id);
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
      queryClient.setQueryData(sessionKeys.transcript(workspaceId, id), {
        pages: [],
        pageParams: [],
      });
      void queryClient.invalidateQueries({
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

/**
 * The one busy-send mutation: `mode` names the verb (steer | queue | interrupt)
 * the composer resolved — from the daemon default, the one-shot modifier, or an
 * explicit button. `expectedTurnId` is a strict fence when provided; omitted, the
 * daemon resolves the active turn at admission and echoes it (invariant 6).
 */
export function useSendSessionPrompt(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, runtimeWorkspaceId);

  return useMutation<SessionPromptSendResult, Error, SendSessionPromptParams>({
    mutationFn: params =>
      sendSessionPrompt(
        requireWorkspace(workspaceId),
        params.id,
        promptRequestFromAction(params, params.mode)
      ),
    /** Projects typed Goal outcomes before refreshing authoritative session reads. */
    onSettled: (data, error, params) => {
      const goal =
        error instanceof SessionGoalCommandError
          ? error.result
          : data && "outcome" in data
            ? data
            : null;
      if (goal) {
        sessionStore.trigger.goalCommandReported({
          sessionId: params.id,
          command: params.message,
          result: goal,
        });
        if (workspaceId && (goal.snapshot !== null || goal.outcome === "cleared")) {
          queryClient.setQueryData(sessionKeys.goal(workspaceId, params.id), {
            goal: goal.snapshot,
          });
        }
      }
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
