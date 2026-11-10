import { useRef } from "react";
import { toast } from "@compozy/ui";
import { runSessionBatch } from "../lib/session-batch";
import { batchSkipNote, batchVerbPast, reportBatch } from "../lib/session-batch-feedback";
import { sessionBatchAction } from "../lib/session-menu-actions";
import type { SessionPayload } from "../types";
import { useMarkSessionSeen } from "./use-mark-session-seen";

type MenuBatchAction = "mark-seen" | "make-top-level";

/** The new selection verbs use the same mutations as rows, with one named result per failure. */
export function useSessionBatchMenuActions(
  busy: boolean,
  detach: (session: SessionPayload) => Promise<unknown>,
  setPendingAction: (action: MenuBatchAction | null) => void
) {
  const markSeen = useMarkSessionSeen();
  const running = useRef(false);
  const run = async (action: MenuBatchAction, sessions: readonly SessionPayload[]) => {
    if (busy || running.current) return;
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
    running.current = true;
    setPendingAction(action);
    try {
      const byId = new Map(eligible.map(session => [session.id, session]));
      const results = await runSessionBatch({
        action,
        ids: eligible.map(session => session.id),
        sessions: eligible,
        previous: [],
        onProgress: () => {},
        execute: id =>
          action === "mark-seen" ? markSeen.mutateAsync(byId.get(id)!) : detach(byId.get(id)!),
      });
      reportBatch(action, results, sessions, skipped);
    } finally {
      running.current = false;
      setPendingAction(null);
    }
  };
  return {
    onMarkSeenMany: (sessions: readonly SessionPayload[]) => void run("mark-seen", sessions),
    onMakeTopLevelMany: (sessions: readonly SessionPayload[]) =>
      void run("make-top-level", sessions),
  };
}
