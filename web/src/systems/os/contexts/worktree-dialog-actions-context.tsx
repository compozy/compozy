import { createContext } from "react";

import type { WorktreeDialogTargets } from "../hooks/use-worktree-dialog-targets";

export type WorktreeDialogActions = Pick<
  WorktreeDialogTargets,
  "requestContextWorktree" | "requestResolveMissingWorktree"
>;

export const WorktreeDialogActionsContext = createContext<WorktreeDialogActions | null>(null);
