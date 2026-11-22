import { useMutation, useMutationState, useQueryClient } from "@tanstack/react-query";
import { toast } from "@compozy/ui";

import {
  archiveStoppedSessions,
  unarchiveSession,
  type ArchiveStoppedScope,
  type SessionArchiveStoppedResponse,
} from "../adapters/session-archive-api";
import { sessionKeys } from "../lib/query-keys";
import {
  archiveStoppedOutcome,
  archiveStoppedScopeKey,
  archiveStoppedUndoOutcome,
  archiveStoppedUndoPlan,
} from "../lib/session-archive-stopped";
import { runSessionBatch } from "../lib/session-batch";

const ARCHIVE_STOPPED_KEY = [...sessionKeys.all, "archive-stopped"] as const;

export interface ArchiveStoppedModel {
  /** Archives the scope's stopped threads; ignored while a run for that scope is in flight. */
  archiveStopped: (scope: ArchiveStoppedScope) => void;
  /** A run for the scope is in flight, in this or any other surface of the window. */
  running: (scope: ArchiveStoppedScope) => boolean;
}

/** One run: the profile it archives in, so Undo and the in-flight state stay with that profile. */
interface ArchiveStoppedRun {
  profile: string;
  scope: ArchiveStoppedScope;
}

function errorText(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback;
}

/**
 * "Archive stopped" for one profile (#1924): one daemon pass over a project or every project,
 * a toast with the count and skip summary, and Undo that restores exactly the reported ids.
 * The in-flight state is the mutation cache's, so every menu reading a scope agrees.
 */
export function useArchiveStopped(profile: string): ArchiveStoppedModel {
  const queryClient = useQueryClient();
  const inFlight = useMutationState({
    filters: {
      mutationKey: ARCHIVE_STOPPED_KEY,
      status: "pending",
      predicate: mutation => (mutation.state.variables as ArchiveStoppedRun).profile === profile,
    },
    select: mutation =>
      archiveStoppedScopeKey((mutation.state.variables as ArchiveStoppedRun).scope),
  });
  // A read that starts after the change: a list fetch already in flight may predate it.
  const refreshLists = () =>
    void queryClient.invalidateQueries({ queryKey: sessionKeys.lists() }, { cancelRefetch: true });

  const undo = (report: SessionArchiveStoppedResponse, runProfile: string) => {
    const { ids, threadOf } = archiveStoppedUndoPlan(report);
    const workspaceOf = new Map(
      report.archived.flatMap(thread =>
        thread.restore_ids.map(id => [id, thread.workspace_id] as const)
      )
    );
    // One lane per thread runs its ids in order, root first; threads run side by side.
    void runSessionBatch({
      action: "unarchive",
      ids,
      previous: [],
      coveredBy: threadOf,
      execute: id => unarchiveSession(workspaceOf.get(id)!, id, runProfile),
      onProgress: () => {},
    }).then(results => {
      refreshLists();
      const outcome = archiveStoppedUndoOutcome(report, results);
      if (outcome.ok) toast.success(outcome.title);
      else toast.error(outcome.title, { description: outcome.description });
    });
  };

  const mutation = useMutation({
    mutationKey: ARCHIVE_STOPPED_KEY,
    mutationFn: (run: ArchiveStoppedRun) => archiveStoppedSessions(run.profile, run.scope),
    onSuccess: (report, run) => {
      refreshLists();
      const outcome = archiveStoppedOutcome(report);
      const action = { label: "Undo", onClick: () => undo(report, run.profile) };
      if (outcome.kind === "empty") {
        toast.message("No stopped threads to archive", { description: outcome.description });
      } else if (outcome.kind === "archived") {
        toast.success(outcome.title, { description: outcome.description, action });
      } else {
        toast.error(outcome.title, {
          description: outcome.description,
          ...(outcome.undoable ? { action } : {}),
        });
      }
    },
    onError: error => toast.error(errorText(error, "Couldn't archive stopped threads")),
  });

  const running = (scope: ArchiveStoppedScope) => inFlight.includes(archiveStoppedScopeKey(scope));
  return {
    archiveStopped: scope => {
      if (!running(scope)) mutation.mutate({ profile, scope });
    },
    running,
  };
}
