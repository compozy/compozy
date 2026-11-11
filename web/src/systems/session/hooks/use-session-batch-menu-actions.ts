import { toast } from "@compozy/ui";
import { runSessionBatch, type SessionBatchResult } from "@/systems/session/lib/session-batch";
import {
  batchSkipNote,
  batchVerbPast,
  reportBatch,
} from "@/systems/session/lib/session-batch-feedback";
import { sessionBatchAction } from "@/systems/session/lib/session-menu-actions";
import type { SessionPayload } from "@/systems/session/types";
import { useMarkSessionSeen } from "@/systems/session/hooks/use-mark-session-seen";
import { useSessionReparent } from "@/systems/session/hooks/use-session-reparent";
import type { SessionBatchAdmission } from "@/systems/session/hooks/use-session-batch-admission";

type MenuBatchAction = "mark-seen" | "make-top-level";

/**
 * The new selection verbs use the same mutations as rows, with one named result per failure.
 * `running` is the host's shared batch admission, held from the first mutation to settlement.
 * Singular nest and Make top-level share the lineage boundary: neither overlaps a batch detach.
 */
export function useSessionBatchMenuActions(
  busy: boolean,
  running: SessionBatchAdmission,
  setPendingAction: (action: MenuBatchAction | null) => void,
  onProgress: (results: readonly SessionBatchResult[]) => void
) {
  const markSeen = useMarkSessionSeen();
  const reparent = useSessionReparent();
  const run = async (action: MenuBatchAction, sessions: readonly SessionPayload[]) => {
    if (busy || reparent.pending || running.held()) return;
    const eligible = sessions.filter(sessionBatchAction(action).eligible);
    const skipped = new Map<string, number>();
    const skippedCount = sessions.length - eligible.length;
    if (skippedCount)
      skipped.set(
        action === "mark-seen"
          ? "attention needs an answer or is already seen"
          : "not a nested user session",
        skippedCount
      );
    if (!eligible.length) {
      toast.info(`No sessions ${batchVerbPast(action)}`, { description: batchSkipNote(skipped) });
      return;
    }
    running.hold();
    setPendingAction(action);
    try {
      const byId = new Map(eligible.map(session => [session.id, session]));
      const results = await runSessionBatch({
        action,
        ids: eligible.map(session => session.id),
        sessions: eligible,
        previous: [],
        onProgress,
        execute: id =>
          action === "mark-seen"
            ? markSeen.mutateAsync(byId.get(id)!)
            : reparent.detachAsync(byId.get(id)!),
      });
      reportBatch(action, results, sessions, skipped);
    } finally {
      running.release();
      setPendingAction(null);
    }
  };
  return {
    onMarkSeenMany: (sessions: readonly SessionPayload[]) => void run("mark-seen", sessions),
    onMakeTopLevelMany: (sessions: readonly SessionPayload[]) =>
      void run("make-top-level", sessions),
    onAdopt: (session: SessionPayload, parent: SessionPayload) => {
      if (!running.held()) reparent.adopt(session, parent);
    },
    onMakeTopLevel: (session: SessionPayload) => {
      if (!running.held()) reparent.detach(session);
    },
  };
}
