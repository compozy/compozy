import { definePrototype, type PrototypeProps } from "@compozy/proto";

import {
  LISTED_ON_DEMAND,
  LISTED_INVALID,
  LISTED_NEEDS_YOU,
  LISTED_PAUSED,
  LISTED_STAYS_ON,
  LISTED_SAME_SLUG,
  MANY_ON_DEMAND,
  MANY_STAYS_ON,
} from "../stays-on-agents-shared/agents-fixtures";
import { AgentsPage } from "../stays-on-agents-shared/agents-page";

export const meta = definePrototype({
  title: "Agents list",
  spec: "always-on-agent",
  surface: "S2",
  route: "/agents",
  states: [
    { id: "cards", label: "The team" },
    { id: "rows", label: "Rows" },
    { id: "needs-you", label: "Someone needs you" },
    { id: "paused", label: "Paused, and why" },
    { id: "no-stays-on", label: "Nobody stays on yet" },
    { id: "same-slug", label: "Same name in two projects" },
    { id: "invalid", label: "An agent that can't start" },
    { id: "many", label: "100 agents, 50 projects" },
    { id: "many-rows", label: "100 agents in rows" },
    { id: "filtered", label: "Filtered by project" },
    { id: "empty-filter", label: "Nothing matches" },
    { id: "add-scope", label: "S16 · + picks a project" },
    { id: "add-scope-none", label: "S16 · Nowhere left to add" },
    { id: "add-scope-cap", label: "S16 · At the limit" },
    { id: "add-scope-auth", label: "S16 · Not signed in" },
  ],
});

const TEAM = { onDemand: LISTED_ON_DEMAND, staysOn: LISTED_STAYS_ON };
const CROWD = { onDemand: MANY_ON_DEMAND, staysOn: MANY_STAYS_ON };

export default function StaysOnAgentsListDraft({ state }: PrototypeProps) {
  switch (state) {
    case "rows":
      return <AgentsPage {...TEAM} initialView="rows" />;
    case "no-stays-on":
      return <AgentsPage onDemand={LISTED_ON_DEMAND} staysOn={[]} />;
    case "same-slug":
      return <AgentsPage onDemand={LISTED_SAME_SLUG} staysOn={LISTED_STAYS_ON} />;
    case "invalid":
      return <AgentsPage onDemand={LISTED_INVALID} staysOn={LISTED_STAYS_ON} />;
    case "empty-filter":
      return <AgentsPage {...TEAM} query="kubernetes" />;
    case "paused":
      return <AgentsPage onDemand={LISTED_ON_DEMAND} staysOn={LISTED_PAUSED} />;
    case "needs-you":
      return <AgentsPage onDemand={LISTED_ON_DEMAND} staysOn={LISTED_NEEDS_YOU} />;
    case "many":
      return <AgentsPage {...CROWD} />;
    case "many-rows":
      return <AgentsPage {...CROWD} initialView="rows" />;
    case "filtered":
      return (
        <AgentsPage
          {...CROWD}
          chips={[{ field: "project", operator: "is", values: ["billing-api"] }]}
        />
      );
    case "add-scope":
    case "add-scope-none":
    case "add-scope-cap":
    case "add-scope-auth": {
      const mode = (
        {
          "add-scope": "scopes",
          "add-scope-none": "none",
          "add-scope-cap": "cap",
          "add-scope-auth": "auth",
        } as const
      )[state];
      return <AgentsPage {...TEAM} adding={{ agent: "triage", mode }} />;
    }
    default:
      return <AgentsPage {...TEAM} />;
  }
}
