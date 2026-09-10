import { Bell, Check, CircleAlert } from "lucide-react";

import { Button, Eyebrow, Icon } from "@compozy/ui";

import type { OsAttentionRow, OsAttentionSections } from "../lib/attention-model";
import { AttentionBellRow } from "./attention-bell-row";

function BellSection({
  label,
  rows,
  onSelect,
  testId,
  onAcknowledge,
  disabled,
}: {
  label: string;
  rows: readonly OsAttentionRow[];
  onSelect: (row: OsAttentionRow) => void;
  testId: string;
  onAcknowledge?: (row?: OsAttentionRow) => void;
  disabled: boolean;
}) {
  if (rows.length === 0) return null;
  return (
    <section
      className="flex flex-col border-line pt-1.5 first:pt-0 [&+&]:mt-1 [&+&]:border-t"
      data-testid={testId}
    >
      <Eyebrow className="flex items-center gap-1.5 px-2.5 pt-1.5 pb-1 text-subtle">
        {label}
        <span className="font-mono text-micro text-faint">{rows.length}</span>
      </Eyebrow>
      {rows.map(row => (
        <div
          className="flex min-w-0 items-center"
          key={row.notificationId ?? `${row.kind}:${row.id}`}
        >
          <AttentionBellRow row={row} onSelect={onSelect} />
          <Button
            size="icon-sm"
            variant="ghost"
            aria-label={`Mark ${row.title} as read`}
            disabled={disabled || !row.notificationId || !onAcknowledge}
            onClick={() => onAcknowledge?.(row)}
          >
            <Check aria-hidden="true" />
          </Button>
        </div>
      ))}
    </section>
  );
}

export interface AttentionBellProps {
  sections: OsAttentionSections;
  total?: number;
  pending?: boolean;
  error?: string | null;
  onAcknowledge?: (row?: OsAttentionRow) => void;
  sessionsDisconnected: boolean;
  tasksDisconnected: boolean;
  loopRequestsDisconnected?: boolean;
  loading: boolean;
  onSelect: (row: OsAttentionRow) => void;
}

/** Acknowledgement removes notifications while source actions remain available in their owning app. */
export function AttentionBell({
  sections,
  total,
  pending = false,
  error,
  onAcknowledge,
  sessionsDisconnected,
  tasksDisconnected,
  loopRequestsDisconnected = false,
  loading,
  onSelect,
}: AttentionBellProps) {
  const disconnected = sessionsDisconnected || tasksDisconnected || loopRequestsDisconnected;
  const unavailable = [
    sessionsDisconnected ? "session" : null,
    tasksDisconnected ? "task" : null,
    loopRequestsDisconnected ? "loop request" : null,
  ].filter((source): source is string => source !== null);
  const unavailableLabel = unavailable.join(" and ");
  const empty = sections.needsYou.length === 0 && sections.finished.length === 0;

  return (
    <div className="flex min-h-0 flex-col" data-testid="os-attention-bell">
      <div className="flex items-center justify-between gap-2 border-b border-line px-1 pb-2">
        <span className="text-micro text-subtle">All workspaces · all profiles</span>
        <Button
          size="sm"
          variant="ghost"
          disabled={pending || disconnected || empty || !onAcknowledge}
          onClick={() => onAcknowledge?.()}
        >
          {pending ? "Clearing…" : "Clear all"}
        </Button>
      </div>
      {error ? (
        <p role="alert" className="px-2 py-2 text-small-body text-danger">
          {error}
        </p>
      ) : null}
      {total !== undefined && total > sections.needsYou.length + sections.finished.length ? (
        <p className="px-2 py-2 text-micro text-subtle">
          Showing {sections.needsYou.length + sections.finished.length} of {total}. Clear all
          includes every notification.
        </p>
      ) : null}
      {disconnected ? (
        <div
          role="status"
          className="flex items-start gap-2 rounded-md border border-warning/30 bg-warning-tint px-2.5 py-2 text-small-body text-warning"
          data-testid="os-bell-disconnected"
        >
          <Icon as={CircleAlert} size="sm" className="mt-0.5 shrink-0" />
          <span>{`${unavailableLabel.charAt(0).toUpperCase()}${unavailableLabel.slice(1)} attention ${unavailable.length === 1 ? "is" : "are"} unavailable. Frozen rows do not count.`}</span>
        </div>
      ) : null}
      <div className="-mx-1 flex max-h-96 min-h-0 flex-col overflow-y-auto">
        <BellSection
          label="Needs you"
          rows={sections.needsYou}
          onSelect={onSelect}
          onAcknowledge={onAcknowledge}
          disabled={pending || disconnected}
          testId="os-bell-needs-you"
        />
        <BellSection
          label="Finished"
          rows={sections.finished}
          onSelect={onSelect}
          onAcknowledge={onAcknowledge}
          disabled={pending || disconnected}
          testId="os-bell-finished"
        />
        {!loading && empty && !disconnected ? (
          <div
            className="flex flex-col items-center gap-1.5 px-2 py-7 text-center"
            data-testid="os-bell-empty"
          >
            <Icon as={Bell} size="lg" className="text-faint" />
            <p className="text-small-body font-medium text-fg-strong">All quiet</p>
            <p className="text-small-body text-muted">No unread notifications.</p>
          </div>
        ) : null}
        {loading && empty ? (
          <p className="px-2 py-5 text-center text-small-body text-muted">Loading attention…</p>
        ) : null}
      </div>
    </div>
  );
}
