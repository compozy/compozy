import { use, useEffect, useState } from "react";

import { notifyUser } from "@/lib/user-feedback";
import { consumeChooseSessionTerminalQuote, useSessionPromptFallback } from "@/systems/session";
import { clearChooseSessionTerminalQuote } from "@/systems/terminal/parts";
import { useActiveWorkspace } from "@/systems/workspace";

import { usePaletteRegistry } from "./use-palette-registry";
import { useCmdPaletteRankSignals } from "./use-cmd-palette-rank-signals";
import { useCmdPaletteFallbackSettings } from "./use-cmd-palette-fallback-settings";
import {
  assemblePaletteResults,
  type PaletteAgentFallback,
  paletteGhostCompletion,
  type PaletteSection,
} from "../lib/cmd-palette-sections";
import type { PaletteDispatchOutcome } from "../lib/cmd-palette-dispatch";
import { workspaceSwitchFeedback } from "../lib/cmd-palette-feedback";
import type { PaletteRowAction, PaletteRowSources } from "../lib/cmd-palette-row-actions";
import type { PaletteRegistry, ResolvedPaletteCommand } from "../lib/cmd-palette-types";
import type { OsAppId, OsWindowRoute } from "../lib/os-types";
import { windowManagerStore } from "../stores/window-manager-store";
import { WorktreeDialogActionsContext } from "../contexts/worktree-dialog-actions-context";
import { useAttentionJump } from "./use-attention-jump";
import { useOsPaletteDomainOpen } from "./use-os-palette-domain-open";
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
  readonly fallback: PaletteAgentFallback | null;
  readonly fallbackPending: boolean;
  readonly entities: OsPaletteEntities;
  readonly domainSections: readonly OsPaletteDomainSection[];
  readonly query: string;
  readonly ghostTail: string | null;
  setQuery(query: string): void;
  /** Pinned command ids from the rank-signal snapshot; the panel reads them. */
  readonly pins: readonly string[];
  /** Every row currently on screen, for resolving the selection to its source. */
  readonly rowSources: PaletteRowSources;
  /** Non-null while the palette is picking the surface one new tab becomes (US-036). */
  readonly destinationWindowId: string | null;
  readonly destination: boolean;
  /** True when destination mode has nothing eligible to offer (US-036.EC-2). */
  readonly destinationEmpty: boolean;
  runCommand(command: ResolvedPaletteCommand): void;
  runFallback(query: string): void;
  /** Runs one action-panel intent against this client's coordinators. */
  runRowAction(action: PaletteRowAction): void;
  openSession(session: OsPaletteSessionResult): void;
  goToTab(windowId: string): void;
  selectWorktree(entry: OsPaletteWorktreeResult): void;
  openDomainRow(row: OsPaletteDomainRow): void;
}

export interface UseOsPaletteRootOptions {
  readonly open: boolean;
  onOpenChange(open: boolean): void;
  /**
   * Runs a resolved command through the one dispatch seam. The outcome decides
   * whether the palette closes, so it has to come back.
   */
  dispatch(
    command: ResolvedPaletteCommand,
    query: string,
    navigate?: (app: OsAppId, route: OsWindowRoute | null) => void
  ): Promise<PaletteDispatchOutcome>;
  /** Pins or unpins a command through the seam's daemon route. */
  setPinned(command: ResolvedPaletteCommand, pinned: boolean): void;
}

/**
 * The palette root's view-model.
 *
 * It composes rather than owns: the registry projection supplies every command,
 * the entity hook supplies what the OS currently holds, and the seam supplies
 * execution. What is left here is genuinely root-level — the query, the
 * destination intent, closing the overlay after an effect lands, and running the
 * action-panel intents that reach this client's own coordinators.
 */
export function useOsPaletteRoot({
  open,
  onOpenChange,
  dispatch,
  setPinned,
}: UseOsPaletteRootOptions): OsPaletteRootModel {
  const { manager, coordinator } = useOsShell();
  const registry = usePaletteRegistry();
  const jumpToSession = useAttentionJump();
  const worktreeDialogs = use(WorktreeDialogActionsContext);
  const workspace = useActiveWorkspace();
  const { activeWorkspaceId, registeredWorkspaces, runtimeWorkspaceId, scope } = workspace;
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
    workspaces: registeredWorkspaces,
  });
  const fallbackAgentEnabled = useCmdPaletteFallbackSettings({
    activeWorkspaceId,
    open,
    scope,
    settled: workspace.hasHydrated && !workspace.pending,
  });
  const assembly = assemblePaletteResults({
    registry,
    query,
    destination,
    signals: rankSignals.data,
    fallbackAgentEnabled,
  });
  const sections = assembly.sections;
  const ghostTail = paletteGhostCompletion(registry, query, destination, rankSignals.data);
  const domainSections = useOsPaletteDomainSearch({
    open: open && !destination,
    query,
    workspaceId: runtimeWorkspaceId,
    scope,
    workspaceNames:
      scope === "global"
        ? new Map(registeredWorkspaces.map(workspace => [workspace.id, workspace.name]))
        : new Map(),
    signals: rankSignals.data,
  });

  const close = () => onOpenChange(false);
  // The choose-slot is an external store. Closing the picker without a pick
  // must drop it — consume already took, so this is a no-op on a real land.
  useEffect(() => {
    if (!open) clearChooseSessionTerminalQuote();
  }, [open]);
  const openDomainRow = useOsPaletteDomainOpen(close);
  const fallback =
    assembly.fallback !== null &&
    assembly.sections.length === 0 &&
    (entities.sessions.length > 0 ||
      entities.tabs.length > 0 ||
      entities.worktrees.length > 0 ||
      domainSections.some(section => section.rows.length > 0))
      ? null
      : assembly.fallback;
  const fallbackSession = useSessionPromptFallback({
    onCreated: session => {
      close();
      jumpToSession({
        sessionId: session.id,
        agentName: session.agent_name,
        workspaceId: session.workspace_id,
      });
    },
    onPickerOpened: close,
  });
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

  const landSession = (session: OsPaletteSessionResult) => {
    consumeChooseSessionTerminalQuote(session.sessionId);
    // A landing that moves the shell to another workspace says so; the context
    // changing under the operator is never silent (US-017.EC-3).
    if (session.workspaceId !== "" && session.workspaceId !== runtimeWorkspaceId) {
      const name =
        registeredWorkspaces.find(workspace => workspace.id === session.workspaceId)?.name ??
        session.workspaceLabel ??
        session.workspaceId;
      notifyUser(workspaceSwitchFeedback(name, session.title));
    }
    // BR-20: one landing implementation for every surface that opens a
    // session — restore, switch workspace first, mark done-seen.
    jumpToSession({
      sessionId: session.sessionId,
      agentName: session.agentName,
      workspaceId: session.workspaceId,
    });
  };

  const openSession = (session: OsPaletteSessionResult) => {
    if (destination) {
      consumeChooseSessionTerminalQuote(session.sessionId);
      close();
      pickDestination({ app: "session", instanceKey: session.sessionId, route: session.route });
      return;
    }
    landSession(session);
    close();
  };

  const goToTab = (windowId: string) => {
    close();
    void coordinator.userActivateWindow(windowId);
  };

  const scopeToWorktree = (entry: OsPaletteWorktreeResult) => {
    close();
    entities.selectWorktree(entry);
  };

  /*
   * The palette closes when the command actually did something, and only then.
   * A view push stays (it *is* the next level), an argument or confirmation step
   * stays (it is still asking), a refusal stays (the row and its reason are
   * still what the operator is looking at), and a daemon invocation holds the
   * surface open — showing itself as pending — until it lands (US-017.AC-2).
   */
  const runCommand = (command: ResolvedPaletteCommand) => {
    const navigate =
      destination && command.action.kind === "navigate"
        ? (app: OsAppId, route: OsWindowRoute | null) =>
            pickDestination({ app, ...(route === null ? {} : { route }) })
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
        // The action is offered only while the registry carries the settings
        // destination, so the deep link cannot point at a page this client
        // could not open. It carries the command so the table can land on that
        // row instead of on a registry the operator then has to search
        // (US-022.AC-1).
        void coordinator.userOpen({
          app: "settings",
          route: { pathname: "/settings/layouts", search: { command: intent.commandId } },
        });
        return;
      case "land-session":
        landSession(intent.session);
        close();
        return;
      case "go-to-tab":
        goToTab(intent.windowId);
        return;
      case "close-tab":
        close();
        void manager.closeWindow(intent.windowId);
        return;
      case "scope-worktree":
        scopeToWorktree(intent.entry);
        return;
      case "remove-worktree": {
        // Removal keeps its shipped confirm dialog; the palette raises it and
        // steps out of the way.
        const workspaceId = intent.entry.workspaceId ?? activeWorkspaceId;
        if (worktreeDialogs === null || workspaceId === null) return;
        close();
        worktreeDialogs.requestRemove(workspaceId, intent.entry);
        return;
      }
      case "open-domain-row":
        openDomainRow(intent.row);
        return;
    }
  };

  return {
    registry,
    sections,
    fallback,
    fallbackPending: fallbackSession.pending,
    entities,
    domainSections,
    query,
    ghostTail,
    setQuery,
    pins: rankSignals.data?.pins ?? [],
    rowSources: {
      commands: sections.flatMap(section => section.commands),
      sessions: entities.sessions,
      tabs: destination ? [] : entities.tabs,
      worktrees: destination ? [] : entities.worktrees,
      domainRows: domainSections.flatMap(section => section.rows),
    },
    destinationWindowId,
    destination,
    destinationEmpty:
      destination && sections.length === 0 && entities.sessions.length === 0 && query === "",
    runCommand,
    runFallback: query => void fallbackSession.run(query),
    runRowAction,
    openSession,
    goToTab,
    selectWorktree: scopeToWorktree,
    openDomainRow,
  };
}
