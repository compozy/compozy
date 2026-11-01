import { Play } from "@compozy/ui/icons";

import {
  Button,
  OwnerAvatar,
  Pill,
  TableCell,
  TableRow,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";

import { AgentMark } from "@/systems/agent/components/agent-mark";
import { useAgentIdentityView } from "@/systems/agent/hooks/use-agent-identity-view";

import { terminalApprovalCopy, terminalExitCopy } from "../lib/terminal-copy";
import { terminalJournalActorLabel } from "../lib/terminal-journal-actor";
import { formatTerminalJournalClock } from "../lib/terminal-journal-clock";
import type { TerminalJournalEntry } from "../types";
import { TerminalJournalConfidence } from "./terminal-journal-confidence";

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
/** An id that is a machine id has no agent to name, so the kind label keeps standing in. */
function TerminalJournalActor({ actor }: { actor: TerminalJournalEntry["actor"] }) {
  const identity = useAgentIdentityView(actor.id);
  const label = terminalJournalActorLabel(actor);
  if (actor.kind !== "agent") {
    return (
      <>
        <OwnerAvatar name={label} ownerId={actor.id} ownerKind={actor.kind} size="sm" />
        <span className="truncate text-form-input text-fg">{label}</span>
      </>
    );
  }
  return (
    <>
      <AgentMark
        appearance={identity.appearance}
        label={identity.known ? identity.label : label}
        quietIdle
        size={20}
        state="unknown"
      />
      <span className="truncate text-form-input text-fg">
        {identity.known ? identity.label : label}
      </span>
    </>
  );
}

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
  return (
    <TableRow
      aria-selected={selected}
      className="cursor-pointer"
      data-selected={selected ? "" : undefined}
      data-state={selected ? "selected" : undefined}
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
          {formatTerminalJournalClock(entry.started_at)}
        </span>
      </TableCell>
      <TableCell>
        <span className="flex min-w-0 items-center gap-1.5">
          {/* The actor's kind is the daemon's own: a system action is not a
              person, and folding it into `human` would put a face on the
              runtime. */}
          <TerminalJournalActor actor={entry.actor} />
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
      <TableCell className="min-w-0 whitespace-normal">
        <span className="block truncate font-mono text-code-block text-fg">{entry.command}</span>
        <span className="mt-0.75 block truncate font-mono text-eyebrow text-faint">
          {entry.cwd}
          {entry.terminal_id ? ` · ${entry.terminal_id}` : null}
        </span>
      </TableCell>
      <TableCell>
        <Pill
          form={outcome.hollow ? "hollow" : "tint"}
          size="sm"
          tone={outcome.tone === "success" ? "success" : "neutral"}
        >
          {outcome.label}
        </Pill>
        <span className="mt-0.75 block font-mono text-eyebrow tabular-nums whitespace-nowrap text-subtle">
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
            <TerminalJournalConfidence
              detectedBy={entry.detected_by}
              testId={`terminal-journal-confidence-${entry.command_id}`}
            />
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
                      variant="quiet"
                    />
                  }
                >
                  <Play aria-hidden="true" />
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
