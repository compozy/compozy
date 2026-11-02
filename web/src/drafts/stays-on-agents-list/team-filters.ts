/** The Agents page's filters: what the quick tabs, the search box and the filter menu each match. */
import type { Filter, FilterFieldsConfig } from "@compozy/ui";

import type { AgentState } from "../stays-on-agents-shared/agent-state";
import type { ListedAgent } from "../stays-on-agents-shared/agents-page";
import { SCOPE_LABEL } from "../stays-on-agents-shared/fixtures";

export type TeamFocus = "all" | "needs-you" | "working" | "paused";

const STATE_ORDER: AgentState[] = ["needs-you", "failed", "working", "paused", "starting"];

/** The state that speaks for an agent across all the places it works. */
export function overallState(agent: ListedAgent): AgentState {
  const states = (agent.places ?? []).map(place => place.state);
  if (agent.kind === "on_demand") return agent.active > 0 ? "working" : "idle";
  return STATE_ORDER.find(state => states.includes(state)) ?? "idle";
}

export function matchesFocus(agent: ListedAgent, focus: TeamFocus) {
  return focus === "all" || overallState(agent) === focus;
}

function matchesQuery(agent: ListedAgent, query: string) {
  const needle = query.trim().toLowerCase();
  if (!needle) return true;
  const places = (agent.places ?? []).map(place => SCOPE_LABEL(place.scope));
  return [agent.label, agent.description, ...places, ...agent.scopes].some(text =>
    text.toLowerCase().includes(needle)
  );
}

function worksIn(agent: ListedAgent, project: string) {
  return (
    agent.scopes.includes(project) || (agent.places ?? []).some(place => place.scope === project)
  );
}

export function buildTeamFilterFields(projects: readonly string[]): FilterFieldsConfig<string> {
  return [
    {
      key: "project",
      label: "Where it works",
      type: "select",
      options: projects.map(value => ({ value, label: value })),
    },
    {
      key: "model",
      label: "Model",
      type: "select",
      options: [
        { value: "claude", label: "Claude" },
        { value: "codex", label: "Codex" },
      ],
    },
    {
      key: "owner",
      label: "Made by",
      type: "select",
      options: [
        { value: "you", label: "You" },
        { value: "extension", label: "An extension" },
      ],
    },
  ];
}

function matchesChip(agent: ListedAgent, chip: Filter<string>) {
  const value = chip.values[0];
  if (!value) return true;
  if (chip.field === "project") return worksIn(agent, value);
  if (chip.field === "model") return agent.provider === value;
  if (chip.field === "owner") return value === "extension" ? !!agent.owner : !agent.owner;
  return true;
}

export function matchesFilters(
  agent: ListedAgent,
  query: string,
  chips: readonly Filter<string>[]
) {
  return matchesQuery(agent, query) && chips.every(chip => matchesChip(agent, chip));
}

const ATTENTION: Partial<Record<AgentState, number>> = {
  "needs-you": 0,
  failed: 1,
  working: 2,
  paused: 3,
};

/** People who need you come first, then whoever is busy; the page keeps the fixture order after. */
export function byAttention(left: ListedAgent, right: ListedAgent) {
  return (ATTENTION[overallState(left)] ?? 4) - (ATTENTION[overallState(right)] ?? 4);
}
