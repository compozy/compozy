import { Check } from "lucide-react";

import { SessionToolCallRow } from "@/systems/session";

import { useOptionalSessionNavigationTarget } from "./hooks/session-navigation-target-context";
import { summaryFailureSuffix } from "./session-timeline-summary";
import { toolMessageFromPart } from "./session-timeline-tool-message";
import type { SessionWorkRow } from "./session-timeline.logic";
import { TranscriptDisclosure } from "./transcript-disclosure";

export interface SessionToolGroupRowProps {
  row: SessionWorkRow;
  /** The owning turn ended in a turn-level failure (danger only then, ADR-009). */
  turnFailed: boolean;
  onToggle: () => void;
}

/**
 * `SessionToolGroupRow` (ADR-006 rule 1): two or more settled tools rest as one
 * sentence — "Ran 6 commands, edited 2 files, read 3 files" — with a check in
 * the well and the chevron on hover. A failure the turn absorbed appends
 * "· 1 failed" in the same ink. Open reveals the production ToolCallRows, each
 * expandable on its own; opening never yanks the viewport (ADR-007).
 */
export function SessionToolGroupRow({ row, turnFailed, onToggle }: SessionToolGroupRowProps) {
  const navigation = useOptionalSessionNavigationTarget();
  const summary = row.summary;
  if (!summary) return null;
  const reveal = navigation?.reveal ?? null;
  // The tool row holding the revealed part opens its body when the matched
  // field lives there; the reader closing it releases that hold only.
  const revealFor = (tool: { toolCallId: string; partIndex?: number }) => {
    const id = `tool:${tool.toolCallId}`;
    const held =
      reveal !== null &&
      reveal.opensBody &&
      reveal.partIndex !== null &&
      tool.partIndex === reveal.partIndex &&
      !(navigation?.released.has(id) ?? false);
    return {
      onRevealRelease: () => navigation?.releaseDisclosure(id),
      revealOpen: held,
      ...(held && reveal.field ? { revealField: reveal.field } : {}),
    };
  };
  const detailsId = `${row.groupId}:entries`;
  const failed = summaryFailureSuffix(summary);
  return (
    <div
      data-testid="work-summary-row"
      data-open={row.expanded}
      data-live={row.active || undefined}
      className="flex min-w-0 flex-col"
    >
      <TranscriptDisclosure
        expanded={row.expanded}
        onToggle={onToggle}
        aria-controls={detailsId}
        icon={
          <Check aria-hidden="true" className="size-3 shrink-0 text-subtle" strokeWidth={1.8} />
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
        className={row.expanded ? "flex min-w-0 flex-col gap-0.5 pt-0.5" : undefined}
      >
        {row.expanded
          ? row.entries.map(tool => (
              <SessionToolCallRow
                key={tool.id}
                message={toolMessageFromPart(tool)}
                partIndex={tool.partIndex}
                turnSettled
                interrupted={tool.status === "interrupted"}
                turnFailed={turnFailed}
                {...revealFor(tool)}
              />
            ))
          : null}
      </div>
    </div>
  );
}
