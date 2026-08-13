import { useState } from "react";

import { sessionStore } from "../stores/session-store";
import { useSessionCreateActions } from "./use-session-create";
import { useSessions } from "./use-sessions";
import type { SessionPayload } from "../types";

interface UseSessionPromptStagingParams {
  workspaceId: string;
  worktreeId: string;
  enabled?: boolean;
}

export interface SessionPromptStaging {
  /** The session the prompt would land in, when one is bound to this worktree. */
  session: SessionPayload | undefined;
  /** Writes the prompt into that session's composer draft. Nothing is sent. */
  stagePrompt: ((text: string) => void) | undefined;
  /** Opens session create bound to this worktree, for when nothing is bound yet. */
  startSession: (() => void) | undefined;
  staged: boolean;
  reset: () => void;
}

/**
 * The bound session a worktree's agent affordances write into.
 *
 * Staging only ever fills the composer draft: the operator reads the prompt and
 * decides whether to send it, so no git command and no turn is triggered from
 * here. With no session bound there is nothing to write to, so the affordance
 * becomes the honest alternative — start one in this worktree — rather than a
 * button that would silently do nothing.
 */
export function useSessionPromptStaging({
  workspaceId,
  worktreeId,
  enabled = true,
}: UseSessionPromptStagingParams): SessionPromptStaging {
  const [staged, setStaged] = useState(false);
  const { openForWorktree } = useSessionCreateActions();
  const sessions = useSessions(workspaceId || null, {
    enabled: enabled && workspaceId !== "" && worktreeId !== "",
    filters: { worktree: worktreeId },
  });
  const session = sessions.data?.at(0);

  if (sessions.isPending || (sessions.isFetching && sessions.data === undefined)) {
    return {
      session: undefined,
      stagePrompt: undefined,
      startSession: undefined,
      staged: false,
      reset: () => setStaged(false),
    };
  }

  if (!session) {
    return {
      session: undefined,
      stagePrompt: undefined,
      startSession: () => openForWorktree(workspaceId, worktreeId),
      staged: false,
      reset: () => setStaged(false),
    };
  }

  return {
    session,
    stagePrompt: (text: string) => {
      sessionStore.trigger.composerDraftChanged({ sessionId: session.id, text });
      setStaged(true);
    },
    startSession: undefined,
    staged,
    reset: () => setStaged(false),
  };
}
