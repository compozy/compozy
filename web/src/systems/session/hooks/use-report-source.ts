import { useQuery } from "@tanstack/react-query";

import type { AgentInstancePayload } from "@/systems/agent";
import { useProfileReadScope } from "@/systems/profiles";

import { sessionScopedDetailOptions } from "../lib/query-options";
import { reportAgent } from "../lib/session-report-identity";

/** Reads the session a report points at; a deleted one keeps the sentence honest. */
export function useReportSource(
  sessionId: string | undefined,
  instances: readonly AgentInstancePayload[]
) {
  const { params } = useProfileReadScope();
  const { data: session } = useQuery(
    sessionScopedDetailOptions(sessionId ?? "", params, { enabled: Boolean(sessionId) })
  );
  const name = session?.agent_name ?? "";
  const instance = instances.find(candidate => candidate.agent_name === name);
  return {
    agent: reportAgent(instance, name || "An agent"),
    session: session?.name?.trim() || "another session",
  };
}
