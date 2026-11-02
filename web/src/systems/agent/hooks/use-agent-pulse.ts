import { useQuery } from "@tanstack/react-query";

import { useProfileReadScope } from "@/systems/profiles";

import { agentPulse, type AgentTeamMember } from "../lib/agent-team";
import { agentActivityOptions } from "../lib/query-options";

const PULSE_READ_DAYS = 14;

/** The last two weeks of one agent; stays-on members only, so the list reads a handful. */
export function useAgentPulse(member: AgentTeamMember) {
  const { destination } = useProfileReadScope();
  const { data, isSuccess } = useQuery(
    agentActivityOptions(
      member.name,
      PULSE_READ_DAYS,
      null,
      destination,
      member.kind === "stays_on" && member.agent !== null
    )
  );
  return { pulse: agentPulse(data), known: isSuccess && data !== null };
}
