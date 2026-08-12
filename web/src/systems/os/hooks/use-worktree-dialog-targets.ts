import { useState } from "react";

import type {
  DiscoveredWorktreePayload,
  WorktreeNestEntry,
  WorktreePayload,
} from "@/systems/workspace";

interface DiscoveredTarget {
  workspaceId: string;
  discovered: DiscoveredWorktreePayload;
}

interface WorktreeTarget {
  workspaceId: string;
  worktree: WorktreePayload;
}

export interface WorktreeDialogTargets {
  adoptTarget: DiscoveredTarget | null;
  removeTarget: WorktreeTarget | null;
  missingTarget: WorktreeTarget | null;
  /**
   * Selecting a discovered row opens the adoption confirm instead of scoping to
   * it — adoption is a consent moment, not a side effect of selection.
   */
  requestAdopt: (workspaceId: string, entry: WorktreeNestEntry) => boolean;
  requestRemove: (workspaceId: string, entry: WorktreeNestEntry) => void;
  requestResolveMissing: (workspaceId: string, entry: WorktreeNestEntry) => void;
  closeAdopt: () => void;
  closeRemove: () => void;
  closeMissing: () => void;
}

/** Which worktree lifecycle dialog the shell currently has open, if any. */
export function useWorktreeDialogTargets(): WorktreeDialogTargets {
  const [adoptTarget, setAdoptTarget] = useState<DiscoveredTarget | null>(null);
  const [removeTarget, setRemoveTarget] = useState<WorktreeTarget | null>(null);
  const [missingTarget, setMissingTarget] = useState<WorktreeTarget | null>(null);

  return {
    adoptTarget,
    removeTarget,
    missingTarget,
    /** Returns true when the selection was consumed by the adoption gesture. */
    requestAdopt: (workspaceId, entry) => {
      if (entry.kind !== "discovered" || !entry.discovered) return false;
      setAdoptTarget({ workspaceId, discovered: entry.discovered });
      return true;
    },
    requestRemove: (workspaceId, entry) => {
      if (entry.worktree) setRemoveTarget({ workspaceId, worktree: entry.worktree });
    },
    requestResolveMissing: (workspaceId, entry) => {
      if (entry.worktree) setMissingTarget({ workspaceId, worktree: entry.worktree });
    },
    closeAdopt: () => setAdoptTarget(null),
    closeRemove: () => setRemoveTarget(null),
    closeMissing: () => setMissingTarget(null),
  };
}
