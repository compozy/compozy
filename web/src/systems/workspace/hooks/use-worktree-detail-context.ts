import { useState } from "react";

import { formatRelativeTime } from "@compozy/ui";

import type { WorktreeCommitDialog } from "../components/worktree-commit-dialog";
import type { WorktreePrDialog } from "../components/worktree-pr-dialog";
import type { WorktreeExitLadder, WorktreeExitLadderRow } from "../lib/worktree-exit-ladder";
import type { WorktreePrActionRow } from "../components/worktree-pr-action-rows";
import type { WorktreeGitStatus, WorktreeForgeStatus, WorktreePayload } from "../types";
import { useSessionPromptStaging } from "@/systems/session";

import {
  useCancelWorktreeExitAction,
  useRunWorktreeExitAction,
  useWorktreeStatus,
} from "./use-worktree-exit";
import { useWorktreeExitLadder } from "./use-worktree-exit-ladder";
import { useWorktrees } from "./use-worktrees";

export interface WorktreeDetailModel {
  worktree: WorktreePayload | undefined;
  /** Display title of the session bound to this worktree, when one is. */
  sessionTitle?: string;
  status: WorktreeGitStatus | undefined;
  forgeStatus: WorktreeForgeStatus | undefined;
  ladder: WorktreeExitLadder | undefined;
  state: "loading" | "ready" | "missing" | "error";
  error?: string;
  retry?: () => void;
  staleLabel?: string;
  isRunning: boolean;
  onAction: (row: WorktreeExitLadderRow) => void;
  onRemove?: () => void;
  onCleanUp?: () => void;
  commitDialog?: React.ComponentProps<typeof WorktreeCommitDialog>;
  prDialog?: React.ComponentProps<typeof WorktreePrDialog>;
  progress: ReturnType<typeof useWorktreeExitLadder>["progress"];
  onCancelProgress?: () => void;
  onDismissProgress: () => void;
  onProgressCta?: () => void;
}

interface UseWorktreeDetailContextParams {
  workspaceId: string;
  worktreeId: string;
  enabled?: boolean;
  onRemove?: () => void;
  onCleanUp?: () => void;
}

const COMMIT_ACTIONS = new Set(["commit", "commit_push"]);

/**
 * Assembles the worktree context from the daemon's own reads.
 *
 * Actions that need input open their dialog first; the rest run straight away.
 * Which is which comes from the plan projection, not from a local guess about
 * what each action means.
 */
export function useWorktreeDetailContext({
  workspaceId,
  worktreeId,
  enabled = true,
  onRemove,
  onCleanUp,
}: UseWorktreeDetailContextParams): WorktreeDetailModel {
  const worktrees = useWorktrees(workspaceId || null, { enabled });
  const status = useWorktreeStatus(workspaceId, worktreeId, { enabled });
  const exit = useWorktreeExitLadder(workspaceId, worktreeId, { enabled });
  const run = useRunWorktreeExitAction(workspaceId, worktreeId);
  const cancel = useCancelWorktreeExitAction(workspaceId, worktreeId);
  const staging = useSessionPromptStaging({ workspaceId, worktreeId, enabled });
  const [dialog, setDialog] = useState<"commit" | "pr" | null>(null);

  function onAction(row: WorktreeExitLadderRow) {
    if (row.opensURL && row.url) {
      window.open(row.url, "_blank", "noreferrer");
      return;
    }
    if (COMMIT_ACTIONS.has(row.action)) return setDialog("commit");
    if (row.action === "open_pr") return setDialog("pr");
    run.mutate({ action: row.action });
  }

  const ladder = exit.ladder;
  const worktree = worktrees.data?.worktrees.find(entry => entry.id === worktreeId);
  const progressRunning = exit.progress?.status === "running";
  const waitingForProgress = Boolean(run.data) && exit.progress === undefined;
  const progressOperationID = progressRunning ? exit.progress?.operationID : undefined;
  const progressCtaRow = ladder?.rows.find(row => row.action === exit.progress?.cta?.action);
  const onProgressCta =
    exit.progress?.cta?.action === "remove"
      ? onCleanUp
      : progressCtaRow
        ? () => onAction(progressCtaRow)
        : undefined;
  const forgeFetchedAt = status.data?.forge?.fetched_at;
  const staleLabel =
    ladder?.cleanup.stale && forgeFetchedAt ? formatRelativeTime(forgeFetchedAt) : undefined;
  const detailError = worktrees.error ?? status.error ?? exit.error;
  const detailLoading = worktrees.isPending || status.isPending || exit.isLoading;
  const state: WorktreeDetailModel["state"] = detailError
    ? "error"
    : detailLoading
      ? "loading"
      : worktree
        ? "ready"
        : "missing";
  const retry = () => {
    void worktrees.refetch();
    void status.refetch();
    exit.refetch();
  };

  return {
    worktree,
    state,
    error: detailError instanceof Error ? detailError.message : undefined,
    retry,
    sessionTitle: staging.session?.name ?? staging.session?.id,
    status: status.data?.status,
    forgeStatus: status.data?.forge ?? undefined,
    ladder,
    staleLabel,
    isRunning: run.isPending || waitingForProgress || progressRunning,
    onAction,
    onRemove,
    onCleanUp,
    progress: exit.progress,
    onCancelProgress:
      progressOperationID && !cancel.isPending
        ? () => cancel.mutate(progressOperationID)
        : undefined,
    onDismissProgress: () => {
      exit.dismissProgress();
      run.reset();
    },
    onProgressCta,
    commitDialog:
      dialog === "commit" && ladder && worktree
        ? {
            open: true,
            onOpenChange: open => {
              setDialog(open ? "commit" : null);
              if (!open) staging.reset();
            },
            worktreeName: worktree.name,
            branch: status.data?.status.branch ?? undefined,
            scope: ladder.commitScope,
            actions: ladder.rows.filter(row => COMMIT_ACTIONS.has(row.action)),
            blockedReason: ladder.rows.find(row => row.action === "commit")?.blockedReason,
            isSubmitting: run.isPending,
            onStagePrompt: staging.stagePrompt,
            onStartSession: staging.startSession,
            promptStaged: staging.staged,
            onCommit: (row, message) => {
              run.mutate({ action: row.action, ...(message.trim() ? { message } : {}) });
              staging.reset();
              setDialog(null);
            },
          }
        : undefined,
    prDialog:
      dialog === "pr" && ladder && worktree
        ? {
            open: true,
            onOpenChange: open => setDialog(open ? "pr" : null),
            worktreeName: worktree.name,
            branch: status.data?.status.branch ?? undefined,
            ladder,
            prefill: exit.plan?.pr_prefill ?? undefined,
            onSubmit: (row: WorktreePrActionRow, fields) => {
              if (row.url) {
                window.open(row.url, "_blank", "noreferrer");
                setDialog(null);
                return;
              }
              run.mutate({
                action: "open_pr",
                draft: row.key === "draft",
                ...(fields.title.trim() ? { title: fields.title } : {}),
                ...(fields.body.trim() ? { body: fields.body } : {}),
                ...(ladder.base ? { base: ladder.base } : {}),
              });
              setDialog(null);
            },
          }
        : undefined,
  };
}
