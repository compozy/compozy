import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useCallback, useState } from "react";

import { toast } from "@compozy/ui";

import { cancelSubagent, deleteSubagent } from "../adapters/subagent-api";
import type { SubagentView } from "../components/subagents/types";
import { sessionKeys } from "../lib/query-keys";
import { invalidateWorkspaceSessionCatalog } from "../lib/session-query-invalidation";
import {
  applySubagentRemoved,
  EMPTY_SUBAGENT_ROSTER,
  type SubagentRoster,
} from "../lib/subagent-roster";

/** The parent row a subagent verb acts on: its own workspace, session, and profile. */
export interface SubagentActionScope {
  workspaceId: string;
  sessionId: string;
  profile: string;
}

/** What one confirmation holds while its DELETE flies. */
interface PendingDelete {
  subagent: SubagentView;
  scope: SubagentActionScope;
}

/** The confirmation one subagent delete shows; `failure` stays on screen with the dialog open. */
export interface SubagentDeleteDialogModel {
  open: boolean;
  subagent: SubagentView | null;
  isDeleting: boolean;
  failure: string | null;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
}

export interface SessionSubagentActionsModel {
  /** The row whose delete confirmation is open, or null. */
  deleteTarget: SubagentView | null;
  openDelete: (subagent: SubagentView, scope: SubagentActionScope) => void;
  /** Cancels a live delegated row; the parent stream settles the roster. */
  stop: (subagent: SubagentView, scope: SubagentActionScope) => void;
  dialog: SubagentDeleteDialogModel;
}

/** A rail row always carries both ids and a profile; an empty scope disables the mutations. */
function hasSubagentScope(scope: SubagentActionScope): boolean {
  return (
    scope.workspaceId.trim() !== "" && scope.sessionId.trim() !== "" && scope.profile.trim() !== ""
  );
}

/**
 * The rail's subagent row verbs: Stop (delegated, live) and Delete (delegated).
 * The caller supplies each row's scope, so this owner lives above the threads
 * and outlives the branch whose row opened it: a delete drops the row from the
 * roster the parent stream owns, then rereads that parent's branch, list and
 * the operator catalogs; a stop leaves the roster to the stream.
 */
export function useSessionSubagentActions(): SessionSubagentActionsModel {
  const queryClient = useQueryClient();
  const [pending, setPending] = useState<PendingDelete | null>(null);
  const [failure, setFailure] = useState<string | null>(null);

  const deletion = useMutation({
    mutationFn: ({ subagent, scope }: PendingDelete) =>
      deleteSubagent(scope.workspaceId, subagent.id, scope.profile),
    onSuccess: (_data, { subagent, scope }) => {
      const { workspaceId, sessionId } = scope;
      queryClient.setQueryData<SubagentRoster>(
        sessionKeys.subagentRoster(workspaceId, sessionId),
        existing => applySubagentRemoved(existing ?? EMPTY_SUBAGENT_ROSTER, subagent.id)
      );
      void queryClient.invalidateQueries({
        queryKey: sessionKeys.subagentBranch(workspaceId, sessionId),
      });
      void queryClient.invalidateQueries({
        queryKey: sessionKeys.subagents(workspaceId, sessionId),
      });
      void queryClient.invalidateQueries({
        queryKey: sessionKeys.subagentRoster(workspaceId, sessionId),
      });
      void invalidateWorkspaceSessionCatalog(queryClient, workspaceId);
      void queryClient.invalidateQueries({ queryKey: sessionKeys.attentionSummary() });
      setPending(null);
      setFailure(null);
    },
    onError: error => {
      console.error("Failed to delete subagent", error);
      setFailure(error instanceof Error && error.message ? error.message : null);
      toast.error("Could not delete subagent");
    },
  });

  const openDelete = useCallback((subagent: SubagentView, scope: SubagentActionScope) => {
    setFailure(null);
    setPending({ subagent, scope });
  }, []);

  const onOpenChange = useCallback(
    (open: boolean) => {
      if (open) return;
      // The dialog owns the wait: a dismissal during the request would hide its outcome.
      if (deletion.isPending) return;
      setPending(null);
      setFailure(null);
    },
    [deletion.isPending]
  );

  const onConfirm = useCallback(() => {
    if (!pending || !hasSubagentScope(pending.scope) || deletion.isPending) return;
    deletion.mutate(pending);
  }, [deletion, pending]);

  const stop = useCallback(
    (subagent: SubagentView, scope: SubagentActionScope) => {
      if (!hasSubagentScope(scope)) return;
      const { workspaceId, sessionId, profile } = scope;
      void cancelSubagent(workspaceId, subagent.id, profile)
        .then(() =>
          Promise.all([
            queryClient.invalidateQueries({
              queryKey: sessionKeys.subagentBranch(workspaceId, sessionId),
            }),
            queryClient.invalidateQueries({
              queryKey: sessionKeys.subagents(workspaceId, sessionId),
            }),
          ])
        )
        .catch(error => {
          console.error("Failed to stop subagent", error);
          toast.error("Could not stop subagent");
        });
    },
    [queryClient]
  );

  return {
    deleteTarget: pending?.subagent ?? null,
    openDelete,
    stop,
    dialog: {
      open: pending !== null,
      subagent: pending?.subagent ?? null,
      isDeleting: deletion.isPending,
      failure,
      onOpenChange,
      onConfirm,
    },
  };
}
