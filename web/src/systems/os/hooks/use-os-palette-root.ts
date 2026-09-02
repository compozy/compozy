import { useEffect, useState } from "react";

import { useSessionPromptFallback } from "@/systems/session";
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
import type { PaletteRowAction, PaletteRowSources } from "../lib/cmd-palette-row-actions";
import type { PaletteRegistry, ResolvedPaletteCommand } from "../lib/cmd-palette-types";
import type { OsAppId, OsWindowRoute } from "../lib/os-types";
import { useAttentionJump } from "./use-attention-jump";
import { useOsPaletteDomainOpen } from "./use-os-palette-domain-open";
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
import { useOsPaletteLandingActions } from "./use-os-palette-landing-actions";
import { useOsPaletteDispatchActions } from "./use-os-palette-dispatch-actions";

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

function visiblePaletteFallback(
  fallback: PaletteAgentFallback | null,
  sectionCount: number,
  entities: OsPaletteEntities,
  domainSections: readonly OsPaletteDomainSection[]
): PaletteAgentFallback | null {
  const hasDirectResult =
    sectionCount > 0 ||
    entities.sessions.length > 0 ||
    entities.tabs.length > 0 ||
    entities.worktrees.length > 0 ||
    domainSections.some(section => section.rows.length > 0);
  return fallback !== null && hasDirectResult ? null : fallback;
}

function paletteRowSources({
  destination,
  domainSections,
  entities,
  sections,
}: {
  destination: boolean;
  domainSections: readonly OsPaletteDomainSection[];
  entities: OsPaletteEntities;
  sections: readonly PaletteSection[];
}): PaletteRowSources {
  return {
    commands: sections.flatMap(section => section.commands),
    sessions: entities.sessions,
    tabs: destination ? [] : entities.tabs,
    worktrees: destination ? [] : entities.worktrees,
    domainRows: domainSections.flatMap(section => section.rows),
  };
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
  const registry = usePaletteRegistry();
  const jumpToSession = useAttentionJump();
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
  const fallback = visiblePaletteFallback(
    assembly.fallback,
    assembly.sections.length,
    entities,
    domainSections
  );
  const landing = useOsPaletteLandingActions({
    close,
    destinationWindowId,
    entities,
    registeredWorkspaces,
    runtimeWorkspaceId,
  });
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
  const commandActions = useOsPaletteDispatchActions({
    activeWorkspaceId,
    close,
    dispatch,
    landing,
    openDomainRow,
    query,
    registry,
    setPinned,
  });

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
    rowSources: paletteRowSources({ destination, domainSections, entities, sections }),
    destinationWindowId,
    destination,
    destinationEmpty:
      destination && sections.length === 0 && entities.sessions.length === 0 && query === "",
    runCommand: commandActions.runCommand,
    runFallback: query => void fallbackSession.run(query),
    runRowAction: commandActions.runRowAction,
    openSession: landing.openSession,
    goToTab: landing.goToTab,
    selectWorktree: landing.selectWorktree,
    openDomainRow,
  };
}
