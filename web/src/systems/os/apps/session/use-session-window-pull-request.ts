import { useState } from "react";
import { toast } from "sonner";

import {
  useSessionPullRequestRefresh,
  type SessionBusyInputHandler,
  type SessionPayload,
} from "@/systems/session";
import { useWorktreeDetailContext, type WorktreeDetailModel } from "@/systems/workspace";

/**
 * Feeds the topbar PR chip (`_uiux.md` S2) from the window's own reads: the
 * bound managed worktree's exit plan through the existing exit flow, and the
 * agent prompts through the normal busy-aware send, which queues while a turn
 * runs and resumes a stopped session.
 */
export function useSessionWindowPullRequest(input: {
  session: SessionPayload;
  workspaceId: string;
  worktreeId: string | undefined;
  /** The binding resolves to a managed worktree record. */
  bound: boolean;
  liveDataEnabled: boolean;
  sendPrompt: SessionBusyInputHandler;
}) {
  const { session, workspaceId, worktreeId, bound, liveDataEnabled, sendPrompt } = input;
  const pullRequestRefresh = useSessionPullRequestRefresh(session);
  const exit = useWorktreeDetailContext({
    workspaceId,
    worktreeId: worktreeId ?? "",
    enabled: bound && liveDataEnabled,
  });
  // The context dialog runs its own copy of this flow; only an action started
  // from the chip shows its dialogs and progress here, so one action never
  // reports twice.
  const [startedHere, setStartedHere] = useState(false);
  const ladder = exit.ladder;

  const pullRequestExit = bound
    ? {
        plan: ladder && {
          primary: ladder.primary,
          blocked: ladder.blocked,
          blockedReason: ladder.blockedReason,
        },
        running: exit.isRunning,
        onAction: (action: string) => {
          const row = ladder?.rows.find(candidate => candidate.action === action);
          if (!row) return;
          setStartedHere(true);
          exit.onAction(row);
        },
      }
    : undefined;

  const exitSurface: WorktreeDetailModel | null =
    bound && startedHere
      ? {
          ...exit,
          onDismissProgress: () => {
            exit.onDismissProgress();
            setStartedHere(false);
          },
        }
      : null;

  const onSendPrompt = (text: string) => {
    void Promise.resolve(sendPrompt({ message: text, attachments: [] })).catch((error: unknown) => {
      toast.error(error instanceof Error ? error.message : "Couldn't send the prompt.");
    });
  };

  return { pullRequestExit, pullRequestRefresh, exitSurface, onSendPrompt };
}
