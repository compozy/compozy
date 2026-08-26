"use client";

import { Pill, TerminalView, type TerminalEngineLoader } from "@compozy/ui";
import { TerminalSquare } from "lucide-react";

import { useTerminalReplay } from "@/systems/terminal";
import type { AgentEventPayload } from "../types";

export interface SessionAgentReportedBlockProps {
  data: AgentEventPayload;
  /** Replaces the emulator for deterministic playback harnesses. */
  engineLoader?: TerminalEngineLoader;
}

/** A read-only transcript specimen of terminal output supplied by an agent. */
export function SessionAgentReportedBlock({ data, engineLoader }: SessionAgentReportedBlockProps) {
  const terminal = data.reported_terminal;
  const output = data.text ?? "";
  const instanceId = reportedTerminalInstanceId(data);
  const replay = useTerminalReplay(instanceId, output, output.length > 0);

  if (data.origin !== "agent_reported" || !terminal || output.length === 0) return null;

  const title = data.title?.trim() || "Command output";
  return (
    <div
      className="max-w-160 overflow-hidden rounded-md border border-line"
      data-testid={`session-agent-reported-block-${terminal.id}`}
    >
      <div className="flex min-h-8 min-w-0 items-center gap-2 border-line border-b bg-canvas px-2.5">
        <TerminalSquare aria-hidden="true" className="size-deck-glyph flex-none text-subtle" />
        <span className="min-w-0 truncate font-semibold text-eyebrow text-fg">{title}</span>
        <div className="ml-auto flex-none">
          <Pill mono size="xs" tone="neutral">
            reported by agent
          </Pill>
        </div>
      </div>
      <div className="flex max-h-55 flex-col bg-terminal-bg">
        <TerminalView
          aria-label="Command output reported by the agent"
          className="px-3 py-2 font-mono text-transcript-meta tracking-mono"
          {...(engineLoader ? { engineLoader } : {})}
          handleRef={replay.handleRef}
          instanceId={instanceId}
          onAttached={replay.onAttached}
          readOnly
          screenReaderMode
        />
      </div>
    </div>
  );
}

function reportedTerminalInstanceId(data: AgentEventPayload): string {
  const fields = [data.session_id ?? "", data.turn_id ?? "", data.reported_terminal?.id ?? ""];
  return `agent-reported:${fields.map(value => `${value.length}:${value}`).join(":")}`;
}
