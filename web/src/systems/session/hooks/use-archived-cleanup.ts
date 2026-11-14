import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createStoreLogic } from "@xstate/store";
import { useSelector, useStore } from "@xstate/store-react";
import { useEffect, useEffectEvent, useRef } from "react";
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

interface CleanupDialogState {
  open: boolean;
  options: CleanupOptions;
  /** A finished run whose outcome was seen: the dialog shows its form again. */
  dismissedRunId: string | null;
}

const cleanupDialogLogic = createStoreLogic({
  context: (): CleanupDialogState => ({
    open: false,
    options: DEFAULT_CLEANUP_OPTIONS,
    dismissedRunId: null,
  }),
  on: {
    opened: context => ({ ...context, open: true }),
    closed: (context, event: { finishedRunId: string | null }) => ({
      ...context,
      open: false,
      dismissedRunId: event.finishedRunId ?? context.dismissedRunId,
    }),
    optionsChanged: (context, event: { options: CleanupOptions }) => ({
      ...context,
      options: event.options,
    }),
    /** A run that ended with nothing left to retry closes the dialog; its end was announced. */
    runSettled: (context, event: { runId: string }) => ({
      ...context,
      open: false,
      dismissedRunId: event.runId,
    }),
  },
});

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
  const store = useStore(cleanupDialogLogic);
  const { open, options, dismissedRunId } = useSelector(store, snapshot => snapshot.context);
  const run = useQuery(sessionHistoryCleanupRunOptions(profile)).data ?? null;
  const active = run?.state === "running" ? run : null;
  const phase = active || run?.id !== dismissedRunId ? cleanupPhase(run) : FORM_PHASE;
  const preview = useCleanupPreview({
    profile,
    scope,
    options,
    enabled: open && phase.kind === "form",
    projectName,
  });
  const { start, stop } = useCleanupRunMutations(profile);

  useRunEndAnnouncement(run, projectName, runId => store.trigger.runSettled({ runId }));

  const setOpen = (next: boolean) => {
    if (next) store.trigger.opened();
    else store.trigger.closed({ finishedRunId: run && !active ? run.id : null });
  };

  return {
    open,
    setOpen,
    options: active ? runCleanupOptions(active) : options,
    setOptions: next => store.trigger.optionsChanged({ options: next }),
    ...preview,
    impact: active ? undefined : preview.impact,
    phase,
    starting: start.isPending,
    progress: active ? { done: active.done, total: active.total } : null,
    confirm: () => start.mutate(cleanupRequest(options, scope)),
    retry: () => {
      if (run) start.mutate(retryRequest(run));
    },
    stop: () => stop.mutate(),
  };
}

const FORM_PHASE: CleanupPhase = { kind: "form" };

/**
 * The live count behind the dialog: the daemon's dry run for the current choices. Each check's
 * own count does not depend on the checks, so with none checked the dry run still counts
 * Everything for the range; the preview itself then says to choose something.
 */
function useCleanupPreview({
  profile,
  scope,
  options,
  enabled,
  projectName,
}: {
  profile: string;
  scope: CleanupScope;
  options: CleanupOptions;
  enabled: boolean;
  projectName: (workspaceId: string) => string;
}): Pick<ArchivedCleanupModel, "impact" | "counts" | "counting" | "previewError"> {
  const unchecked = nothingChecked(options.what);
  const counted = unchecked ? { ...options, what: DEFAULT_CLEANUP_OPTIONS.what } : options;
  const previewable = cleanupPreviewable(counted);
  const preview = useQuery({
    ...sessionHistoryCleanupPreviewOptions(profile, cleanupRequest(counted, scope)),
    enabled: enabled && previewable,
  });
  const plan = previewable ? preview.data : undefined;
  const shown = !unchecked && cleanupPreviewable(options);
  return {
    impact: shown ? plan && cleanupImpactFromPlan(plan, projectName) : EMPTY_CLEANUP_IMPACT,
    counts: plan?.choices ?? NO_COUNTS,
    counting: previewable && (preview.isFetching || preview.isPlaceholderData),
    previewError: preview.error ? errorText(preview.error, "The count failed.") : null,
  };
}

/** Starting and stopping the profile's run; each answer becomes the run read at once. */
function useCleanupRunMutations(profile: string) {
  const queryClient = useQueryClient();
  const runKey = sessionKeys.historyCleanupRun(profile);
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
  const stop = useMutation({
    mutationFn: () => stopSessionHistoryCleanup(profile),
    onSuccess: stopped => queryClient.setQueryData(runKey, stopped),
    onError: error => toast.error(errorText(error, "Couldn’t stop cleaning up history")),
  });
  return { start, stop };
}

/**
 * Announces how a run this window watched ended: a completed run, and a stopped one with nothing to
 * retry, close the dialog with a toast; a run that kept sessions stays on its outcome, so the
 * dialog lists them and offers Try again on its next open.
 */
function useRunEndAnnouncement(
  run: SessionHistoryCleanupRun | null,
  projectName: (workspaceId: string) => string,
  onSettled: (runId: string) => void
) {
  const watched = useRef<string | null>(null);
  const announce = useEffectEvent((ended: SessionHistoryCleanupRun) => {
    if (ended.state === "completed") {
      // The run's own scope: the list may show another project by the time it ends.
      toast.success(
        deletedSentence(
          ended.deleted,
          ended.all_workspaces ? ALL_PROJECTS : projectName(ended.workspace_id ?? "")
        )
      );
    } else if (ended.state === "stopped") {
      toast.message(stoppedSentence(ended));
    } else {
      toast.error("Some sessions weren’t deleted", {
        description: "Open Clean up history to see which, and try them again.",
      });
    }
    // Only an outcome with nothing to list or retry closes the dialog.
    if (cleanupPhase(ended).kind === "form") onSettled(ended.id);
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
