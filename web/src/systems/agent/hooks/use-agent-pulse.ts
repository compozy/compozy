import { useQueries } from "@tanstack/react-query";

import { useProfileReadScope } from "@/systems/profiles";

import { agentPulse, type AgentTeamMember } from "../lib/agent-team";
import { agentActivityBatchOptions } from "../lib/query-options";
import type { AgentActivityPayload } from "../types";

const PULSE_READ_DAYS = 14;
const MAX_BATCH_ITEMS = 100;
const MAX_BATCH_SCOPES = 5000;

export interface AgentPulseRead {
  pulse: ReturnType<typeof agentPulse>;
  known: boolean;
}

type PulseSelection = { key: string; name: string; workspaces: string[] };

function pulseBatches(members: readonly AgentTeamMember[]) {
  const batches: PulseSelection[][] = [];
  let scopeCount = 0;
  for (const member of members) {
    const scopes = [...new Set(member.scopes)].sort();
    for (let offset = 0; offset < scopes.length; offset += MAX_BATCH_ITEMS) {
      const workspaces = scopes.slice(offset, offset + MAX_BATCH_ITEMS);
      let batch = batches[batches.length - 1];
      if (
        !batch ||
        batch.length === MAX_BATCH_ITEMS ||
        scopeCount + workspaces.length > MAX_BATCH_SCOPES
      ) {
        batch = [];
        batches.push(batch);
        scopeCount = 0;
      }
      batch.push({ key: member.key, name: member.name, workspaces });
      scopeCount += workspaces.length;
    }
  }
  return batches;
}

export function useAgentPulses(members: readonly AgentTeamMember[]) {
  const { destination } = useProfileReadScope();
  const selected = members
    .filter(member => member.kind === "always_on" && member.agent !== null)
    .sort((a, b) => a.key.localeCompare(b.key));
  const batches = pulseBatches(selected);
  const reads = useQueries({
    queries: batches.map(batch =>
      agentActivityBatchOptions(
        batch.map(({ name, workspaces }) => ({ name, workspaces })),
        PULSE_READ_DAYS,
        destination
      )
    ),
  });
  const days = new Map<string, AgentActivityPayload["reports_by_day"]>();
  const known = new Map<string, boolean>();
  for (const [batchIndex, batch] of batches.entries()) {
    const read = reads[batchIndex];
    for (const [index, selection] of batch.entries()) {
      const activity = read?.data?.activities[index];
      known.set(
        selection.key,
        (known.get(selection.key) ?? true) && !!read?.isSuccess && !!activity
      );
      days.set(selection.key, [
        ...(days.get(selection.key) ?? []),
        ...(activity?.reports_by_day ?? []),
      ]);
    }
  }
  return new Map(
    selected.map(member => {
      const totals = new Map<string, AgentActivityPayload["reports_by_day"][number]>();
      for (const day of days.get(member.key) ?? []) {
        const prior = totals.get(day.day) ?? { day: day.day, quiet: 0, told: 0, needed_you: 0 };
        totals.set(day.day, {
          day: day.day,
          quiet: prior.quiet + day.quiet,
          told: prior.told + day.told,
          needed_you: prior.needed_you + day.needed_you,
        });
      }
      const complete = known.get(member.key) ?? false;
      return [
        member.key,
        {
          known: complete,
          pulse: complete
            ? agentPulse({
                reports_by_day: [...totals.values()].sort((a, b) => a.day.localeCompare(b.day)),
              })
            : null,
        },
      ];
    })
  );
}
