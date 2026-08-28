import { X } from "lucide-react";

import { Button, DetailInspector, Eyebrow, MonoId, PropertyRow } from "@compozy/ui";

import { terminalApprovalCopy, terminalExitCopy } from "../lib/terminal-copy";
import { formatTerminalJournalClock } from "../lib/terminal-journal-clock";
import { copyTerminalJournalCommand } from "../lib/terminal-journal-copy";
import type { TerminalJournalEntry } from "../types";
import { TerminalJournalConfidence } from "./terminal-journal-confidence";
import { TerminalJournalOutput } from "./terminal-journal-output";

export interface TerminalJournalDetailProps {
  entry: TerminalJournalEntry;
  onClose: () => void;
  onOpenTerminal?: () => void;
  onCopyCommand?: (command: string) => void | Promise<void>;
  /** Close lives in the title only while the rail is inline. The drawer owns one. */
  inline?: boolean;
}

function focusOnMount(element: HTMLElement | null): void {
  element?.focus();
}

/**
 * The whole record for one command.
 *
 * Opening it puts the attribute columns away: they read better as rows here
 * than as columns fighting the command for width. Output size states the
 * byte count the row carries — not a reconstructed tail.
 */
export function TerminalJournalDetail({
  entry,
  onClose,
  onOpenTerminal,
  onCopyCommand,
  inline = false,
}: TerminalJournalDetailProps) {
  const outcome = terminalExitCopy({
    cause: entry.exit_cause,
    code: entry.exit_code,
    signal: entry.signal,
  });

  async function handleCopy(): Promise<void> {
    if (onCopyCommand) {
      await onCopyCommand(entry.command);
      return;
    }
    await copyTerminalJournalCommand(entry.command);
  }

  return (
    <DetailInspector
      aria-label="Command record"
      className="h-full min-h-0"
      onOpenChange={open => {
        if (!open) onClose();
      }}
      open
      title={
        inline ? (
          <>
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
          </>
        ) : (
          "Command record"
        )
      }
    >
      <div
        className="flex min-h-0 flex-1 flex-col overflow-y-auto focus-visible:outline-none"
        data-testid="terminal-journal-detail"
        onKeyDown={event => {
          if (event.key !== "Escape") return;
          event.stopPropagation();
          onClose();
        }}
        ref={focusOnMount}
        tabIndex={-1}
      >
        <div className="flex flex-col gap-2 px-4 py-3.5">
          <div className="rounded-xs bg-chat-fill-code px-2.5 py-2 font-mono text-form-input break-all whitespace-pre-wrap text-fg">
            {entry.command}
          </div>
          {entry.terminal_id ? <MonoId size="sm" value={entry.terminal_id} /> : null}
        </div>

        <div className="flex flex-col gap-1 px-4 pb-3.5">
          <PropertyRow label="Started" mono>
            {formatTerminalJournalClock(entry.started_at, { seconds: true })}
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
            <TerminalJournalConfidence detectedBy={entry.detected_by} />
          </PropertyRow>
          <PropertyRow label="Ran under">{entry.profile_name}</PropertyRow>
        </div>

        <TerminalJournalOutput outputBytes={entry.output_bytes} truncated={entry.truncated} />

        <div className="flex gap-1.75 px-4 pb-4.5">
          <Button
            data-testid="terminal-journal-copy-command"
            onClick={() => {
              void handleCopy();
            }}
            size="sm"
            type="button"
            variant="outline"
          >
            Copy command
          </Button>
          {onOpenTerminal ? (
            <Button onClick={onOpenTerminal} size="sm" type="button" variant="outline">
              Open terminal
            </Button>
          ) : null}
        </div>
      </div>
    </DetailInspector>
  );
}

function formatDuration(durationMs: number): string {
  if (durationMs < 1000) return `${durationMs}ms`;
  const seconds = durationMs / 1000;
  if (seconds < 60) return `${seconds.toFixed(1)}s`;
  const minutes = Math.floor(seconds / 60);
  return `${minutes}m ${Math.round(seconds % 60)}s`;
}
