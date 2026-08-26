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
  /** Refreshes the journal when the operator reveals it. */
  onViewJournal?: (() => void) | undefined;
}

export interface TerminalWindowAppState {
  active: TerminalInfo | null;
  activeProfile: string;
  /** A unique symbol member widens across inference, so the shape is explicit. */
  activeTab: TerminalTabId;
  attentionIds: ReadonlySet<string>;
  destinationTerminals: TerminalInfo[];
  limitOpen: boolean;
  openTerminal: (() => void) | undefined;
  setActiveTab: (tab: TerminalTabId) => void;
  setLimitOpen: (open: boolean) => void;
  tabsIdBase: string;
}

/**
 * The window's interaction state: which tab is looked at, whether the limit
 * dialog is open, and the scope those facts belong to.
 *
 * An empty project opens on the terminals view, not on the journal: with
 * nothing running, the honest first offer is opening a terminal.
 */
export function useTerminalWindowAppState({
  workspaceId,
  profile,
  terminals,
  inputRequests,
  limit,
  readOnly,
  actions,
  onViewJournal,
}: UseTerminalWindowAppStateOptions): TerminalWindowAppState {
  const [selectedTab, setSelectedTab] = useState<TerminalTabId | null>(null);
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
  }
  const attentionIds = new Set(inputRequests.map(request => request.terminal_id));
  const selected =
    selectedTab !== null &&
    (selectedTab === TERMINAL_JOURNAL_TAB || hasTerminal(terminals, selectedTab))
      ? selectedTab
      : (terminals[0]?.id ?? null);
  const activeTab: TerminalTabId = selected ?? TERMINAL_NO_TERMINALS;
  const active = terminals.find(terminal => terminal.id === activeTab) ?? null;
  const setActiveTab = (tab: TerminalTabId) => {
    if (tab === TERMINAL_JOURNAL_TAB) onViewJournal?.();
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
    openTerminal,
    setActiveTab,
    setLimitOpen,
    tabsIdBase,
  };
}
