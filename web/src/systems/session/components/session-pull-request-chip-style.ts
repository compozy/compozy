import {
  GitMergeIcon,
  GitPullRequestClosedIcon,
  GitPullRequestDraftIcon,
  GitPullRequestIcon,
  type LucideIcon,
} from "@compozy/ui/icons";

import type { PullRequestShape } from "../lib/pull-request-signal";

export const PULL_REQUEST_SHAPE_ICON: Record<PullRequestShape, LucideIcon> = {
  open: GitPullRequestIcon,
  draft: GitPullRequestDraftIcon,
  merged: GitMergeIcon,
  closed: GitPullRequestClosedIcon,
};

/** Worktree-chip geometry (`SessionWorktreeBindingChip`): 24 px surface-2 pill, badge type. */
export const SESSION_CHIP_CLASS =
  "inline-flex h-6 shrink-0 items-center gap-1.5 whitespace-nowrap rounded-pill bg-surface-2 py-0 pr-1.5 pl-2.5 text-badge font-medium text-muted-foreground outline-none transition-colors duration-fast ease-out hover:bg-selected hover:text-fg focus-visible:shadow-focus-ring data-popup-open:bg-selected data-popup-open:text-fg [&_svg]:size-3 [&_svg]:shrink-0";
