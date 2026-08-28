"use client";

import { TerminalView, type TerminalEngineLoader } from "@compozy/ui";
import { TerminalSquare } from "lucide-react";
import { useState } from "react";

import { cn } from "@/lib/utils";
import { terminalReplayFailedCopy, useTerminalReplay } from "@/systems/terminal/parts";
import type { AgentEventPayload } from "../types";

const REPORTED_COLLAPSED_LINE_LIMIT = 12;
const BYTE_FORMATTER = new Intl.NumberFormat();

export interface SessionAgentReportedBlockProps {
  data: AgentEventPayload;
  /** Replaces the emulator for deterministic playback harnesses. */
  engineLoader?: TerminalEngineLoader;
}

function lineCount(text: string): number {
  return text.length > 0 ? text.split("\n").length : 0;
}

function clampLines(text: string, expanded: boolean): string {
  if (expanded) return text;
  const lines = text.split("\n");
  if (lines.length <= REPORTED_COLLAPSED_LINE_LIMIT) return text;
  return lines.slice(0, REPORTED_COLLAPSED_LINE_LIMIT).join("\n");
}

function reportedAriaLabel(title: string | undefined, collapsed: boolean): string {
  const base = title ? `${title} — reported by the agent` : "Reported by the agent";
  return collapsed ? `${base} — collapsed` : base;
}

/** Provenance mark — not a status pill, and outside the signal palette. */
function SessionReportedMark() {
  return (
    <span className="inline-flex h-4.5 items-center rounded-xxs bg-neutral-tint px-1.5 font-medium text-micro text-muted">
      reported by agent
    </span>
  );
}

/** A read-only transcript specimen of terminal output supplied by an agent. */
export function SessionAgentReportedBlock({ data, engineLoader }: SessionAgentReportedBlockProps) {
  const terminal = data.reported_terminal;
  const output = data.text ?? "";
  const instanceId = reportedTerminalInstanceId(data);
  const overflow = lineCount(output) > REPORTED_COLLAPSED_LINE_LIMIT;
  const [expanded, setExpanded] = useState(false);
  const visibleOutput = clampLines(output, expanded || !overflow);
  const replay = useTerminalReplay(instanceId, visibleOutput, output.length > 0);
  const hiddenCount = Math.max(0, lineCount(output) - REPORTED_COLLAPSED_LINE_LIMIT);

  if (data.origin !== "agent_reported" || !terminal || output.length === 0) return null;

  const title = data.title?.trim() || undefined;
  return (
    <div
      className="max-w-160 overflow-hidden rounded-md border border-line"
      data-testid={`session-agent-reported-block-${terminal.id}`}
    >
      <div className="flex min-h-8 min-w-0 items-center gap-2 border-line border-b bg-canvas px-2.5">
        <TerminalSquare aria-hidden="true" className="size-deck-glyph flex-none text-subtle" />
        {title ? (
          <span className="min-w-0 truncate font-semibold text-fg text-transcript-body">
            {title}
          </span>
        ) : null}
        <div className="ml-auto flex-none">
          <SessionReportedMark />
        </div>
      </div>
      <div className="flex max-h-55 flex-col bg-terminal-bg">
        <TerminalView
          aria-label={reportedAriaLabel(title, overflow && !expanded)}
          className="px-3 py-2 font-mono text-transcript-meta tracking-mono"
          {...(engineLoader ? { engineLoader } : {})}
          handleRef={replay.handleRef}
          instanceId={instanceId}
          onAttached={replay.onAttached}
          readOnly
          screenReaderMode
        />
      </div>
      {replay.writeError ? (
        <div
          className="flex min-h-7 w-full items-center justify-center border-line-strong border-t bg-terminal-bg px-3 font-mono text-micro text-subtle"
          role="status"
        >
          {terminalReplayFailedCopy()}
        </div>
      ) : null}
      {overflow ? (
        <button
          aria-expanded={expanded}
          className={cn(
            "flex min-h-7 w-full items-center justify-center border-line-strong border-t border-dashed",
            "bg-terminal-bg px-3 font-mono text-micro text-subtle",
            "transition-colors duration-base ease-out hover:text-fg",
            "focus-visible:shadow-focus-ring focus-visible:outline-none"
          )}
          data-testid="session-agent-reported-more"
          onClick={() => setExpanded(value => !value)}
          type="button"
        >
          {expanded ? "show fewer lines" : `show ${hiddenCount} more lines`}
        </button>
      ) : null}
      {terminal.truncated === true ? (
        <div
          className="flex min-h-7 w-full items-center justify-center border-line-strong border-t border-dashed bg-terminal-bg px-3 font-mono text-micro text-subtle"
          data-testid="session-agent-reported-truncated"
        >
          {`truncated · ${BYTE_FORMATTER.format(terminal.total_bytes)} bytes`}
        </div>
      ) : null}
    </div>
  );
}

function reportedTerminalInstanceId(data: AgentEventPayload): string {
  const fields = [data.session_id ?? "", data.turn_id ?? "", data.reported_terminal?.id ?? ""];
  return `agent-reported:${fields.map(value => `${value.length}:${value}`).join(":")}`;
}
