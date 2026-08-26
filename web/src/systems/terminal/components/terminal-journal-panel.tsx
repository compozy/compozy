"use client";

import { useState } from "react";

import {
  Button,
  Table,
  TableBody,
  TableHead,
  TableHeader,
  TableRow,
} from "@compozy/ui";

import type { TerminalJournalEntry, TerminalJournalFilters } from "../types";
import { TerminalJournalDetail } from "./terminal-journal-detail";
import { TerminalJournalEmpty } from "./terminal-journal-empty";
import { TerminalJournalRow } from "./terminal-journal-row";
import {
  TerminalJournalToolbar,
  type TerminalJournalFilterChip,
} from "./terminal-journal-toolbar";

export interface TerminalJournalPanelProps {
  entries: readonly TerminalJournalEntry[];
  filters: readonly TerminalJournalFilterChip[];
  /** True while another page is available behind the cursor. */
  hasMore: boolean;
  isLoadingMore?: boolean;
  /** Set only in the read-only all-profiles view. */
  showOwner?: boolean;
  onLoadMore: () => void;
  onClearFilter: (key: keyof TerminalJournalFilters) => void;
  onClearFilters: () => void;
  onOpenFilters?: () => void;
  onOpenTerminal?: (terminalId: string) => void;
  /** Offered from the empty journal, where there is no terminal to open yet. */
  onOpenNewTerminal?: () => void;
  onReplay?: (recordingId: string) => void;
}

/**
 * Every command in this project, in one place.
 *
 * Paging is cursor-driven and the footer states rows *loaded*: retention is
 * unbounded, so a total would mean reading the whole history and the panel will
 * not print a number nothing counted.
 */
export function TerminalJournalPanel({
  entries,
  filters,
  hasMore,
  isLoadingMore = false,
  showOwner = false,
  onLoadMore,
  onClearFilter,
  onClearFilters,
  onOpenFilters,
  onOpenTerminal,
  onOpenNewTerminal,
  onReplay,
}: TerminalJournalPanelProps) {
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const selected = entries.find(entry => entry.command_id === selectedId) ?? null;

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col" data-testid="terminal-journal">
      <TerminalJournalToolbar
        filters={filters}
        onClearFilter={onClearFilter}
        onOpenFilters={onOpenFilters}
      />

      {entries.length === 0 ? (
        <TerminalJournalEmpty
          filtered={filters.length > 0}
          hasMore={hasMore}
          onClearFilters={onClearFilters}
          onLoadMore={onLoadMore}
          // The all-profiles view is a read of every profile's history and
          // offers no mutation, by construction rather than by convention: a
          // terminal opened from here would silently belong to whichever
          // profile happened to be current.
          onOpenTerminal={showOwner ? undefined : onOpenNewTerminal}
        />
      ) : (
        <div
          className="grid min-h-0 flex-1 grid-cols-[minmax(0,1fr)] data-[detail]:grid-cols-[minmax(0,1fr)_21.25rem]"
          data-detail={selected ? "" : undefined}
        >
          <div className="min-h-0 overflow-y-auto">
            <Table aria-label="Command journal" className="table-fixed">
              <TableHeader>
                <TableRow>
                  <TableHead className="w-18">When</TableHead>
                  <TableHead className="w-35">Who</TableHead>
                  <TableHead>Command</TableHead>
                  <TableHead className="w-43">Result</TableHead>
                  {selected ? null : (
                    <>
                      <TableHead className="w-30">Permission</TableHead>
                      <TableHead className="w-29">Confidence</TableHead>
                      <TableHead aria-label="Recording" className="w-13" />
                    </>
                  )}
                </TableRow>
              </TableHeader>
              <TableBody>
                {entries.map(entry => (
                  <TerminalJournalRow
                    compact={Boolean(selected)}
                    entry={entry}
                    key={entry.command_id}
                    onReplay={
                      entry.recording && onReplay
                        ? () => onReplay(entry.recording as string)
                        : undefined
                    }
                    onSelect={() => setSelectedId(entry.command_id)}
                    selected={entry.command_id === selectedId}
                    showOwner={showOwner}
                  />
                ))}
              </TableBody>
            </Table>
          </div>
          {selected ? (
            <TerminalJournalDetail
              entry={selected}
              onClose={() => setSelectedId(null)}
              onOpenTerminal={
                selected.terminal_id && onOpenTerminal
                  ? () => onOpenTerminal(selected.terminal_id as string)
                  : undefined
              }
            />
          ) : null}
        </div>
      )}

      {entries.length > 0 ? (
        <div className="flex min-h-11.5 flex-none items-center gap-3 border-line border-t px-3.5 py-2.25">
          <span className="mr-auto text-badge text-subtle" data-testid="terminal-journal-loaded">
            {entries.length} {entries.length === 1 ? "row" : "rows"} loaded · newest first
          </span>
          {hasMore ? (
            <Button
              data-testid="terminal-journal-load-more"
              disabled={isLoadingMore}
              onClick={onLoadMore}
              size="sm"
              type="button"
              variant="ghost"
            >
              Load older rows
            </Button>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
