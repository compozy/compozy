import { createContext, use } from "react";

import type { Filter } from "@compozy/ui";

import type { TerminalJournalEntry } from "../types";

export interface TerminalJournalState {
  chips: Filter<string>[];
  compact: boolean;
  emptyMatch: boolean;
  entries: readonly TerminalJournalEntry[];
  examinedCount?: number;
  filtered: boolean;
  hasMore: boolean;
  inline: boolean;
  isLoadingMore: boolean;
  loaded: number | undefined;
  replay?: React.ReactNode;
  selected: TerminalJournalEntry | null;
  selectedId: string | null;
  showOwner: boolean;
}

export interface TerminalJournalActions {
  onCopyCommand?: (command: string) => void | Promise<void>;
  onFiltersChange: (chips: Filter<string>[]) => void;
  onLoadMore: () => void;
  onOpenNewTerminal?: () => void;
  onOpenTerminal?: (terminalId: string) => void;
  onReplay?: (recordingId: string, entry: TerminalJournalEntry) => void;
  select: (id: string | null) => void;
}

export const TerminalJournalStateContext = createContext<TerminalJournalState | null>(null);
export const TerminalJournalActionsContext = createContext<TerminalJournalActions | null>(null);

export function useTerminalJournalState(): TerminalJournalState {
  const state = use(TerminalJournalStateContext);
  if (state === null) throw new Error("Terminal journal state is unavailable");
  return state;
}

export function useTerminalJournalActions(): TerminalJournalActions {
  const actions = use(TerminalJournalActionsContext);
  if (actions === null) throw new Error("Terminal journal actions are unavailable");
  return actions;
}
