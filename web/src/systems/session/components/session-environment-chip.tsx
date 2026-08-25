import { FolderGit2Icon, FolderIcon, FolderPlusIcon } from "lucide-react";

import { Button, cn, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";

const ACTION_CLASS = "text-muted hover:bg-row-hover hover:text-fg-strong";

export type SessionEnvironmentChipState = "root" | "worktree" | "new" | "pending" | "failed";

interface SessionEnvironmentChipProps {
  state: SessionEnvironmentChipState;
  label: string;
  /** Present only when the daemon says `/worktree` can run right now. */
  onFork?: () => void;
  /** Verbatim daemon reason for an unavailable fork. Never reworded here. */
  forkUnavailableReason?: string;
  /**
   * Marks a state that has no live wiring yet. Set only by stories: the composer
   * cannot reach these states because a session's worktree binding is immutable.
   */
  presentational?: boolean;
}

const ICON = {
  root: FolderIcon,
  worktree: FolderGit2Icon,
  new: FolderPlusIcon,
  pending: FolderPlusIcon,
  failed: FolderPlusIcon,
} as const;

/**
 * States where a session runs, in the composer control row.
 *
 * A persisted session's binding cannot change, so this is a statement of fact
 * rather than a picker: it never opens an environment popover. The only
 * interactive variant is the one that leads to a fork, and it appears only when
 * the daemon reports `/worktree` as available.
 */
export function SessionEnvironmentChip({
  state,
  label,
  onFork,
  forkUnavailableReason,
  presentational = false,
}: SessionEnvironmentChipProps) {
  const Glyph = ICON[state];
  const forkAvailable = Boolean(onFork);
  const environmentName = `${state === "worktree" ? "Worktree" : "Workspace"}: ${label}`;
  const isLiveState = state === "root" || state === "worktree";
  const tooltip = isLiveState ? `${environmentName} — fork into a new worktree` : environmentName;
  const accessibleName = forkUnavailableReason ? `${tooltip}. ${forkUnavailableReason}` : tooltip;
  const toneClass = cn(
    state === "pending" && "border-dashed text-warning [&_svg]:text-warning",
    state === "failed" && "border-dashed text-danger [&_svg]:text-danger",
    state === "new" && "border-dashed"
  );

  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            aria-disabled={forkAvailable ? undefined : true}
            aria-label={accessibleName}
            className={cn("size-6", ACTION_CLASS, toneClass)}
            data-binding={state === "root" ? "root" : "worktree"}
            data-fork={
              forkAvailable ? "available" : forkUnavailableReason ? "unavailable" : undefined
            }
            data-locked=""
            data-presentational={presentational ? "true" : undefined}
            data-slot="session-environment-chip"
            data-state={state}
            onClick={forkAvailable ? onFork : undefined}
            size="icon-xs"
            type="button"
            variant="ghost"
          >
            <Glyph aria-hidden="true" className="size-3.5" />
          </Button>
        }
      />
      <TooltipContent>{accessibleName}</TooltipContent>
    </Tooltip>
  );
}
