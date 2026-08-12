import { useState } from "react";

import { isSessionRunning, useSessions } from "@/systems/session";
import {
  decodeWorktreeRefusal,
  selectWorktreeForScope,
  useAdoptWorktree,
  useDismissWorktree,
  useRemoveWorktree,
  useWorktreeCreateDialog,
  WorktreeAdoptDialog,
  WorktreeCreateDialog,
  WorktreeMissingResolutionDialog,
  WorktreeRemoveDialog,
  type DiscoveredWorktreePayload,
  type WorktreePayload,
  type WorktreesResponse,
} from "@/systems/workspace";

import { useOsShell } from "../hooks/use-os-shell";

/**
 * Worktree lifecycle dialog boundaries for the desktop shell.
 *
 * Each mirrors `WorkspaceSetupDialogBoundary`: the domain hook runs in exactly
 * one place and only while its dialog is mounted, so a closed dialog holds no
 * mutation state. They live here rather than in `desktop-shell.tsx` so the shell
 * stays a composition root instead of also owning worktree lifecycle wiring.
 */

export function WorktreeCreateDialogBoundary({
  workspaceId,
  workspaceName,
  listing,
  scopeId,
  onOpenChange,
}: {
  workspaceId: string;
  workspaceName: string;
  listing: WorktreesResponse | undefined;
  scopeId: string;
  onOpenChange: (open: boolean) => void;
}) {
  const model = useWorktreeCreateDialog(workspaceId, listing, {
    generatedName: suggestWorktreeName(listing),
  });
  return (
    <WorktreeCreateDialog
      open
      onOpenChange={onOpenChange}
      workspaceName={workspaceName}
      model={model}
      onSelectHoldingWorktree={worktreeId =>
        selectWorktreeForScope(scopeId, workspaceId, worktreeId)
      }
    />
  );
}

/**
 * Two-step removal. The force escalation is local state so the doorway must be
 * crossed deliberately; the refusal itself always comes from the daemon.
 */
export function WorktreeRemoveDialogBoundary({
  workspaceId,
  worktree,
  onClose,
}: {
  workspaceId: string;
  worktree: WorktreePayload;
  onClose: () => void;
}) {
  const remove = useRemoveWorktree(workspaceId);
  const [forceRequested, setForceRequested] = useState(false);
  const sessions = useSessions(workspaceId, { filters: { worktree: worktree.id } });
  const { coordinator } = useOsShell();
  const boundSessions = sessions.data ?? [];
  const running = boundSessions.filter(isSessionRunning).length;
  const firstSession = boundSessions[0] ?? null;

  return (
    <WorktreeRemoveDialog
      open
      onOpenChange={open => {
        if (!open) onClose();
      }}
      worktree={worktree}
      refusal={decodeWorktreeRefusal(remove.error)}
      forceRequested={forceRequested}
      onRequestForce={() => setForceRequested(true)}
      boundSessions={{ running, idle: boundSessions.length - running }}
      isRemoving={remove.isPending}
      onConfirm={force => remove.mutate({ worktreeID: worktree.id, force }, { onSuccess: onClose })}
      onOpenSession={
        firstSession
          ? () => {
              void coordinator.userOpen({
                app: "session",
                instanceKey: firstSession.id,
                route: {
                  pathname: `/agents/${encodeURIComponent(firstSession.agent_name)}/sessions/${encodeURIComponent(firstSession.id)}`,
                  search: {},
                },
              });
              onClose();
            }
          : undefined
      }
    />
  );
}

/** Missing resolution: dismiss the record, or restore it through adoption. */
export function WorktreeMissingDialogBoundary({
  workspaceId,
  worktree,
  onClose,
}: {
  workspaceId: string;
  worktree: WorktreePayload;
  onClose: () => void;
}) {
  const dismiss = useDismissWorktree(workspaceId);
  const restore = useAdoptWorktree(workspaceId);

  return (
    <WorktreeMissingResolutionDialog
      open
      onOpenChange={open => {
        if (!open) onClose();
      }}
      worktree={worktree}
      outcome={dismiss.isSuccess ? WORKTREE_DISMISS_NOOP_OUTCOME : null}
      refusal={decodeWorktreeRefusal(restore.error)}
      isPending={dismiss.isPending || restore.isPending}
      onDismissRecord={() => dismiss.mutate(worktree.id)}
      // Adoption is the restore path: it revalidates Git identity and returns
      // this same record to ready.
      onRestore={() => restore.mutate({ path: worktree.path }, { onSuccess: onClose })}
    />
  );
}

const WORKTREE_DISMISS_NOOP_OUTCOME =
  "Nothing to clean up — no branch, worktree directory, or instance data found.";

/** Adoption confirm/refusal for a selected discovered row (ADR-002). */
export function WorktreeAdoptDialogBoundary({
  workspaceId,
  discovered,
  scopeId,
  onClose,
}: {
  workspaceId: string;
  discovered: DiscoveredWorktreePayload;
  scopeId: string;
  onClose: () => void;
}) {
  const adopt = useAdoptWorktree(workspaceId);
  return (
    <WorktreeAdoptDialog
      open
      onOpenChange={open => {
        if (!open) onClose();
      }}
      discovered={discovered}
      refusal={decodeWorktreeRefusal(adopt.error)}
      isAdopting={adopt.isPending}
      onConfirm={() =>
        adopt.mutate(
          { path: discovered.path },
          {
            onSuccess: worktree => {
              // Adoption completes the gesture the selection started.
              selectWorktreeForScope(scopeId, workspaceId, worktree.id);
              onClose();
            },
          }
        )
      }
    />
  );
}

/**
 * Suggestion only — it rides the placeholder, so an untouched field still means
 * "let the daemon name it".
 */
function suggestWorktreeName(listing: WorktreesResponse | undefined): string {
  const taken = new Set(listing?.worktrees.map(worktree => worktree.name) ?? []);
  const base = "steady-heron";
  if (!taken.has(base)) return base;
  for (let suffix = 2; ; suffix += 1) {
    const candidate = `${base}-${suffix}`;
    if (!taken.has(candidate)) return candidate;
  }
}
