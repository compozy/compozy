import { definePrototype, type PrototypeProps } from "@compozy/proto";

import {
  LISTED_ON_DEMAND,
  LISTED_INVALID,
  LISTED_NEEDS_YOU,
  LISTED_PAUSED,
  LISTED_ALWAYS_ON,
  LISTED_SAME_SLUG,
  MANY_ON_DEMAND,
  MANY_ALWAYS_ON,
} from "../always-on-agents-shared/agents-fixtures";
import { AgentsPage, type AgentsPageProps } from "../always-on-agents-shared/agents-page";
import { ALWAYS_ON_CROWD } from "./team-crowds";

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
    { id: "no-always-on", label: "No always-on agents yet" },
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
    { id: "columns", label: "Two columns · The team" },
    { id: "columns-many", label: "Two columns · 100 agents, Load more" },
    { id: "columns-full-always-on", label: "Two columns · Always-on full, On-demand nearly empty" },
    { id: "columns-full-on-demand", label: "Two columns · On-demand full, Always-on nearly empty" },
    { id: "columns-no-always-on", label: "Two columns · No always-on agents yet" },
    { id: "columns-filtered", label: "Two columns · Search empties one column" },
    { id: "columns-narrow", label: "Two columns · Narrow window, one column" },
  ],
});

const TEAM = { onDemand: LISTED_ON_DEMAND, alwaysOn: LISTED_ALWAYS_ON };
const CROWD = { onDemand: MANY_ON_DEMAND, alwaysOn: MANY_ALWAYS_ON };

/** Matches Orchestrator and nobody on demand. */
const ALWAYS_ON_ONLY_QUERY = "spec task";
const COLUMNS = { layout: "columns" } as const;

const ADDING = "triage";

/** Every state is a set of props on one page; the narrow window adds its own frame below. */
const STATE_PROPS: Record<string, AgentsPageProps> = {
  rows: { ...TEAM, initialView: "rows" },
  "no-always-on": { onDemand: LISTED_ON_DEMAND, alwaysOn: [] },
  "same-slug": { onDemand: LISTED_SAME_SLUG, alwaysOn: LISTED_ALWAYS_ON },
  invalid: { onDemand: LISTED_INVALID, alwaysOn: LISTED_ALWAYS_ON },
  "empty-filter": { ...TEAM, query: "kubernetes" },
  paused: { onDemand: LISTED_ON_DEMAND, alwaysOn: LISTED_PAUSED },
  "needs-you": { onDemand: LISTED_ON_DEMAND, alwaysOn: LISTED_NEEDS_YOU },
  many: CROWD,
  "many-rows": { ...CROWD, initialView: "rows" },
  filtered: {
    ...CROWD,
    chips: [{ field: "project", operator: "is", values: ["billing-api"] }],
  },
  "add-scope": { ...TEAM, adding: { agent: ADDING, mode: "scopes" } },
  "add-scope-none": { ...TEAM, adding: { agent: ADDING, mode: "none" } },
  "add-scope-cap": { ...TEAM, adding: { agent: ADDING, mode: "cap" } },
  "add-scope-auth": { ...TEAM, adding: { agent: ADDING, mode: "auth" } },
  columns: { ...TEAM, ...COLUMNS },
  "columns-many": { ...CROWD, ...COLUMNS },
  "columns-full-always-on": {
    ...COLUMNS,
    onDemand: LISTED_ON_DEMAND.slice(0, 1),
    alwaysOn: ALWAYS_ON_CROWD,
  },
  "columns-full-on-demand": {
    ...COLUMNS,
    onDemand: MANY_ON_DEMAND,
    alwaysOn: LISTED_ALWAYS_ON.slice(0, 1),
  },
  "columns-no-always-on": { ...COLUMNS, onDemand: LISTED_ON_DEMAND, alwaysOn: [] },
  "columns-filtered": { ...TEAM, ...COLUMNS, query: ALWAYS_ON_ONLY_QUERY },
};

export default function AlwaysOnAgentsListDraft({ state }: PrototypeProps) {
  if (state === "columns-narrow") {
    return (
      <div className="mx-auto w-180 max-w-full border-x border-line">
        <AgentsPage {...CROWD} {...COLUMNS} />
      </div>
    );
  }
  return <AgentsPage {...(STATE_PROPS[state] ?? TEAM)} />;
}
