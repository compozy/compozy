import { Bot, FileEdit, FileText, LoaderCircle, Search, Terminal, Wrench } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { createElement } from "react";

import { SessionWorkEntryView } from "./session-work-entry";
import {
  summaryFailureSuffix,
  type SessionToolGroupSummary,
  type SessionToolSummaryCategory,
} from "./session-timeline-summary";
import { isStreamingState, type SessionWorkRow } from "./session-timeline.logic";
import { TranscriptDisclosure } from "@compozy/ui";

export interface SessionToolGroupRowProps {
  row: SessionWorkRow;
  /** The owning turn ended in a turn-level failure (danger only then, ADR-009). */
  turnFailed: boolean;
  onToggle: () => void;
}

const CATEGORY_ICON: Record<SessionToolSummaryCategory, LucideIcon> = {
  command: Terminal,
  edit: FileEdit,
  read: FileText,
  search: Search,
  subagent: Bot,
  tool: Wrench,
};

/** The group's glyph names its dominant kind — the category with the most calls. */
function summaryIcon(summary: SessionToolGroupSummary): LucideIcon {
  const dominant = summary.parts.reduce<SessionToolGroupSummary["parts"][number] | null>(
    (best, part) => (best === null || part.count > best.count ? part : best),
    null
  );
  return dominant ? CATEGORY_ICON[dominant.category] : Wrench;
}

/**
 * A compact work summary opens the original ordered tool and reasoning
 * details, hung off one rail that drops from the group's glyph.
 */
export function SessionToolGroupRow({ row, turnFailed, onToggle }: SessionToolGroupRowProps) {
  const summary = row.summary;
  if (!summary) return null;
  const detailsId = `${row.groupId}:entries`;
  const failed = summaryFailureSuffix(summary);
  const busy = row.entries.some(entry =>
    entry.kind === "tool" ? entry.status === "running" : isStreamingState(entry.state)
  );
  return (
    <div
      data-testid="work-summary-row"
      data-open={row.expanded}
      data-live={row.active || undefined}
      className="flex min-w-0 flex-col"
    >
      <TranscriptDisclosure
        className="min-w-0 max-w-full"
        title={summary.label}
        expanded={row.expanded}
        onToggle={onToggle}
        aria-controls={detailsId}
        icon={
          busy ? (
            <LoaderCircle
              aria-hidden="true"
              className="size-3 shrink-0 animate-spin text-subtle motion-reduce:animate-none"
            />
          ) : (
            createElement(summaryIcon(summary), {
              "aria-hidden": true,
              className: "size-3.5 shrink-0 text-faint",
            })
          )
        }
        label={
          <span data-testid="work-summary-label">
            {summary.label}
            {failed ? (
              <span data-testid="work-summary-failed">
                <span aria-hidden="true" className="px-1.5 text-faint">
                  ·
                </span>
                {failed}
              </span>
            ) : null}
          </span>
        }
      />
      <div
        id={detailsId}
        data-testid="work-summary-entries"
        hidden={!row.expanded}
        aria-hidden={!row.expanded}
        inert={!row.expanded}
        className={row.expanded ? "session-rail flex min-w-0 flex-col" : undefined}
      >
        {row.expanded
          ? row.entries.map(entry => (
              <div key={`${entry.kind}:${entry.id}`} className="session-rail-step min-w-0">
                <SessionWorkEntryView
                  entry={entry}
                  disclosed
                  onRail
                  active={row.active}
                  turnFailed={turnFailed}
                />
              </div>
            ))
          : null}
      </div>
    </div>
  );
}
