import { createContext } from "react";

import type { WorktreeDialogTargets } from "../hooks/use-worktree-dialog-targets";

/**
 * `requestRemove` joins the surface because the palette's action panel offers
 * removing a worktree, and removal has one owner: the shipped confirm dialog.
 * A second confirmation written into the palette would be a second answer to the
 * same question.
 */
export type WorktreeDialogActions = Pick<
  WorktreeDialogTargets,
  "requestContextWorktree" | "requestResolveMissingWorktree" | "requestRemove"
>;

export const WorktreeDialogActionsContext = createContext<WorktreeDialogActions | null>(null);
