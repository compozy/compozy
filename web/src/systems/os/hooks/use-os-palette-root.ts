import { useState } from "react";

import { useActiveWorkspace } from "@/systems/workspace";

import { usePaletteRegistry } from "./use-palette-registry";
import { useCmdPaletteRankSignals } from "./use-cmd-palette-rank-signals";
import {
  assemblePaletteSections,
  paletteGhostCompletion,
  type PaletteSection,
} from "../lib/cmd-palette-sections";
import type { PaletteRegistry, ResolvedPaletteCommand } from "../lib/cmd-palette-types";
import type { OsAppId, OsWindowRoute } from "../lib/os-types";
import { windowManagerStore } from "../stores/window-manager-store";
import { useAttentionJump } from "./use-attention-jump";
import { useOsShell } from "./use-os-shell";
import {
  useOsPaletteEntities,
  type OsPaletteEntities,
  type OsPaletteSessionResult,
  type OsPaletteWorktreeResult,
} from "./use-os-palette-entities";
import {
  useOsPaletteDomainSearch,
  type OsPaletteDomainRow,
  type OsPaletteDomainSection,
} from "./use-os-palette-domain-search";
import { useWindowPaletteIntent } from "./use-window-manager-store";

export interface OsPaletteRootModel {
  readonly registry: PaletteRegistry;
  readonly sections: readonly PaletteSection[];
  readonly entities: OsPaletteEntities;
  readonly domainSections: readonly OsPaletteDomainSection[];
  readonly query: string;
  readonly ghostTail: string | null;
  setQuery(query: string): void;
  /** Non-null while the palette is picking the surface one new tab becomes (US-036). */
  readonly destinationWindowId: string | null;
  readonly destination: boolean;
  /** True when destination mode has nothing eligible to offer (US-036.EC-2). */
  readonly destinationEmpty: boolean;
  runCommand(command: ResolvedPaletteCommand): void;
  openSession(session: OsPaletteSessionResult): void;
  goToTab(windowId: string): void;
  selectWorktree(entry: OsPaletteWorktreeResult): void;
  openDomainRow(row: OsPaletteDomainRow): void;
}

export interface UseOsPaletteRootOptions {
  readonly open: boolean;
  onOpenChange(open: boolean): void;
  /** Runs a resolved command through the one dispatch seam. */
  dispatch(command: ResolvedPaletteCommand, query: string): void;
}

/**
 * The palette root's view-model.
 *
 * It composes rather than owns: the registry projection supplies every command,
 * the entity hook supplies what the OS currently holds, and the seam supplies
 * execution. What is left here is genuinely root-level — the query, the
 * destination intent, and closing the overlay after an effect lands.
 */
export function useOsPaletteRoot({
  open,
  onOpenChange,
  dispatch,
}: UseOsPaletteRootOptions): OsPaletteRootModel {
  const { manager, coordinator } = useOsShell();
  const registry = usePaletteRegistry();
  const jumpToSession = useAttentionJump();
  const { activeWorkspaceId, runtimeWorkspaceId, scope, workspaces } = useActiveWorkspace();
  const [query, setQuery] = useState("");
  const paletteIntent = useWindowPaletteIntent();
  const destinationWindowId =
    open && paletteIntent?.kind === "destination" ? paletteIntent.windowId : null;
  const destination = destinationWindowId !== null;
  const rankSignals = useCmdPaletteRankSignals(runtimeWorkspaceId, open);
  const entities = useOsPaletteEntities({
    open,
    activeWorkspaceId,
    runtimeWorkspaceId,
    scope,
    destinationWindowId,
    destination,
    query,
    signals: rankSignals.data,
    workspaces,
  });
  const sections = assemblePaletteSections({
    registry,
    query,
    destination,
    signals: rankSignals.data,
  });
  const ghostTail = paletteGhostCompletion(registry, query, destination, rankSignals.data);
  const domainSections = useOsPaletteDomainSearch({
    open: open && !destination,
    query,
    workspaceId: runtimeWorkspaceId,
    scope,
    workspaceNames: new Map(workspaces.map(workspace => [workspace.id, workspace.name])),
    signals: rankSignals.data,
  });

  const close = () => onOpenChange(false);
  const pickDestination = (target: {
    app: OsAppId;
    instanceKey?: string;
    route?: OsWindowRoute;
  }) => {
    if (destinationWindowId === null) return;
    windowManagerStore.trigger.paletteIntentCleared();
    void coordinator
      .userOpen({ ...target, stackTargetWindowId: destinationWindowId })
      .then(openedId => {
        // The empty tab hands its place to the picked surface — the open joined
        // its frame, so closing it leaves the destination behind.
        if (openedId !== null) void manager.closeWindow(destinationWindowId);
      });
  };

  return {
    registry,
    sections,
    entities,
    domainSections,
    query,
    ghostTail,
    setQuery,
    destinationWindowId,
    destination,
    destinationEmpty:
      destination && sections.length === 0 && entities.sessions.length === 0 && query === "",
    runCommand: command => {
      if (!command.available) {
        dispatch(command, query);
        return;
      }
      // A view row goes a level deeper; everything else ends the interaction.
      if (command.action.kind !== "view") close();
      dispatch(command, query);
    },
    openSession: session => {
      close();
      if (destination) {
        pickDestination({
          app: "session",
          instanceKey: session.sessionId,
          route: session.route,
        });
        return;
      }
      // BR-20: one landing implementation for every surface that opens a
      // session — restore, switch workspace first, mark done-seen.
      jumpToSession({
        sessionId: session.sessionId,
        agentName: session.agentName,
        workspaceId: session.workspaceId,
      });
    },
    goToTab: windowId => {
      close();
      void coordinator.userActivateWindow(windowId);
    },
    selectWorktree: entry => {
      close();
      entities.selectWorktree(entry);
    },
    openDomainRow: row => {
      close();
      void coordinator.userOpen({ app: row.app, route: row.route });
    },
  };
}
