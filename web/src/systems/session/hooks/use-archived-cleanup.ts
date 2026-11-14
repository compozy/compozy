import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useEffectEvent, useRef, useState } from "react";
import { toast } from "@compozy/ui";

import {
  startSessionHistoryCleanup,
  stopSessionHistoryCleanup,
  type SessionHistoryCleanupRequest,
  type SessionHistoryCleanupRun,
} from "../adapters/session-history-cleanup-api";
import {
  DEFAULT_CLEANUP_OPTIONS,
  EMPTY_CLEANUP_IMPACT,
  cleanupImpactFromPlan,
  cleanupPhase,
  cleanupPreviewable,
  cleanupRequest,
  deletedSentence,
  nothingChecked,
  runCleanupOptions,
  stoppedSentence,
  type CleanupChoiceCounts,
  type CleanupImpact,
  type CleanupOptions,
  type CleanupPhase,
  type CleanupScope,
} from "../lib/archived-cleanup-model";
import { sessionKeys } from "../lib/query-keys";
import {
  sessionHistoryCleanupPreviewOptions,
  sessionHistoryCleanupRunOptions,
} from "../lib/session-history-cleanup-options";

export interface ArchivedCleanupInput {
  /** The one profile the list is bounded by; the aggregate lens has no cleanup. */
  profile: string;
  scope: CleanupScope;
  /** Names a workspace id from the plan or a run; `@user` and `""` are the User scope. */
  projectName: (workspaceId: string) => string;
}

export interface ArchivedCleanupModel {
  open: boolean;
  setOpen: (open: boolean) => void;
  options: CleanupOptions;
  setOptions: (options: CleanupOptions) => void;
  impact?: CleanupImpact;
  counts: CleanupChoiceCounts;
  /** A dry run for the current choices is in flight; the count shown may be the previous one. */
  counting: boolean;
  previewError: string | null;
  phase: CleanupPhase;
  starting: boolean;
  /** The running cleanup's progress, for the footer; `null` when none runs. */
  progress: { done: number; total: number } | null;
  confirm: () => void;
  retry: () => void;
  stop: () => void;
}

export const ALL_PROJECTS = "All projects";

const NO_COUNTS: CleanupChoiceCounts = { everything: 0, children: 0, subagents: 0 };

function errorText(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback;
}

/** A retry repeats the run's own scope, checks, and cutoff, limited to the threads that failed. */
function retryRequest(run: SessionHistoryCleanupRun): SessionHistoryCleanupRequest {
  const request: SessionHistoryCleanupRequest = run.all_workspaces
    ? { all_workspaces: true, delete: run.delete }
    : { workspace_id: run.workspace_id, delete: run.delete };
  if (run.archived_before) request.archived_before = run.archived_before;
  const ids = run.failures.map(failure => failure.session_id);
  if (ids.length > 0) request.session_ids = ids;
  return request;
}

/**
 * "Clean up history" for one profile's Archived list (#1717): the dialog's checks and range, the
 * daemon's dry run as the live count, and the profile's background run. The run outlives the
 * dialog; its progress and end arrive on the catalog stream, and the end is announced here.
 */
export function useArchivedCleanup({
  profile,
  scope,
  projectName,
}: ArchivedCleanupInput): ArchivedCleanupModel {
  const queryClient = useQueryClient();
  const [open, setOpenState] = useState(false);
  const [options, setOptions] = useState<CleanupOptions>(DEFAULT_CLEANUP_OPTIONS);
  // A finished run's outcome shows once; closing it returns the dialog to its form.
  const [dismissedRunId, setDismissedRunId] = useState<string | null>(null);
  const runKey = sessionKeys.historyCleanupRun(profile);
  const run = useQuery(sessionHistoryCleanupRunOptions(profile)).data ?? null;
  const running = run?.state === "running";
  const phase =
    running || run?.id !== dismissedRunId ? cleanupPhase(run) : { kind: "form" as const };
  const shownOptions = running && run ? runCleanupOptions(run) : options;

  // Each check's own count does not depend on the checks, so with none checked the dry run
  // still counts Everything for the range; the preview itself then says to choose something.
  const unchecked = nothingChecked(options.what);
  const previewOptions = unchecked ? { ...options, what: DEFAULT_CLEANUP_OPTIONS.what } : options;
  const preview = useQuery({
    ...sessionHistoryCleanupPreviewOptions(profile, cleanupRequest(previewOptions, scope)),
    enabled: open && phase.kind === "form" && cleanupPreviewable(previewOptions),
  });
  const plan = cleanupPreviewable(previewOptions) ? preview.data : undefined;
  const impact =
    unchecked || !cleanupPreviewable(options)
      ? EMPTY_CLEANUP_IMPACT
      : plan && cleanupImpactFromPlan(plan, projectName);

  const start = useMutation({
    mutationFn: (request: SessionHistoryCleanupRequest) =>
      startSessionHistoryCleanup(profile, request),
    onSuccess: started => queryClient.setQueryData(runKey, started),
    onError: error => {
      toast.error(errorText(error, "Couldn’t start cleaning up history"));
      // A 409 means another window or the CLI started one: show its progress.
      void queryClient.invalidateQueries({ queryKey: runKey });
    },
  });
  const stopRun = useMutation({
    mutationFn: () => stopSessionHistoryCleanup(profile),
    onSuccess: stopped => queryClient.setQueryData(runKey, stopped),
    onError: error => toast.error(errorText(error, "Couldn’t stop cleaning up history")),
  });

  useRunEndAnnouncement(run, projectName, () => {
    setOpenState(false);
    setDismissedRunId(run?.id ?? null);
  });

  const setOpen = (next: boolean) => {
    setOpenState(next);
    if (!next && run && !running) setDismissedRunId(run.id);
  };

  return {
    open,
    setOpen,
    options: shownOptions,
    setOptions,
    impact: running ? undefined : impact,
    counts: plan?.choices ?? NO_COUNTS,
    counting:
      cleanupPreviewable(previewOptions) && (preview.isFetching || preview.isPlaceholderData),
    previewError: preview.error ? errorText(preview.error, "The count failed.") : null,
    phase,
    starting: start.isPending,
    progress: running && run ? { done: run.done, total: run.total } : null,
    confirm: () => start.mutate(cleanupRequest(options, scope)),
    retry: () => {
      if (run) start.mutate(retryRequest(run));
    },
    stop: () => stopRun.mutate(),
  };
}

/**
 * Announces how a run this window watched ended: completed and stopped runs close the dialog with
 * a toast; a run that kept sessions says so, and the dialog shows which on its next open.
 */
function useRunEndAnnouncement(
  run: SessionHistoryCleanupRun | null,
  projectName: (workspaceId: string) => string,
  onSettled: () => void
) {
  const watched = useRef<string | null>(null);
  const announce = useEffectEvent((ended: SessionHistoryCleanupRun) => {
    switch (ended.state) {
      case "completed":
        // The run's own scope: the list may show another project by the time it ends.
        toast.success(
          deletedSentence(
            ended.deleted,
            ended.all_workspaces ? ALL_PROJECTS : projectName(ended.workspace_id ?? "")
          )
        );
        onSettled();
        return;
      case "stopped":
        toast(stoppedSentence(ended));
        onSettled();
        return;
      default:
        toast.error("Some sessions weren’t deleted", {
          description: "Open Clean up history to see which, and try them again.",
        });
    }
  });
  useEffect(() => {
    if (!run) return;
    if (run.state === "running") {
      watched.current = run.id;
      return;
    }
    if (watched.current !== run.id) return;
    watched.current = null;
    announce(run);
  }, [run]);
}
