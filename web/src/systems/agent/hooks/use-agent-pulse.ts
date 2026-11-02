import { useQueries } from "@tanstack/react-query";

import { useProfileReadScope } from "@/systems/profiles";

import { agentPulse, type AgentTeamMember } from "../lib/agent-team";
import { agentActivityOptions } from "../lib/query-options";
import type { AgentActivityPayload } from "../types";

const PULSE_READ_DAYS = 14;

/** The last two weeks of one agent; stays-on members only, so the list reads a handful. */
export function useAgentPulse(member: AgentTeamMember) {
  const { destination } = useProfileReadScope();
  const reads = useQueries({
    queries: member.scopes.map(scope =>
      agentActivityOptions(
        member.name,
        PULSE_READ_DAYS,
        scope,
        destination,
        member.kind === "stays_on" && member.agent !== null
      )
    ),
  });
  const days = new Map<string, AgentActivityPayload["reports_by_day"][number]>();
  for (const read of reads) {
    for (const day of read.data?.reports_by_day ?? []) {
      const total = days.get(day.day) ?? { day: day.day, quiet: 0, told: 0, needed_you: 0 };
      days.set(day.day, {
        day: day.day,
        quiet: total.quiet + day.quiet,
        told: total.told + day.told,
        needed_you: total.needed_you + day.needed_you,
      });
    }
  }
  const known = reads.length > 0 && reads.every(read => read.isSuccess && read.data !== null);
  const reports_by_day = [...days.values()].sort((a, b) => a.day.localeCompare(b.day));
  return { pulse: known ? agentPulse({ reports_by_day }) : null, known };
}
