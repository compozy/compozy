import { Asterisk, Info, SquareTerminal, X } from "lucide-react";

import { Button, Eyebrow, MonoId, Pill, PropertyRow, Time } from "@compozy/ui";

import {
  terminalApprovalCopy,
  terminalConfidenceCopy,
  terminalExitCopy,
} from "../lib/terminal-copy";
import type { TerminalJournalEntry } from "../types";

export interface TerminalJournalDetailProps {
  entry: TerminalJournalEntry;
  /** The last screen this command left, when the daemon still holds it. */
  lastOutput?: readonly string[];
  /** Length only — the characters never left the program. */
  redactedInputLength?: number;
  onClose: () => void;
  onOpenTerminal?: () => void;
}

/**
 * The whole record for one command.
 *
 * Opening it puts the attribute columns away: they read better as rows here
 * than as columns fighting the command for width. Hidden input stayed hidden —
 * the record keeps the length, never the characters.
 */
export function TerminalJournalDetail({
  entry,
  lastOutput,
  redactedInputLength,
  onClose,
  onOpenTerminal,
}: TerminalJournalDetailProps) {
  const outcome = terminalExitCopy({
    cause: entry.exit_cause,
    code: entry.exit_code,
    signal: entry.signal,
  });
  const confidence = terminalConfidenceCopy(entry.detected_by);
  return (
    <aside
      aria-label="Command record"
      className="flex min-w-0 flex-col overflow-y-auto border-line border-l bg-canvas-soft"
      data-testid="terminal-journal-detail"
    >
      <div className="flex min-h-10 flex-none items-center gap-2 border-line-soft border-b pr-2.5 pl-4.5">
        <Eyebrow className="mr-auto">Command record</Eyebrow>
        <Button
          aria-label="Close record"
          onClick={onClose}
          size="icon-sm"
          type="button"
          variant="ghost"
        >
          <X aria-hidden="true" className="size-3" />
        </Button>
      </div>

      <div className="flex flex-col gap-2 px-4.5 py-3.5">
        <div className="rounded-xs bg-chat-fill-code px-2.5 py-2 font-mono text-form-input break-all whitespace-pre-wrap text-fg">
          {entry.command}
        </div>
        {entry.terminal_id ? <MonoId size="sm" value={entry.terminal_id} /> : null}
      </div>

      <div className="flex flex-col gap-1 px-4.5 pb-3.5">
        <Eyebrow className="flex items-center gap-1.5">
          <Info aria-hidden="true" className="size-3" />
          Record
        </Eyebrow>
        <PropertyRow label="Started">
          <Time iso={entry.started_at} />
        </PropertyRow>
        <PropertyRow label="Ran for" mono>
          {entry.duration_ms === null ? "unknown" : formatDuration(entry.duration_ms)}
        </PropertyRow>
        <PropertyRow label="Where" mono>
          {entry.cwd}
        </PropertyRow>
        <PropertyRow label="Result">{`${outcome.label} · ${outcome.code}`}</PropertyRow>
        <PropertyRow label="Permission">{terminalApprovalCopy(entry.approval)}</PropertyRow>
        <PropertyRow label="Confidence">
          <Pill size="xs" tone={confidence.estimated ? "warning" : "neutral"}>
            {confidence.label}
          </Pill>
        </PropertyRow>
        <PropertyRow label="Ran under">{entry.profile_name}</PropertyRow>
      </div>

      {lastOutput && lastOutput.length > 0 ? (
        <div className="flex flex-col gap-1 px-4.5 pb-3.5">
          <Eyebrow className="flex items-center gap-1.5">
            <SquareTerminal aria-hidden="true" className="size-3" />
            Last output
          </Eyebrow>
          <div className="rounded-xs bg-terminal-bg px-2.75 py-2.25 font-mono text-form-input leading-relaxed text-terminal-fg">
            {lastOutput.map(line => (
              <span className="block break-all whitespace-pre-wrap" key={line}>
                {line}
              </span>
            ))}
            {redactedInputLength === undefined ? null : (
              <span
                className="mt-0.5 inline-flex items-center gap-1.25 rounded-xxs bg-badge-fill px-1.5 font-mono text-micro text-subtle"
                data-testid="terminal-redacted-marker"
              >
                <Asterisk aria-hidden="true" className="size-2.5" />
                hidden input · {redactedInputLength} characters
              </span>
            )}
          </div>
        </div>
      ) : null}

      {onOpenTerminal ? (
        <div className="flex gap-1.75 px-4.5 pb-4.5">
          <Button onClick={onOpenTerminal} size="sm" type="button" variant="outline">
            Open terminal
          </Button>
        </div>
      ) : null}
    </aside>
  );
}

function formatDuration(durationMs: number): string {
  if (durationMs < 1000) return `${durationMs}ms`;
  const seconds = durationMs / 1000;
  if (seconds < 60) return `${seconds.toFixed(1)}s`;
  const minutes = Math.floor(seconds / 60);
  return `${minutes}m ${Math.round(seconds % 60)}s`;
}
