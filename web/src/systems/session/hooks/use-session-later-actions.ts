import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "@compozy/ui";

import {
  moveSessionToLater,
  resumeSessionFromLater,
  type MoveSessionToLaterRequest,
} from "../adapters/session-later-api";
import { batchSkipNote, batchVerbPast, reportBatch } from "../lib/session-batch-feedback";
import { batchSkipReason, type BatchVerb } from "../lib/session-batch-eligibility";
import {
  runningSessionIds,
  runSessionBatch,
  sessionBatchFailureMessage,
} from "../lib/session-batch";
import type { SessionBatchAdmission } from "./use-session-batch-admission";
import { invalidateSessionMutationQueries } from "../lib/session-query-invalidation";
import type { SessionPayload } from "../types";
import { requireWorkspace } from "./session-action-workspace";
import { useAbortableMutationRequest } from "./use-abortable-mutation-request";
import type { SessionActionTarget } from "./use-session-actions";

type LaterVerb = Extract<BatchVerb, "later" | "back-from-later">;

/** A captured session without a project belongs to User, unlike an absent UI selection. */
function ownerWorkspace(workspaceId: string | null | undefined): string {
  return requireWorkspace(workspaceId ?? "");
}

/** The Move to Later dialog a host renders with `SessionLaterDialog`. */
export interface SessionLaterConfirmation {
  open: boolean;
  /** The sessions the dialog moves; more than one is a bulk move. */
  sessions: readonly SessionPayload[];
  isMoving: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: (request: MoveSessionToLaterRequest) => void;
}

function reportError(action: LaterVerb, error: unknown): void {
  toast.error(
    error instanceof Error && error.message ? error.message : sessionBatchFailureMessage(action)
  );
}

/** Splits a selection into the sessions a verb applies to and the skipped ones by reason. */
function eligibleFor(action: LaterVerb, sessions: readonly SessionPayload[]) {
  const eligible: SessionPayload[] = [];
  const skipped = new Map<string, number>();
  for (const session of sessions) {
    const reason = batchSkipReason(action, session);
    if (reason === null) eligible.push(session);
    else skipped.set(reason, (skipped.get(reason) ?? 0) + 1);
  }
  if (eligible.length === 0) {
    const note = batchSkipNote(skipped);
    if (note) toast.info(`No sessions ${batchVerbPast(action)}`, { description: note });
  }
  return { eligible, skipped };
}

/** Parks the supplied row in Later; the daemon stops its whole tree first. */
export function useMoveSessionToLater() {
  const queryClient = useQueryClient();
  const request = useAbortableMutationRequest();
  return useMutation({
    mutationFn: ({
      id,
      workspace_id,
      profile_name,
      body,
    }: SessionActionTarget & { body: MoveSessionToLaterRequest }) =>
      request(signal =>
        moveSessionToLater(ownerWorkspace(workspace_id), id, profile_name, body, signal)
      ),
    onSettled: (_data, _error, { id, workspace_id }) => {
      void invalidateSessionMutationQueries(queryClient, ownerWorkspace(workspace_id), id);
    },
  });
}

/** Brings the supplied row back from Later and refreshes the catalogs that list it. */
export function useResumeSessionFromLater() {
  const queryClient = useQueryClient();
  const request = useAbortableMutationRequest();
  return useMutation({
    mutationFn: ({ id, workspace_id, profile_name }: SessionActionTarget) =>
      request(signal =>
        resumeSessionFromLater(ownerWorkspace(workspace_id), id, profile_name, {}, signal)
      ),
    onSettled: (_data, _error, { id, workspace_id }) => {
      void invalidateSessionMutationQueries(queryClient, ownerWorkspace(workspace_id), id);
    },
  });
}

/**
 * Move to Later and Back from Later for a session list host, single and bulk.
 * Move to Later always asks first (an optional reason and return date); Back
 * from Later runs at once. Batches run with bounded concurrency through `runSessionBatch`
 * and report once, as archive does. `busy` is the host's other lifecycle work:
 * while it runs, these verbs decline, and the host folds `pendingAction` back
 * into its own lock. `batchRunning` is the host's one synchronous batch admission,
 * so a Later batch and any sibling batch family cannot both start in one turn.
 */
export function useSessionLaterActions({
  busy,
  batchRunning,
  onMovedToLater,
}: {
  busy: boolean;
  batchRunning: SessionBatchAdmission;
  /** Each session this host moved to Later, single or batched, once the daemon confirmed it. */
  onMovedToLater?: (session: SessionPayload) => void;
}) {
  const move = useMoveSessionToLater();
  const back = useResumeSessionFromLater();
  const [target, setTarget] = useState<{
    sessions: readonly SessionPayload[];
    skipped: ReadonlyMap<string, number>;
  } | null>(null);
  const [batch, setBatch] = useState<{ action: LaterVerb; runningIds: readonly string[] } | null>(
    null
  );

  const pendingMutation = move.isPending ? "later" : back.isPending ? "back-from-later" : null;
  const pendingAction: LaterVerb | null = batch?.action ?? pendingMutation;
  const pendingMutationId = move.isPending ? move.variables?.id : back.variables?.id;
  const pendingSessionIds: readonly string[] =
    batch !== null
      ? batch.runningIds
      : pendingMutation !== null && pendingMutationId
        ? [pendingMutationId]
        : [];
  const locked = () => busy || pendingAction !== null || batchRunning.held();

  const runBatch = (
    action: LaterVerb,
    sessions: readonly SessionPayload[],
    skipped: ReadonlyMap<string, number>,
    execute: (session: SessionPayload) => Promise<unknown>
  ) => {
    if (batchRunning.held()) return Promise.resolve(null);
    batchRunning.hold();
    setBatch({ action, runningIds: [] });
    const byId = new Map(sessions.map(session => [session.id, session]));
    return runSessionBatch({
      action,
      ids: sessions.map(session => session.id),
      previous: [],
      sessions,
      execute: id => execute(byId.get(id)!),
      onProgress: results => setBatch({ action, runningIds: runningSessionIds(results) }),
    })
      .then(results => {
        reportBatch(action, results, sessions, skipped);
        return results;
      })
      .finally(() => {
        batchRunning.release();
        setBatch(null);
      });
  };

  const confirmMove = (body: MoveSessionToLaterRequest) => {
    if (!target || move.isPending || batchRunning.held()) return;
    const { sessions, skipped } = target;
    if (sessions.length === 1 && skipped.size === 0) {
      const session = sessions[0]!;
      move.mutate(
        { ...session, body },
        {
          onError: error => reportError("later", error),
          onSuccess: () => {
            setTarget(null);
            onMovedToLater?.(session);
          },
        }
      );
      return;
    }
    void runBatch("later", sessions, skipped, async session => {
      await move.mutateAsync({ ...session, body });
      onMovedToLater?.(session);
    }).then(results => {
      // A partial failure keeps the dialog open on the sessions still to move,
      // with the same reason and date, so a retry does not retype the form.
      const failed = sessions.filter(session =>
        results?.some(result => result.id === session.id && result.status === "failed")
      );
      setTarget(failed.length > 0 ? { sessions: failed, skipped: new Map() } : null);
    });
  };

  return {
    pendingAction,
    pendingSessionIds,
    handlers: {
      onMoveToLater: (session: SessionPayload) => {
        if (!locked()) setTarget({ sessions: [session], skipped: new Map() });
      },
      onMoveToLaterMany: (sessions: readonly SessionPayload[]) => {
        if (locked()) return;
        const { eligible, skipped } = eligibleFor("later", sessions);
        if (eligible.length > 0) setTarget({ sessions: eligible, skipped });
      },
      onBackFromLater: (session: SessionPayload) => {
        if (!locked())
          back.mutate(session, { onError: error => reportError("back-from-later", error) });
      },
      onBackFromLaterMany: (sessions: readonly SessionPayload[]) => {
        if (locked()) return;
        const { eligible, skipped } = eligibleFor("back-from-later", sessions);
        if (eligible.length > 0) {
          void runBatch("back-from-later", eligible, skipped, session => back.mutateAsync(session));
        }
      },
    },
    laterDialog: {
      open: target !== null,
      sessions: target?.sessions ?? [],
      isMoving: move.isPending || batch?.action === "later",
      onOpenChange: open => {
        if (!open && !move.isPending && !batchRunning.held()) setTarget(null);
      },
      onConfirm: confirmMove,
    } satisfies SessionLaterConfirmation,
  };
}
