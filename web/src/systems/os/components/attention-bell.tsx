import { CircleAlert, ListChecks, SquareTerminal } from "lucide-react";

import { Icon, PopoverDescription, PopoverHeader, PopoverTitle } from "@agh/ui";

import type { OsAttentionRow } from "../lib/attention-model";

export interface AttentionBellProps {
  rows: readonly OsAttentionRow[];
  sessionsDisconnected: boolean;
  tasksDisconnected: boolean;
  loading: boolean;
  onSelect: (row: OsAttentionRow) => void;
}

export function AttentionBell({
  rows,
  sessionsDisconnected,
  tasksDisconnected,
  loading,
  onSelect,
}: AttentionBellProps) {
  const disconnected = sessionsDisconnected || tasksDisconnected;

  return (
    <div className="flex min-h-0 flex-col" data-testid="os-attention-bell">
      <PopoverHeader className="border-b border-line-soft px-1 pb-2">
        <PopoverTitle>Needs your attention</PopoverTitle>
        <PopoverDescription>Open the owning window to review and respond.</PopoverDescription>
      </PopoverHeader>
      {disconnected ? (
        <div
          role="status"
          className="flex items-start gap-2 rounded-md border border-warning/30 bg-warning-tint px-2.5 py-2 text-small-body text-warning"
          data-testid="os-bell-disconnected"
        >
          <Icon as={CircleAlert} size="sm" className="mt-0.5 shrink-0" />
          <span>
            {sessionsDisconnected && tasksDisconnected
              ? "Session and task attention are unavailable."
              : sessionsDisconnected
                ? "Session attention is unavailable."
                : "Task attention is unavailable."}
          </span>
        </div>
      ) : null}
      <div className="-mx-1 flex max-h-80 min-h-0 flex-col overflow-y-auto">
        {rows.map(row => (
          <button
            key={`${row.kind}:${row.id}`}
            type="button"
            className="flex w-full items-start gap-2 rounded-md px-2 py-2 text-left transition-colors hover:bg-row-hover focus-visible:shadow-focus-ring focus-visible:outline-none"
            data-testid={`os-attention-${row.kind}-${row.id}`}
            onClick={() => onSelect(row)}
          >
            <span className="mt-0.5 grid size-6 shrink-0 place-items-center rounded-sm border border-line bg-elevated text-muted">
              <Icon as={row.kind === "session" ? SquareTerminal : ListChecks} size="sm" />
            </span>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-small-body font-medium text-fg-strong">
                {row.title}
              </span>
              <span className="block truncate font-mono text-micro text-subtle">
                {row.kind === "session"
                  ? `${row.agentName} · waiting-for-auth`
                  : `${row.identifier ?? row.id} · awaiting approval`}
              </span>
            </span>
          </button>
        ))}
        {!loading && rows.length === 0 && !disconnected ? (
          <div className="px-2 py-5 text-center" data-testid="os-bell-empty">
            <p className="text-small-body font-medium text-fg-strong">Nothing waiting</p>
            <p className="mt-1 text-small-body text-muted">
              Waiting sessions and tasks awaiting approval will appear here.
            </p>
          </div>
        ) : null}
        {loading && rows.length === 0 ? (
          <p className="px-2 py-5 text-center text-small-body text-muted">Loading attention…</p>
        ) : null}
      </div>
    </div>
  );
}
