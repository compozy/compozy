import type { TerminalInfo } from "@/systems/terminal";
import { DESTINATION_COPY, USER_SCOPE_SELECTOR } from "@/systems/workspace";

/** A place a new terminal can open: a project, or User (`@user`). */
export interface TerminalLauncherDestination {
  value: string;
  label: string;
  /** Why a terminal cannot start here (the project folder is gone); `null` when it can. */
  unavailableReason: string | null;
}

/** Shared with the projects overview: a registered project whose folder no longer exists. */
export const TERMINAL_LAUNCHER_FOLDER_MISSING = "Folder not found";

/** Every project, then User, in the catalog's order. */
export function terminalLauncherDestinations(
  workspaces: readonly { id: string; name: string; root_missing?: boolean }[]
): TerminalLauncherDestination[] {
  return [
    ...workspaces.map(workspace => ({
      value: workspace.id,
      label: workspace.name,
      unavailableReason: workspace.root_missing ? TERMINAL_LAUNCHER_FOLDER_MISSING : null,
    })),
    { value: USER_SCOPE_SELECTOR, label: DESTINATION_COPY.user, unavailableReason: null },
  ];
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

/** Running terminals only: an exited one is history, read from the journal. */
export function runningTerminals(terminals: readonly TerminalInfo[]): TerminalInfo[] {
  return terminals.filter(terminal => terminal.state === "running");
}
