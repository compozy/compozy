import type { TerminalInfo } from "@/systems/terminal";
import { DESTINATION_COPY, USER_SCOPE_SELECTOR } from "@/systems/workspace";

/** A place a new terminal can open: a project, one of its managed worktrees, or User (`@user`). */
export interface TerminalLauncherDestination {
  /** Unique row key: the project id, `@user`, or `<project id>/<worktree id>`. */
  value: string;
  /** The scope the terminal opens in. */
  workspaceId: string;
  /** The managed worktree the terminal starts in; `null` for the project root or User. */
  worktreeId: string | null;
  label: string;
  /** A worktree row's project, shown beside its name; `null` elsewhere. */
  projectLabel: string | null;
  /** Why a terminal cannot start here (the project folder is gone); `null` when it can. */
  unavailableReason: string | null;
}

/** The slice of a worktree row the launcher reads. */
export interface TerminalLauncherWorktree {
  id: string;
  workspace_id: string;
  name: string;
  branch: string;
  state: string;
  profile_name: string;
}

/** Shared with the projects overview: a registered project whose folder no longer exists. */
export const TERMINAL_LAUNCHER_FOLDER_MISSING = "Folder not found";

/**
 * Every project followed by its ready worktrees owned by `profile`, then User,
 * in the catalog's order. A terminal binds only to a worktree of its own
 * profile, and a pending, failed, or removed one has no checkout to start in.
 */
export function terminalLauncherDestinations(
  workspaces: readonly { id: string; name: string; root_missing?: boolean }[],
  worktrees: readonly TerminalLauncherWorktree[] = [],
  profile: string | null = null
): TerminalLauncherDestination[] {
  const destinations: TerminalLauncherDestination[] = [];
  for (const workspace of workspaces) {
    const missing = workspace.root_missing === true;
    destinations.push({
      value: workspace.id,
      workspaceId: workspace.id,
      worktreeId: null,
      label: workspace.name,
      projectLabel: null,
      unavailableReason: missing ? TERMINAL_LAUNCHER_FOLDER_MISSING : null,
    });
    if (missing) continue;
    for (const worktree of worktrees) {
      if (worktree.workspace_id !== workspace.id || worktree.state !== "ready") continue;
      if (profile !== null && worktree.profile_name !== profile) continue;
      destinations.push({
        value: `${workspace.id}/${worktree.id}`,
        workspaceId: workspace.id,
        worktreeId: worktree.id,
        label: worktree.name,
        projectLabel: workspace.name,
        unavailableReason: null,
      });
    }
  }
  destinations.push({
    value: USER_SCOPE_SELECTOR,
    workspaceId: USER_SCOPE_SELECTOR,
    worktreeId: null,
    label: DESTINATION_COPY.user,
    projectLabel: null,
    unavailableReason: null,
  });
  return destinations;
}

/** The remembered project when a terminal can start there, else the first that can (User always can). */
export function defaultTerminalLauncherDestination(
  destinations: readonly TerminalLauncherDestination[],
  rememberedWorkspaceId: string | null | undefined
): string {
  const available = destinations.filter(destination => destination.unavailableReason === null);
  const remembered = available.find(destination => destination.value === rememberedWorkspaceId);
  return (remembered ?? available[0])?.value ?? USER_SCOPE_SELECTOR;
}

/** The create body for a destination: a worktree row names its worktree. */
export function terminalLauncherCreateInput(destination: TerminalLauncherDestination) {
  return destination.worktreeId === null ? {} : { worktree: destination.worktreeId };
}

/** Running terminals only: an exited one is history, read from the journal. */
export function runningTerminals(terminals: readonly TerminalInfo[]): TerminalInfo[] {
  return terminals.filter(terminal => terminal.state === "running");
}
