import { useState } from "react";
import { toast } from "@compozy/ui";

import { createClientId } from "@/lib/client-id";
import {
  useSendSessionPrompt,
  useSessionPullRequestRefresh,
  type SessionBusyInputHandler,
  type SessionPayload,
} from "@/systems/session";
import { useWorktreeDetailContext, type WorktreeDetailModel } from "@/systems/workspace";

/**
 * Feeds the topbar PR chip (`_uiux.md` S2) from the window's own reads: the
 * bound managed worktree's exit plan through the existing exit flow, and the
 * agent prompts as normal user prompts: queued behind a running turn through the
 * busy-input gate, otherwise sent like the composer's idle prompt, which also
 * resumes a stopped session.
 */
export function useSessionWindowPullRequest(input: {
  session: SessionPayload;
  workspaceId: string;
  worktreeId: string | undefined;
  /** The binding resolves to a managed worktree record. */
  bound: boolean;
  liveDataEnabled: boolean;
  /** A turn is in flight, so a new prompt queues behind it. */
  running: boolean;
  queuePrompt: SessionBusyInputHandler;
}) {
  const { session, workspaceId, worktreeId, bound, liveDataEnabled, running, queuePrompt } = input;
  const sendIdlePrompt = useSendSessionPrompt({ workspaceId });
  const pullRequestRefresh = useSessionPullRequestRefresh(session);
  const exit = useWorktreeDetailContext({
    workspaceId,
    worktreeId: worktreeId ?? "",
    enabled: bound && liveDataEnabled,
    // The chip's exit surface has no inline error slot.
    onPrepareError: message => toast.error(message),
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
          rows: ladder.rows,
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
    const sent = running
      ? queuePrompt({ message: text, attachments: [] })
      : sendIdlePrompt.mutateAsync({
          id: session.id,
          idempotencyKey: createClientId(),
          messageId: createClientId(),
          message: text,
        });
    void Promise.resolve(sent).catch((error: unknown) => {
      toast.error(error instanceof Error ? error.message : "Couldn't send the prompt.");
    });
  };

  return { pullRequestExit, pullRequestRefresh, exitSurface, onSendPrompt };
}
