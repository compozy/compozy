import type { TerminalInfo } from "@/systems/terminal";
import { DESTINATION_COPY, USER_SCOPE_SELECTOR } from "@/systems/workspace";

/** A place a new terminal can open: a project, or User (`@user`). */
export interface TerminalLauncherDestination {
  value: string;
  label: string;
}

/** Every project, then User, in the catalog's order. */
export function terminalLauncherDestinations(
  workspaces: readonly { id: string; name: string }[]
): TerminalLauncherDestination[] {
  return [
    ...workspaces.map(workspace => ({ value: workspace.id, label: workspace.name })),
    { value: USER_SCOPE_SELECTOR, label: DESTINATION_COPY.user },
  ];
}

/** The remembered project when it still exists, else the first project, else User. */
export function defaultTerminalLauncherDestination(
  destinations: readonly TerminalLauncherDestination[],
  rememberedWorkspaceId: string | null | undefined
): string {
  const remembered = destinations.find(destination => destination.value === rememberedWorkspaceId);
  return (remembered ?? destinations[0])?.value ?? USER_SCOPE_SELECTOR;
}

/** Running terminals only: an exited one is history, read from the journal. */
export function runningTerminals(terminals: readonly TerminalInfo[]): TerminalInfo[] {
  return terminals.filter(terminal => terminal.state === "running");
}
