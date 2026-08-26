/**
 * The transcript's adapter for daemon-authored call and mailbox turns.
 *
 * Keeps the navigation and the text extraction here so the agent-comms component
 * stays presentational and testable without a router.
 */
import { useNavigate } from "@tanstack/react-router";

import { AgentSyntheticTurn, type SyntheticTurn } from "@/systems/agent-comms";

import { isRecord, stringField } from "./timeline-message-parts";

/** The turn's own words, joined across whatever text parts it was written as. */
function turnText(content: unknown): string {
  if (typeof content === "string") return content;
  if (!Array.isArray(content)) return "";
  const lines: string[] = [];
  for (const part of content) {
    if (!isRecord(part) || stringField(part, "type") !== "text") continue;
    lines.push(stringField(part, "text") ?? "");
  }
  return lines.join("\n").trim();
}

export function SessionSyntheticTurn({
  synthetic,
  content,
  timestamp,
}: {
  synthetic: SyntheticTurn;
  content: unknown;
  timestamp: string;
}) {
  const navigate = useNavigate();
  return (
    <AgentSyntheticTurn
      data-testid="session-synthetic-turn"
      onOpenCall={callId => {
        void navigate({ to: "/agents/calls/$callId", params: { callId } });
      }}
      text={turnText(content)}
      timestamp={timestamp}
      turn={synthetic}
    />
  );
}
