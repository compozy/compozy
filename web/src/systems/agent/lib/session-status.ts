import type { PillTone } from "@agh/ui";

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
  tone: PillTone;
}

const ACTIVE_STATUS: AgentSessionStatus = { kind: "active", label: "ACTIVE", tone: "success" };
const RUNNING_STATUS: AgentSessionStatus = { kind: "running", label: "RUNNING", tone: "info" };
const STARTING_STATUS: AgentSessionStatus = {
  kind: "starting",
  label: "STARTING",
  tone: "warning",
};
const STOPPING_STATUS: AgentSessionStatus = {
  kind: "stopping",
  label: "STOPPING",
  tone: "warning",
};
const FAILED_STATUS: AgentSessionStatus = { kind: "failed", label: "FAILED", tone: "danger" };
const DONE_STATUS: AgentSessionStatus = { kind: "done", label: "DONE", tone: "neutral" };
const HUNG_STATUS: AgentSessionStatus = { kind: "hung", label: "HUNG", tone: "warning" };
const UNHEALTHY_STATUS: AgentSessionStatus = {
  kind: "unhealthy",
  label: "UNHEALTHY",
  tone: "warning",
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
