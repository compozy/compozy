import { use } from "react";

import type { PaletteDispatchOutcome } from "../lib/cmd-palette-dispatch";
import type { PaletteRowAction } from "../lib/cmd-palette-row-actions";
import type { PaletteRegistry, ResolvedPaletteCommand } from "../lib/cmd-palette-types";
import type { OsAppId, OsWindowRoute } from "../lib/os-types";
import { WorktreeDialogActionsContext } from "../contexts/worktree-dialog-actions-context";
import type { OsPaletteDomainRow } from "./use-os-palette-domain-search";
import type { OsPaletteLandingActions } from "./use-os-palette-landing-actions";
import { useOsShell } from "./use-os-shell";

interface UseOsPaletteDispatchActionsOptions {
  activeWorkspaceId: string | null;
  close: () => void;
  dispatch(
    command: ResolvedPaletteCommand,
    query: string,
    navigate?: (app: OsAppId, route: OsWindowRoute | null) => void
  ): Promise<PaletteDispatchOutcome>;
  landing: OsPaletteLandingActions;
  openDomainRow: (row: OsPaletteDomainRow) => void;
  query: string;
  registry: PaletteRegistry;
  setPinned: (command: ResolvedPaletteCommand, pinned: boolean) => void;
}

export function useOsPaletteDispatchActions({
  activeWorkspaceId,
  close,
  dispatch,
  landing,
  openDomainRow,
  query,
  registry,
  setPinned,
}: UseOsPaletteDispatchActionsOptions) {
  const { manager, coordinator } = useOsShell();
  const worktreeDialogs = use(WorktreeDialogActionsContext);
  const runCommand = (command: ResolvedPaletteCommand) => {
    const navigate =
      landing.destinationWindowId !== null && command.action.kind === "navigate"
        ? (app: OsAppId, route: OsWindowRoute | null) =>
            landing.pickDestination({ app, ...(route === null ? {} : { route }) })
        : undefined;
    void dispatch(command, query, navigate).then(outcome => {
      if (command.action.kind === "view") return;
      if (outcome.status === "ran" || outcome.status === "invoked") close();
    });
  };
  const runRowAction = (action: PaletteRowAction) => {
    const intent = action.intent;
    switch (intent.kind) {
      case "run-command": {
        const command = registry.byId.get(intent.commandId);
        if (command !== undefined) runCommand(command);
        return;
      }
      case "pin": {
        const command = registry.byId.get(intent.commandId);
        if (command !== undefined) setPinned(command, intent.pinned);
        return;
      }
      case "open-shortcut-settings":
        close();
        void coordinator.userOpen({
          app: "settings",
          route: { pathname: "/settings/layouts", search: { command: intent.commandId } },
        });
        return;
      case "land-session":
        landing.landSession(intent.session);
        close();
        return;
      case "go-to-tab":
        landing.goToTab(intent.windowId);
        return;
      case "close-tab":
        close();
        void manager.closeWindow(intent.windowId);
        return;
      case "scope-worktree":
        landing.selectWorktree(intent.entry);
        return;
      case "remove-worktree": {
        const workspaceId = intent.entry.workspaceId ?? activeWorkspaceId;
        if (worktreeDialogs === null || workspaceId === null) return;
        close();
        worktreeDialogs.requestRemove(workspaceId, intent.entry);
        return;
      }
      case "open-domain-row":
        openDomainRow(intent.row);
    }
  };
  return { runCommand, runRowAction };
}
