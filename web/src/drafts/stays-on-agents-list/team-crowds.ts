/** A team where most agents stay on, to see the two-column variation lopsided the other way. */
import { MANY_ON_DEMAND, MANY_STAYS_ON } from "../stays-on-agents-shared/agents-fixtures";
import type { ListedAgent } from "../stays-on-agents-shared/agents-page";

function asStaysOn(agent: ListedAgent, index: number): ListedAgent {
  return {
    ...agent,
    kind: "stays_on",
    recap: `${(index * 3) % 40} reports in 2 weeks`,
    places: agent.scopes.map((scope, placeIndex) => ({
      scope,
      state: index % 6 === 0 && placeIndex === 0 ? ("working" as const) : ("idle" as const),
      now: index % 6 === 0 && placeIndex === 0 ? "Going through this week's changes" : undefined,
    })),
  };
}

export const STAYS_ON_CROWD: readonly ListedAgent[] = [
  ...MANY_STAYS_ON,
  ...MANY_ON_DEMAND.slice(3, 43).map(asStaysOn),
];
