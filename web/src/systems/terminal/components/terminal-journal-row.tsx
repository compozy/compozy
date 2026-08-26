import { Play, TriangleAlert } from "lucide-react";

import {
  Button,
  cn,
  OwnerAvatar,
  Pill,
  TableCell,
  TableRow,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";

import {
  terminalApprovalCopy,
  terminalConfidenceCopy,
  terminalExitCopy,
} from "../lib/terminal-copy";
import type { TerminalJournalEntry } from "../types";

export interface TerminalJournalRowProps {
  entry: TerminalJournalEntry;
  selected: boolean;
  /** The table is one composite: exactly one row is the Tab stop. */
  focusStop: boolean;
  /** Names the owning profile. Only the read-only all-profiles view sets it. */
  showOwner?: boolean;
  /** Attribute columns fold away while the record rail is open. */
  compact?: boolean;
  onSelect: () => void;
  onReplay?: () => void;
}

/**
 * One command, read as a row.
 *
 * The command is the title; the folder and the terminal it ran in are demoted
 * beneath it. Everything else is an attribute. Rows are calm at rest — the one
 * warning on this screen marks a guess, never a failure.
 */
export function TerminalJournalRow({
  entry,
  selected,
  focusStop,
  showOwner = false,
  compact = false,
  onSelect,
  onReplay,
}: TerminalJournalRowProps) {
  const outcome = terminalExitCopy({
    cause: entry.exit_cause,
    code: entry.exit_code,
    signal: entry.signal,
  });
  const confidence = terminalConfidenceCopy(entry.detected_by);
  return (
    <TableRow
      aria-selected={selected}
      className="cursor-pointer"
      data-selected={selected ? "" : undefined}
      data-testid={`terminal-journal-row-${entry.command_id}`}
      onClick={onSelect}
      // The row opens the record, so it is operable the way a control is:
      // reachable by Tab, chosen by Enter or Space. A click-only row hides the
      // journal from anyone not using a mouse.
      onKeyDown={event => {
        if (event.target !== event.currentTarget) return;
        if (event.key === "ArrowDown" || event.key === "ArrowUp") {
          event.preventDefault();
          const sibling =
            event.key === "ArrowDown"
              ? event.currentTarget.nextElementSibling
              : event.currentTarget.previousElementSibling;
          if (sibling instanceof HTMLElement) sibling.focus();
          return;
        }
        if (event.key !== "Enter" && event.key !== " ") return;
        event.preventDefault();
        onSelect();
      }}
      role="row"
      tabIndex={focusStop ? 0 : -1}
    >
      <TableCell>
        <span className="font-mono tabular-nums whitespace-nowrap text-fg text-transcript-meta">
          {formatClock(entry.started_at)}
        </span>
      </TableCell>
      <TableCell>
        <span className="flex min-w-0 items-center gap-1.5">
          {/* The actor's kind is the daemon's own: a system action is not a
              person, and folding it into `human` would put a face on the
              runtime. */}
          <OwnerAvatar
            name={entry.actor.id}
            ownerId={entry.actor.id}
            ownerKind={entry.actor.kind}
            size="sm"
          />
          <span className="truncate text-form-input text-fg">{entry.actor.id}</span>
          {showOwner ? (
            <Pill
              data-testid={`terminal-journal-owner-${entry.command_id}`}
              mono
              size="xs"
              tone="neutral"
            >
              {entry.profile_name}
            </Pill>
          ) : null}
        </span>
      </TableCell>
      <TableCell>
        <span className="block truncate font-mono text-code-block text-fg-strong">
          {entry.command}
        </span>
        <span className="mt-0.75 block truncate font-mono text-micro text-faint">
          {entry.cwd}
          {entry.terminal_id ? ` · ${entry.terminal_id}` : null}
        </span>
      </TableCell>
      <TableCell>
        <Pill size="sm" tone={outcome.tone === "success" ? "success" : "neutral"}>
          {outcome.label}
        </Pill>
        <span className="mt-0.75 block font-mono text-micro tabular-nums whitespace-nowrap text-subtle">
          {outcome.code}
        </span>
      </TableCell>
      {compact ? null : (
        <>
          <TableCell>
            <span className="text-form-input text-subtle">
              {terminalApprovalCopy(entry.approval)}
            </span>
          </TableCell>
          <TableCell>
            <Pill
              className={cn(confidence.estimated && "gap-1")}
              data-testid={`terminal-journal-confidence-${entry.command_id}`}
              size="xs"
              tone={confidence.estimated ? "warning" : "neutral"}
            >
              {confidence.estimated ? (
                <TriangleAlert aria-hidden="true" className="size-2.5" />
              ) : null}
              {confidence.label}
            </Pill>
          </TableCell>
          <TableCell>
            {entry.recording && onReplay ? (
              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      aria-label={`Replay ${entry.command}`}
                      data-testid={`terminal-journal-replay-${entry.command_id}`}
                      onClick={event => {
                        event.stopPropagation();
                        onReplay();
                      }}
                      size="icon-sm"
                      type="button"
                      variant="ghost"
                    />
                  }
                >
                  <Play aria-hidden="true" className="size-3" />
                </TooltipTrigger>
                <TooltipContent side="left">Replay this recording</TooltipContent>
              </Tooltip>
            ) : null}
          </TableCell>
        </>
      )}
    </TableRow>
  );
}

/**
 * Wall-clock only: the row is scanned, and the full stamp lives in the rail.
 *
 * Fixed 24-hour, because these are operational values compared down a column —
 * an AM/PM suffix would break the tabular alignment the scan depends on.
 */
function formatClock(iso: string): string {
  const parsed = new Date(iso);
  if (Number.isNaN(parsed.getTime())) return iso;
  return parsed.toLocaleTimeString(undefined, {
    hour: "2-digit",
    hour12: false,
    minute: "2-digit",
  });
}
