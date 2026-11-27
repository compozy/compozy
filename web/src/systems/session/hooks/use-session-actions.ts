import {
  useIsMutating,
  useMutation,
  useMutationState,
  useQueryClient,
} from "@tanstack/react-query";

import {
  archiveSession,
  archiveSessionTree,
  createSession,
  type CreateSessionParams,
  deleteSession,
  deleteSessionTree,
  renameSession,
  stopSession,
  unarchiveSession,
} from "../adapters/session-api";

export {
  useClearSessionConversation,
  useRepairSession,
  type RepairSessionParams,
} from "./use-session-maintenance-actions";

import { sessionStore } from "../stores/session-store";
import { sessionKeys } from "../lib/query-keys";
import { discardComposerDraftAttachments } from "../lib/session-composer-attachment-drafts";
import {
  invalidateSessionMutationQueries,
  invalidateWorkspaceSessionCatalog,
} from "../lib/session-query-invalidation";
import type { SessionPayload } from "../types";
import { useActiveWorkspace, workspaceBadgeIdentity } from "@/systems/workspace";
import { useProfileReadScope } from "@/systems/profiles";
import { sessionStartedToast } from "../lib/session-create-target";
import {
  isAutoComputerId,
  isRemoteComputerId,
  normalizeComputerId,
  type ComputerId,
} from "@/lib/computer-scope";
import { notifyUser } from "@/lib/user-feedback";
import { useAbortableMutationRequest } from "./use-abortable-mutation-request";
import { requireWorkspace } from "./session-action-workspace";
import type { SessionProcessPolicy } from "../adapters/session-processes-api";

/** A captured session without a project belongs to User, unlike an absent UI selection. */
function sessionOwnerWorkspace(workspaceId: string | null | undefined): string {
  return requireWorkspace(workspaceId ?? "");
}

export type SessionActionTarget = Pick<SessionPayload, "id" | "workspace_id" | "profile_name"> & {
  computerId?: ComputerId;
};

export interface RenameSessionParams extends SessionActionTarget {
  name: string;
}

/** Renames the supplied row and invalidates its owning workspace after settlement. */
export function useRenameSession() {
  const queryClient = useQueryClient();
  const request = useAbortableMutationRequest();

  return useMutation({
    mutationFn: ({ id, workspace_id, profile_name, name, computerId }: RenameSessionParams) =>
      request(signal =>
        isRemoteComputerId(computerId)
          ? renameSession(
              sessionOwnerWorkspace(workspace_id),
              id,
              profile_name,
              { name },
              signal,
              computerId
            )
          : renameSession(sessionOwnerWorkspace(workspace_id), id, profile_name, { name }, signal)
      ),
    onSuccess: (session, { computerId }) => {
      const successWorkspaceId = sessionOwnerWorkspace(session.workspace_id);
      queryClient.setQueryData(
        sessionKeys.forComputer(computerId).detail(successWorkspaceId, session.id),
        session
      );
    },
    onSettled: (_data, _error, variables) => {
      void invalidateSessionMutationQueries(
        queryClient,
        sessionOwnerWorkspace(variables.workspace_id),
        variables.id,
        variables.computerId
      );
    },
  });
}

/** A create request plus the profile and target computer that own it. */
export type CreateSessionVariables = CreateSessionParams & {
  profile?: string;
  computerId?: ComputerId;
};

export function useCreateSession() {
  const queryClient = useQueryClient();
  // The default owner is the acting profile — the Profile picker overrides it
  // under the aggregate (ADR-003).
  const { aggregate, destination } = useProfileReadScope();
  const { breadthMode, registeredWorkspaces } = useActiveWorkspace();

  return useMutation({
    mutationFn: ({ profile, computerId, ...params }: CreateSessionVariables) =>
      computerId && !isAutoComputerId(computerId)
        ? createSession(params, profile ?? destination, undefined, computerId)
        : createSession(params, profile ?? destination),
    onSuccess: (session, variables) => {
      const scopedComputerId = normalizeComputerId(
        session.computer_id ??
          (isAutoComputerId(variables.computer) ? undefined : variables.computerId)
      );
      const keys = sessionKeys.forComputer(scopedComputerId);
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
      queryClient.setQueryData(keys.detail(workspaceId, session.id), session);
      void queryClient.invalidateQueries({ queryKey: keys.detail(workspaceId, session.id) });
      if (scopedComputerId === "local") {
        void invalidateWorkspaceSessionCatalog(queryClient, workspaceId);
      } else {
        void queryClient.invalidateQueries({ queryKey: keys.workspaceLists(workspaceId) });
        void queryClient.invalidateQueries({ queryKey: keys.workspaceLists("") });
      }
    },
  });
}

export interface StopSessionParams extends SessionActionTarget {
  /** Wait for the settled outcome instead of the 202 acceptance. */
  wait?: boolean;
  /** Stop or keep the processes the session left running; omitted keeps them. */
  processes?: SessionProcessPolicy;
}

/** Stops the captured session in its owner scope and refreshes owner and aggregate reads. */
export function useStopSession() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      id,
      workspace_id,
      profile_name,
      wait,
      processes,
      computerId,
    }: StopSessionParams) => {
      const options = { wait, ...(processes ? { processes } : {}) };
      return isRemoteComputerId(computerId)
        ? stopSession(sessionOwnerWorkspace(workspace_id), id, profile_name, options, computerId)
        : stopSession(sessionOwnerWorkspace(workspace_id), id, profile_name, options);
    },
    onSettled: (_data, _error, { id, workspace_id, computerId }) => {
      void invalidateSessionMutationQueries(
        queryClient,
        sessionOwnerWorkspace(workspace_id),
        id,
        computerId
      );
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

/** Catalog delivery can precede the archive response; keep only its request owner mounted. */
export function useSessionArchivePending(requestOwnerId: string): boolean {
  return (
    useIsMutating({
      mutationKey: sessionKeys.archives(),
      predicate: mutation => mutation.meta?.requestOwnerId === requestOwnerId,
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

/** Captured session ownership and the optional process disposition for deletion. */
export interface DeleteSessionParams extends SessionActionTarget {
  /** Stop or keep the processes the session left running; omitted keeps them. */
  processes?: SessionProcessPolicy;
  /** The children known when asked; present opts into deleting the daemon's current tree. */
  descendants?: readonly SessionPayload[];
}

/** Deletes the captured owner-scoped target and reconciles detail, catalog, and attention caches. */
export function useDeleteSession(
  options: { onDeleteSuccess?: (session: SessionActionTarget) => void | Promise<void> } = {}
) {
  const queryClient = useQueryClient();

  // Capture ownership with the target, not the active profile/workspace at confirmation time.
  return useMutation({
    mutationKey: sessionKeys.deletions(),
    mutationFn: ({ processes, descendants, computerId, ...session }: DeleteSessionParams) => {
      const workspaceId = sessionOwnerWorkspace(session.workspace_id);
      const remote = isRemoteComputerId(computerId);
      if (descendants) {
        return remote
          ? deleteSessionTree(
              workspaceId,
              session.id,
              session.profile_name,
              undefined,
              processes,
              computerId
            )
          : deleteSessionTree(workspaceId, session.id, session.profile_name, undefined, processes);
      }
      // Omitted keeps the processes, the daemon default, so only an answer is sent.
      if (remote) {
        return deleteSession(
          workspaceId,
          session.id,
          session.profile_name,
          undefined,
          processes,
          computerId
        );
      }
      return processes
        ? deleteSession(workspaceId, session.id, session.profile_name, undefined, processes)
        : deleteSession(workspaceId, session.id, session.profile_name);
    },
    onMutate: async ({ id, workspace_id, computerId }) => {
      const deleteWorkspaceId = sessionOwnerWorkspace(workspace_id);
      const keys = sessionKeys.forComputer(computerId);
      sessionStore.trigger.sessionLiveTailSuspended({ sessionId: id });
      await Promise.all([
        queryClient.cancelQueries({ queryKey: keys.detail(deleteWorkspaceId, id) }),
        queryClient.cancelQueries({ queryKey: keys.byIdRoot(id) }),
      ]);
    },
    onSuccess: async (_data, session) => {
      const successWorkspaceId = sessionOwnerWorkspace(session.workspace_id);
      const keys = sessionKeys.forComputer(session.computerId);
      const targets: SessionActionTarget[] = [
        session,
        ...(session.descendants ?? []).map(descendant =>
          session.computerId ? { ...descendant, computerId: session.computerId } : descendant
        ),
      ];
      await Promise.all(targets.map(target => options.onDeleteSuccess?.(target)));
      for (const { id } of targets) {
        discardComposerDraftAttachments(id);
        sessionStore.trigger.sessionInteractionRemoved({ sessionId: id });
        queryClient.removeQueries({ queryKey: keys.detail(successWorkspaceId, id) });
        queryClient.removeQueries({ queryKey: keys.byIdRoot(id) });
      }
      void invalidateWorkspaceSessionCatalog(queryClient, successWorkspaceId, session.computerId);
      void queryClient.invalidateQueries({ queryKey: keys.workspaceLists("") });
      void queryClient.invalidateQueries({ queryKey: keys.attentionSummary() });
    },
    onSettled: (_data, error, { id, workspace_id, descendants, computerId }) => {
      sessionStore.trigger.sessionLiveTailResumed({ sessionId: id });
      if (error) {
        // A failed tree delete can still have removed some of it.
        for (const target of [{ id }, ...(descendants ?? [])]) {
          void invalidateSessionMutationQueries(
            queryClient,
            sessionOwnerWorkspace(workspace_id),
            target.id,
            computerId
          );
        }
      }
    },
  });
}

export interface ArchiveSessionParams extends SessionActionTarget {
  /** Stop a live session through the daemon's normal stop path before archiving it. */
  stop?: boolean;
  /** Stop or keep the processes the session left running; omitted keeps them. */
  processes?: SessionProcessPolicy;
  /** The children known when asked; present opts into archiving the daemon's current tree. */
  descendants?: readonly SessionPayload[];
}

/** Archives the supplied row without depending on the currently selected workspace. */
export function useArchiveSession(options: { requestOwnerId?: string } = {}) {
  const queryClient = useQueryClient();
  const request = useAbortableMutationRequest();

  return useMutation({
    mutationKey: sessionKeys.archives(),
    meta: { requestOwnerId: options.requestOwnerId },
    mutationFn: ({
      id,
      workspace_id,
      profile_name,
      stop,
      processes,
      descendants,
      computerId,
    }: ArchiveSessionParams) =>
      request(signal => {
        const workspace = sessionOwnerWorkspace(workspace_id);
        const remote = isRemoteComputerId(computerId);
        if (descendants) {
          return remote
            ? archiveSessionTree(
                workspace,
                id,
                profile_name,
                signal,
                stop ?? false,
                processes,
                computerId
              )
            : archiveSessionTree(workspace, id, profile_name, signal, stop ?? false, processes);
        }
        if (remote) {
          return archiveSession(
            workspace,
            id,
            profile_name,
            signal,
            stop ?? false,
            processes,
            computerId
          );
        }
        return processes
          ? archiveSession(workspace, id, profile_name, signal, stop ?? false, processes)
          : archiveSession(workspace, id, profile_name, signal, stop ?? false);
      }),
    onSettled: (_data, _error, { id, workspace_id, descendants, computerId }) => {
      for (const target of [{ id }, ...(descendants ?? [])]) {
        void invalidateSessionMutationQueries(
          queryClient,
          sessionOwnerWorkspace(workspace_id),
          target.id,
          computerId
        );
      }
    },
  });
}

/** Restores the supplied row and refreshes the catalog in its owning workspace. */
export function useUnarchiveSession() {
  const queryClient = useQueryClient();
  const request = useAbortableMutationRequest();

  return useMutation({
    mutationFn: ({ id, workspace_id, profile_name, computerId }: SessionActionTarget) =>
      request(signal =>
        isRemoteComputerId(computerId)
          ? unarchiveSession(sessionOwnerWorkspace(workspace_id), id, profile_name, signal, computerId)
          : unarchiveSession(sessionOwnerWorkspace(workspace_id), id, profile_name, signal)
      ),
    onSettled: (_data, _error, { id, workspace_id, computerId }) => {
      void invalidateSessionMutationQueries(
        queryClient,
        sessionOwnerWorkspace(workspace_id),
        id,
        computerId
      );
    },
  });
}
