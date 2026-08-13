import { FolderGit2Icon, FolderIcon, FolderPlusIcon, LockIcon } from "lucide-react";

import { Button, cn } from "@compozy/ui";

const CHIP_CLASS =
  "inline-flex h-6 items-center gap-1.5 whitespace-nowrap rounded-md border border-line bg-canvas-tint px-[9px] text-badge font-medium text-muted [&_svg]:size-3 [&_svg]:text-subtle";

export type SessionEnvironmentChipState = "root" | "worktree" | "new" | "pending" | "failed";

interface SessionEnvironmentChipProps {
  state: SessionEnvironmentChipState;
  label: string;
  /** Secondary mono fact — `root` for the workspace binding, a name for a draft target. */
  mono?: string;
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

const LOCK_TITLE = "Environment is fixed for a live session — fork to move";

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
  mono,
  onFork,
  forkUnavailableReason,
  presentational = false,
}: SessionEnvironmentChipProps) {
  const Glyph = ICON[state];
  const toneClass = cn(
    state === "pending" && "border-dashed text-warning [&_svg]:text-warning",
    state === "failed" && "border-dashed text-danger [&_svg]:text-danger",
    state === "new" && "border-dashed"
  );

  if (onFork) {
    return (
      <Button
        aria-label={`Environment ${label} — fork to move`}
        className={cn(CHIP_CLASS, "hover:border-line-strong hover:bg-row-hover hover:text-fg")}
        data-binding="worktree"
        data-fork="available"
        data-locked=""
        data-slot="session-environment-chip"
        data-state={state}
        onClick={onFork}
        size="sm"
        title={LOCK_TITLE}
        type="button"
        variant="outline"
      >
        <Glyph aria-hidden="true" />
        {label}
        <LockIcon aria-hidden="true" />
      </Button>
    );
  }

  return (
    <span
      className={cn(CHIP_CLASS, "cursor-default", toneClass)}
      data-binding={state === "root" ? "root" : "worktree"}
      data-fork={forkUnavailableReason ? "unavailable" : undefined}
      data-locked=""
      data-presentational={presentational ? "true" : undefined}
      data-slot="session-environment-chip"
      data-state={state}
      title={forkUnavailableReason ?? (state === "worktree" ? LOCK_TITLE : undefined)}
    >
      <Glyph aria-hidden="true" />
      <span data-slot="session-environment-chip-label">{label}</span>
      {mono ? (
        <span
          className="font-mono text-micro text-subtle"
          data-slot="session-environment-chip-mono"
        >
          {mono}
        </span>
      ) : null}
      {state === "worktree" ? <LockIcon aria-hidden="true" /> : null}
    </span>
  );
}
