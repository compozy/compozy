"use client";

import { ArrowLeft, ScrollText } from "lucide-react";
import { useState, type ReactNode } from "react";

import {
  Button,
  DETAIL_INSPECTOR_INLINE_BREAKPOINT,
  Table,
  TableBody,
  TableHead,
  TableHeader,
  TableRow,
  useInlineLayout,
  type Filter,
} from "@compozy/ui";

import type { TerminalJournalEntry } from "../types";
import {
  TerminalJournalActionsContext,
  TerminalJournalStateContext,
  useTerminalJournalActions,
  useTerminalJournalState,
  type TerminalJournalActions,
  type TerminalJournalState,
} from "../hooks/use-terminal-journal-context";
import { TerminalJournalDetail } from "./terminal-journal-detail";
import { TerminalJournalEmpty } from "./terminal-journal-empty";
import { TerminalJournalFooter } from "./terminal-journal-footer";
import { TerminalJournalRow } from "./terminal-journal-row";
import { TerminalJournalToolbar } from "./terminal-journal-toolbar";

/**
 * The journal's identity row — the same anatomy as a terminal's head.
 *
 * The pinned tab says where you are; this row says what you are reading and
 * for which project, once, before the table takes over.
 */
export function TerminalJournalHead({
  projectLabel,
  onBack,
}: {
  projectLabel?: string;
  /** Returns to the terminal underneath the overlay. */
  onBack?: () => void;
}) {
  return (
    <header
      className="flex min-h-11 flex-none items-center gap-2.5 border-line border-b bg-canvas px-3"
      data-testid="terminal-journal-head"
    >
      {onBack ? (
        <Button
          aria-label="Back to terminal"
          data-testid="terminal-journal-back"
          onClick={onBack}
          size="icon-sm"
          type="button"
          variant="ghost"
        >
          <ArrowLeft aria-hidden="true" className="size-3.5" />
        </Button>
      ) : null}
      <ScrollText aria-hidden="true" className="size-3.5 text-muted" />
      <span className="truncate font-semibold text-fg-strong text-ws-name tracking-tight">
        Journal
      </span>
      {projectLabel ? (
        <span className="truncate text-badge text-subtle">{projectLabel}</span>
      ) : null}
    </header>
  );
}

export interface TerminalJournalPanelProps {
  entries: readonly TerminalJournalEntry[];
  /** The filter chips, exactly as the `Filters` primitive edits them. */
  chips: Filter<string>[];
  /** True while another page is available behind the cursor. */
  hasMore: boolean;
  isLoadingMore?: boolean;
  /** Set only in the read-only all-profiles view. */
  showOwner?: boolean;
  /** Rows this query examined. Omit until the host has counted them. */
  examinedCount?: number;
  /** Rows currently in the table. Defaults to `entries.length`, or examined on a miss. */
  loadedCount?: number;
  /** In-table replay. The host keeps this panel mounted; the table stays visible. */
  replay?: ReactNode;
  /** Controlled selection so "Open in journal" can restore the row. */
  selectedCommandId?: string | null;
  onSelectedCommandIdChange?: (id: string | null) => void;
  onLoadMore: () => void;
  onFiltersChange: (chips: Filter<string>[]) => void;
  onOpenTerminal?: (terminalId: string) => void;
  /** Offered from the empty journal, where there is no terminal to open yet. */
  onOpenNewTerminal?: () => void;
  onReplay?: (recordingId: string, entry: TerminalJournalEntry) => void;
  onCopyCommand?: (command: string) => void | Promise<void>;
}

/**
 * Every command in this project, in one place.
 *
 * Paging is cursor-driven and the footer states rows *loaded*: retention is
 * unbounded, so a total would mean reading the whole history and the panel will
 * not print a number nothing counted.
 */
export function TerminalJournalPanel(props: TerminalJournalPanelProps) {
  return (
    <TerminalJournalProvider {...props}>
      <div className="flex min-h-0 min-w-0 flex-1 flex-col" data-testid="terminal-journal">
        <TerminalJournalFilters />
        <TerminalJournalContent />
        <TerminalJournalPaging />
      </div>
    </TerminalJournalProvider>
  );
}

function TerminalJournalProvider({
  children,
  entries,
  chips,
  hasMore,
  isLoadingMore = false,
  showOwner = false,
  examinedCount,
  loadedCount,
  replay,
  selectedCommandId,
  onSelectedCommandIdChange,
  onLoadMore,
  onFiltersChange,
  onOpenTerminal,
  onOpenNewTerminal,
  onReplay,
  onCopyCommand,
}: TerminalJournalPanelProps & { children: ReactNode }) {
  const [internalId, setInternalId] = useState<string | null>(null);
  const selectedId = selectedCommandId !== undefined ? selectedCommandId : internalId;
  const selected = entries.find(entry => entry.command_id === selectedId) ?? null;
  const filtered = chips.length > 0;
  const emptyMatch = filtered && entries.length === 0;
  const loaded = emptyMatch ? (loadedCount ?? examinedCount) : (loadedCount ?? entries.length);
  const inline = useInlineLayout(DETAIL_INSPECTOR_INLINE_BREAKPOINT);
  const compact = inline && Boolean(selected);

  const select = (id: string | null): void => {
    if (selectedCommandId === undefined) setInternalId(id);
    onSelectedCommandIdChange?.(id);
  };
  const state: TerminalJournalState = {
    chips,
    compact,
    emptyMatch,
    entries,
    examinedCount,
    filtered,
    hasMore,
    inline,
    isLoadingMore,
    loaded,
    replay,
    selected,
    selectedId,
    showOwner,
  };
  const actions: TerminalJournalActions = {
    onCopyCommand,
    onFiltersChange,
    onLoadMore,
    onOpenNewTerminal,
    onOpenTerminal,
    onReplay,
    select,
  };

  return (
    <TerminalJournalStateContext value={state}>
      <TerminalJournalActionsContext value={actions}>{children}</TerminalJournalActionsContext>
    </TerminalJournalStateContext>
  );
}

function TerminalJournalFilters() {
  const { chips } = useTerminalJournalState();
  const { onFiltersChange } = useTerminalJournalActions();
  return <TerminalJournalToolbar chips={chips} onChange={onFiltersChange} />;
}

function TerminalJournalContent() {
  const { entries, examinedCount, filtered, hasMore, replay, showOwner } =
    useTerminalJournalState();
  const { onFiltersChange, onLoadMore, onOpenNewTerminal } = useTerminalJournalActions();
  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      {replay ? (
        <div
          className="flex min-h-48 min-w-0 flex-1 flex-col border-line border-b"
          data-testid="terminal-journal-replay"
        >
          {replay}
        </div>
      ) : null}
      {entries.length === 0 ? (
        <TerminalJournalEmpty
          examinedCount={examinedCount}
          filtered={filtered}
          hasMore={hasMore}
          onClearFilters={() => onFiltersChange([])}
          onLoadMore={onLoadMore}
          // The all-profiles view is a read of every profile's history and
          // offers no mutation, by construction rather than by convention.
          onOpenTerminal={showOwner ? undefined : onOpenNewTerminal}
        />
      ) : (
        <TerminalJournalTable />
      )}
    </div>
  );
}

function TerminalJournalTable() {
  const { compact, entries, inline, replay, selected, selectedId, showOwner } =
    useTerminalJournalState();
  const { onCopyCommand, onOpenTerminal, onReplay, select } = useTerminalJournalActions();
  const selectedTerminalId = selected?.terminal_id;
  const openSelectedTerminal =
    selectedTerminalId && onOpenTerminal ? () => onOpenTerminal(selectedTerminalId) : undefined;
  return (
    <div className="flex min-h-0 min-w-0 flex-1">
      <div className="min-h-0 min-w-0 flex-1 overflow-y-auto">
        <Table aria-label="Command journal" className="table-fixed" overflowX="hidden">
          <TableHeader>
            <TableRow>
              <TableHead className="w-18">When</TableHead>
              <TableHead className="w-35">Who</TableHead>
              <TableHead>Command</TableHead>
              <TableHead className="w-43">Result</TableHead>
              {compact ? null : (
                <>
                  <TableHead className="w-30">Permission</TableHead>
                  <TableHead className="w-29">Confidence</TableHead>
                  <TableHead aria-label="Recording" className="w-13" />
                </>
              )}
            </TableRow>
          </TableHeader>
          <TableBody>
            {entries.map((entry, index) => {
              const recordingId = entry.recording;
              return (
                <TerminalJournalRow
                  compact={compact}
                  entry={entry}
                  focusStop={selected ? entry.command_id === selected.command_id : index === 0}
                  key={entry.command_id}
                  onReplay={
                    recordingId && onReplay ? () => onReplay(recordingId, entry) : undefined
                  }
                  onSelect={() => select(entry.command_id)}
                  selected={entry.command_id === selectedId}
                  showOwner={showOwner}
                />
              );
            })}
          </TableBody>
        </Table>
      </div>
      {selected && !replay ? (
        <TerminalJournalDetail
          entry={selected}
          inline={inline}
          onClose={() => select(null)}
          onCopyCommand={onCopyCommand}
          onOpenTerminal={openSelectedTerminal}
        />
      ) : null}
    </div>
  );
}

function TerminalJournalPaging() {
  const { chips, emptyMatch, entries, filtered, hasMore, isLoadingMore, loaded } =
    useTerminalJournalState();
  const { onLoadMore } = useTerminalJournalActions();
  if (entries.length === 0 && !filtered) return null;
  return (
    <TerminalJournalFooter
      emptyMatch={emptyMatch}
      filterCount={chips.length}
      hasMore={hasMore}
      isLoadingMore={isLoadingMore}
      loadedCount={loaded}
      onLoadMore={onLoadMore}
    />
  );
}
