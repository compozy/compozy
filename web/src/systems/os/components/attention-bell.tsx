import { Bell, CircleAlert } from "lucide-react";

import { Eyebrow, Icon } from "@compozy/ui";

import type { OsAttentionRow, OsAttentionSections } from "../lib/attention-model";
import { AttentionBellRow } from "./attention-bell-row";

function BellSection({
  label,
  rows,
  onSelect,
  testId,
}: {
  label: string;
  rows: readonly OsAttentionRow[];
  onSelect: (row: OsAttentionRow) => void;
  testId: string;
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
        <AttentionBellRow key={`${row.kind}:${row.id}`} row={row} onSelect={onSelect} />
      ))}
    </section>
  );
}

export interface AttentionBellProps {
  sections: OsAttentionSections;
  sessionsDisconnected: boolean;
  tasksDisconnected: boolean;
  loading: boolean;
  onSelect: (row: OsAttentionRow) => void;
}

/**
 * Two sections, populated only: **Needs you** (questions, permission gates,
 * failures, task approvals) and **Finished** (work that completed while the
 * operator was elsewhere). Finished is visible but never counted — the badge
 * stays a trustworthy "how many are blocked on me" number.
 *
 * There is no dismiss control anywhere: a `done` marker clears by viewing the
 * session and by nothing else, so a manual "mark seen" would be a lie the
 * runtime cannot honour.
 */
export function AttentionBell({
  sections,
  sessionsDisconnected,
  tasksDisconnected,
  loading,
  onSelect,
}: AttentionBellProps) {
  const disconnected = sessionsDisconnected || tasksDisconnected;
  const empty = sections.needsYou.length === 0 && sections.finished.length === 0;

  return (
    <div className="flex min-h-0 flex-col" data-testid="os-attention-bell">
      {disconnected ? (
        <div
          role="status"
          className="flex items-start gap-2 rounded-md border border-warning/30 bg-warning-tint px-2.5 py-2 text-small-body text-warning"
          data-testid="os-bell-disconnected"
        >
          <Icon as={CircleAlert} size="sm" className="mt-0.5 shrink-0" />
          <span>
            {sessionsDisconnected && tasksDisconnected
              ? "Session and task attention are unavailable. Listed rows are frozen and do not count."
              : sessionsDisconnected
                ? "Session attention is unavailable. Listed rows are frozen and do not count."
                : "Task attention is unavailable."}
          </span>
        </div>
      ) : null}
      <div className="-mx-1 flex max-h-96 min-h-0 flex-col overflow-y-auto">
        <BellSection
          label="Needs you"
          rows={sections.needsYou}
          onSelect={onSelect}
          testId="os-bell-needs-you"
        />
        <BellSection
          label="Finished"
          rows={sections.finished}
          onSelect={onSelect}
          testId="os-bell-finished"
        />
        {!loading && empty && !disconnected ? (
          <div
            className="flex flex-col items-center gap-1.5 px-2 py-7 text-center"
            data-testid="os-bell-empty"
          >
            <Icon as={Bell} size="lg" className="text-faint" />
            <p className="text-small-body font-medium text-fg-strong">All quiet</p>
            <p className="text-small-body text-muted">Nothing needs you.</p>
          </div>
        ) : null}
        {loading && empty ? (
          <p className="px-2 py-5 text-center text-small-body text-muted">Loading attention…</p>
        ) : null}
      </div>
    </div>
  );
}
