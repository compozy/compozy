import { useState, type ReactNode } from "react";
import { FolderGit2Icon, FolderIcon, FolderPlusIcon } from "@compozy/ui/icons";

import {
  cn,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
  InputGroupButton,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";

export type SessionEnvironmentChipState = "root" | "worktree" | "new" | "pending" | "failed";

interface SessionEnvironmentChipProps {
  menu?: ReactNode;
  state: SessionEnvironmentChipState;
  label: string;
  /** Present only when the daemon says `/worktree` can run right now. */
  onFork?: () => void;
  /** Verbatim daemon reason for an unavailable fork. Never reworded here. */
  forkUnavailableReason?: string;
  /** Marks illustrative creation states in stories. */
  presentational?: boolean;
}

const ICON = {
  root: FolderIcon,
  worktree: FolderGit2Icon,
  new: FolderPlusIcon,
  pending: FolderPlusIcon,
  failed: FolderPlusIcon,
} as const;

function environmentAccessibleName(
  state: SessionEnvironmentChipState,
  label: string,
  hasMenu: boolean,
  unavailableReason: string | undefined
) {
  const environment = `${state === "worktree" ? "Worktree" : "Project"}: ${label}`;
  let action = "";
  if (hasMenu) action = " — session environment";
  else if (state === "root" || state === "worktree")
    action = " — move to a separate copy (worktree)";
  return `${environment}${action}${unavailableReason ? `. ${unavailableReason}` : ""}`;
}

/** Current execution environment, with optional environment actions. */
export function SessionEnvironmentChip({
  state,
  label,
  menu,
  onFork,
  forkUnavailableReason,
  presentational = false,
}: SessionEnvironmentChipProps) {
  const [menuOpen, setMenuOpen] = useState(false);
  const Glyph = ICON[state];
  const forkAvailable = Boolean(onFork);
  const interactive = Boolean(menu) || forkAvailable;
  const accessibleName = environmentAccessibleName(
    state,
    label,
    Boolean(menu),
    forkUnavailableReason
  );
  const toneClass = cn(
    state === "pending" && "border-dashed text-warning [&_svg]:text-warning",
    state === "failed" && "border-dashed text-danger [&_svg]:text-danger",
    state === "new" && "border-dashed"
  );

  const button = (
    <InputGroupButton
      aria-disabled={interactive ? undefined : true}
      aria-label={accessibleName}
      className={toneClass}
      data-binding={state === "root" ? "root" : "worktree"}
      data-fork={forkAvailable ? "available" : forkUnavailableReason ? "unavailable" : undefined}
      data-locked={menu ? undefined : ""}
      data-presentational={presentational ? "true" : undefined}
      data-slot="session-environment-chip"
      data-state={state}
      onClick={menu ? undefined : onFork}
      size="tool-icon"
      type="button"
    >
      <Glyph aria-hidden="true" />
    </InputGroupButton>
  );
  const triggerWithTooltip = (
    <Tooltip disabled={menuOpen}>
      <TooltipTrigger render={menu ? <DropdownMenuTrigger render={button} /> : button} />
      <TooltipContent>{accessibleName}</TooltipContent>
    </Tooltip>
  );
  return menu ? (
    <DropdownMenu open={menuOpen} onOpenChange={setMenuOpen}>
      {triggerWithTooltip}
      <DropdownMenuContent align="start">{menu}</DropdownMenuContent>
    </DropdownMenu>
  ) : (
    triggerWithTooltip
  );
}
