import type { StateGlyphState } from "@compozy/ui";

import { isSessionRunning, type SessionPayload } from "@/systems/session";

export type AgentSessionStatusKind =
  | "running"
  | "active"
  | "starting"
  | "stopping"
  | "failed"
  | "done"
  | "hung"
  | "unhealthy";

export interface AgentSessionStatus {
  kind: AgentSessionStatusKind;
  label: string;
  glyph: StateGlyphState;
}

const ACTIVE_STATUS: AgentSessionStatus = { kind: "active", label: "Active", glyph: "idle" };
const RUNNING_STATUS: AgentSessionStatus = { kind: "running", label: "Running", glyph: "running" };
const STARTING_STATUS: AgentSessionStatus = {
  kind: "starting",
  label: "Starting",
  glyph: "queued",
};
const STOPPING_STATUS: AgentSessionStatus = {
  kind: "stopping",
  label: "Stopping",
  glyph: "stopped",
};
const FAILED_STATUS: AgentSessionStatus = { kind: "failed", label: "Failed", glyph: "failed" };
const DONE_STATUS: AgentSessionStatus = { kind: "done", label: "Done", glyph: "done" };
const HUNG_STATUS: AgentSessionStatus = { kind: "hung", label: "Hung", glyph: "failed" };
const UNHEALTHY_STATUS: AgentSessionStatus = {
  kind: "unhealthy",
  label: "Unhealthy",
  glyph: "failed",
};

export function isAgentSessionFailure(session: SessionPayload): boolean {
  return (
    session.state === "stopped" &&
    (Boolean(session.failure) ||
      session.stop_reason === "agent_crashed" ||
      session.stop_reason === "error")
  );
}

export function getAgentSessionStatus(session: SessionPayload): AgentSessionStatus {
  if (isSessionRunning(session)) {
    return RUNNING_STATUS;
  }
  if (session.badge === "hung") {
    return HUNG_STATUS;
  }
  if (session.badge === "unhealthy") {
    return UNHEALTHY_STATUS;
  }

  switch (session.state) {
    case "active":
      return ACTIVE_STATUS;
    case "starting":
      return STARTING_STATUS;
    case "stopping":
      return STOPPING_STATUS;
    case "stopped":
      if (isAgentSessionFailure(session)) {
        return FAILED_STATUS;
      }
      return DONE_STATUS;
  }
}
