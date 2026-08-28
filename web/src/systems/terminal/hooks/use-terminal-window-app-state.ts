"use client";

import { useId, useState } from "react";

import { TERMINAL_JOURNAL_TAB, type TerminalTabId } from "../components/terminal-tab-id";
import type { TerminalWindowActions } from "../components/terminal-window-actions";
import type { TerminalInfo, TerminalInputRequest } from "../types";
import { useTerminalScopeCleanup } from "./use-terminal-scope-cleanup";
import { useTerminalStore } from "./use-terminal-store";

/** No terminal is selected, because the project has none yet. */
export const TERMINAL_NO_TERMINALS = "";

function hasTerminal(terminals: readonly TerminalInfo[], id: string): boolean {
  return terminals.some(terminal => terminal.id === id);
}

export interface UseTerminalWindowAppStateOptions {
  workspaceId: string;
  profile: string;
  terminals: readonly TerminalInfo[];
  inputRequests: readonly TerminalInputRequest[];
  /** The per-project cap, from `[terminal].max_per_workspace`. */
  limit: number;
  readOnly: boolean;
  actions: TerminalWindowActions;
  /**
   * The PTY the route named. `undefined` is an isolated window with no host
   * route. `null` is `/terminal` with no id. A string is `/terminal/:id`.
   */
  requestedTerminalId?: string | null;
  /** Retargets the host route to this PTY. Absent in isolated windows. */
  onSelectTerminal?: (terminalId: string) => void;
  /** Refreshes the journal when the operator reveals it. */
  onViewJournal?: (() => void) | undefined;
  /** The journal tab is no longer the surface being read. */
  onLeaveJournal?: (() => void) | undefined;
}

export interface TerminalWindowAppState {
  active: TerminalInfo | null;
  activeProfile: string;
  /** A unique symbol member widens across inference, so the shape is explicit. */
  activeTab: TerminalTabId;
  attentionIds: ReadonlySet<string>;
  destinationTerminals: TerminalInfo[];
  limitOpen: boolean;
  /** The route named a PTY that is not in this catalog. */
  missingRequested: boolean;
  openTerminal: (() => void) | undefined;
  setActiveTab: (tab: TerminalTabId) => void;
  setLimitOpen: (open: boolean) => void;
  tabsIdBase: string;
}

/**
 * The window's interaction state: which tab is looked at, whether the limit
 * dialog is open, and the scope those facts belong to.
 *
 * When the host names a PTY, that route is the only selection truth. The
 * journal is a local overlay on top of it — it is not a second PTY id.
 */
export function useTerminalWindowAppState({
  workspaceId,
  profile,
  terminals,
  inputRequests,
  limit,
  readOnly,
  actions,
  requestedTerminalId,
  onSelectTerminal,
  onViewJournal,
  onLeaveJournal,
}: UseTerminalWindowAppStateOptions): TerminalWindowAppState {
  const [selectedTab, setSelectedTab] = useState<TerminalTabId | null>(null);
  const [journalOpen, setJournalOpen] = useState(false);
  const [limitOpen, setLimitOpen] = useState(false);
  const tabsIdBase = useId();
  // Interaction state belongs to one `(workspace, profile)`: a dialog opened
  // for one project must not survive into another. Compared during render so
  // there is never a frame where the stale dialog shows over the new scope.
  const scope = `${workspaceId} ${profile}`;
  const [previousScope, setPreviousScope] = useState(scope);
  if (previousScope !== scope) {
    setPreviousScope(scope);
    setLimitOpen(false);
    setSelectedTab(null);
    setJournalOpen(false);
  }
  const attentionIds = new Set(inputRequests.map(request => request.terminal_id));
  const routeOwned = requestedTerminalId !== undefined;
  const missingRequested =
    routeOwned &&
    requestedTerminalId !== null &&
    !hasTerminal(terminals, requestedTerminalId) &&
    !journalOpen;
  const activeTab = resolveActiveTab({
    journalOpen,
    missingRequested,
    requestedTerminalId,
    routeOwned,
    selectedTab,
    terminals,
  });
  const active = terminals.find(terminal => terminal.id === activeTab) ?? null;
  const setActiveTab = (tab: TerminalTabId) => {
    if (tab === TERMINAL_JOURNAL_TAB) {
      if (journalOpen && terminals.length === 0) {
        setJournalOpen(false);
        onLeaveJournal?.();
        return;
      }
      setJournalOpen(true);
      onViewJournal?.();
      return;
    }
    setJournalOpen(false);
    onLeaveJournal?.();
    if (onSelectTerminal) {
      onSelectTerminal(tab);
      return;
    }
    setSelectedTab(tab);
  };
  const destinationTerminals = terminals.filter(terminal => terminal.profile_name === profile);
  const atLimit = destinationTerminals.length >= limit;
  const store = useTerminalStore();
  const activeProfile = active?.profile_name ?? profile;
  useTerminalScopeCleanup({ workspaceId, profile: activeProfile, terminals, store });

  // The daemon refuses a create at the cap, so the offer becomes the way out.
  const openTerminal =
    actions.onOpenTerminal && !readOnly
      ? () => {
          if (atLimit) {
            setLimitOpen(true);
            return;
          }
          actions.onOpenTerminal?.();
        }
      : undefined;

  return {
    active,
    activeProfile,
    activeTab,
    attentionIds,
    destinationTerminals,
    limitOpen,
    missingRequested,
    openTerminal,
    setActiveTab,
    setLimitOpen,
    tabsIdBase,
  };
}

function resolveActiveTab({
  journalOpen,
  missingRequested,
  requestedTerminalId,
  routeOwned,
  selectedTab,
  terminals,
}: {
  journalOpen: boolean;
  missingRequested: boolean;
  requestedTerminalId: string | null | undefined;
  routeOwned: boolean;
  selectedTab: TerminalTabId | null;
  terminals: readonly TerminalInfo[];
}): TerminalTabId {
  if (journalOpen) return TERMINAL_JOURNAL_TAB;
  if (missingRequested && requestedTerminalId) return requestedTerminalId;
  if (routeOwned) {
    if (typeof requestedTerminalId === "string" && hasTerminal(terminals, requestedTerminalId)) {
      return requestedTerminalId;
    }
    const first = terminals[0];
    return first ? first.id : TERMINAL_NO_TERMINALS;
  }
  if (
    selectedTab !== null &&
    (selectedTab === TERMINAL_JOURNAL_TAB || hasTerminal(terminals, selectedTab))
  ) {
    return selectedTab;
  }
  const first = terminals[0];
  return first ? first.id : TERMINAL_NO_TERMINALS;
}
