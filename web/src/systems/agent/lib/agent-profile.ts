/** What the agent's home derives from its definition, its places and its activity. */
import type { AgentAvatarState } from "@compozy/ui";

import { formatRelativeClock } from "./agent-time";
import type { AgentTeamPlace } from "./agent-team";
import type { AgentActivityPayload, AgentPayload } from "../types";

export const AGENT_PROFILE_PANELS = ["model", "tools"] as const;
export type AgentProfilePanel = (typeof AGENT_PROFILE_PANELS)[number];

export const AGENT_EDITOR_PAGES = ["what-it-does", "check-ins", "memory", "where"] as const;
export type AgentEditorPage = (typeof AGENT_EDITOR_PAGES)[number];

export function agentCanResume(reason: AgentTeamPlace["pauseReason"]) {
  return reason !== "definition_missing" && reason !== "definition_not_stays_on";
}

/** The freshest check-in across places, in plain words. */
export function agentLastCheckIn(places: readonly AgentTeamPlace[], now = Date.now()) {
  const stamps = places.flatMap(place => {
    const at = Date.parse(place.instance.last_wake_at ?? "");
    return Number.isNaN(at) ? [] : [at];
  });
  if (stamps.length === 0) return null;
  return formatRelativeClock(new Date(Math.max(...stamps)).toISOString(), now);
}

export function agentHeadline(input: {
  staysOn: boolean;
  active: number;
  places: readonly AgentTeamPlace[];
  state: AgentAvatarState;
}) {
  const { staysOn, active, places, state } = input;
  if (!staysOn) return state === "working" ? `${active} running now` : "Starts when you ask";
  if (places.length === 0) return "Waiting for its first project";
  const place = places.find(entry => entry.state === state);
  const where = place?.label ?? "";
  if (state === "needs-you") return `${place?.now ?? "Needs you"} in ${where}`;
  if (state === "working") return `${place?.now ?? "Working"} in ${where}`;
  if (state === "paused") {
    const stopped = places.filter(entry => entry.state === "paused");
    return stopped.length > 1 ? `Paused in ${stopped.length} projects` : `Paused in ${where}`;
  }
  return `Resting in ${places.length} ${places.length === 1 ? "project" : "projects"}`;
}

const PLACE_ATTENTION: Partial<Record<AgentAvatarState, number>> = {
  "needs-you": 0,
  failed: 1,
  working: 2,
  paused: 3,
};

export function agentPlacesByAttention(places: readonly AgentTeamPlace[]) {
  return [...places].sort(
    (left, right) => (PLACE_ATTENTION[left.state] ?? 4) - (PLACE_ATTENTION[right.state] ?? 4)
  );
}

export function agentPlaceStatus(place: AgentTeamPlace, short: Record<string, string>) {
  if (place.pauseReason) return short[place.pauseReason] ?? "Paused";
  if (place.state === "paused") return "You paused it";
  if (place.now) return place.now;
  return place.state === "working" ? "Working" : "Resting until its next check-in";
}

export function agentPlaceTimes(place: AgentTeamPlace, now = Date.now()) {
  const last = formatRelativeClock(place.instance.last_wake_at, now);
  if (place.state === "paused") return `Last checked in ${last ?? "never"}`;
  const next = formatRelativeClock(place.instance.next_wake_at, now);
  return `${last ? `Checked in ${last}` : "No check-ins yet"}${next ? ` · next ${next}` : ""}`;
}

function dayLabel(day: string) {
  const date = new Date(`${day}T00:00:00Z`);
  if (Number.isNaN(date.getTime())) return day;
  return date.toLocaleDateString("en-US", { day: "numeric", month: "short", timeZone: "UTC" });
}

export interface AgentActivityView {
  days: { date: string; quiet: number; reported: number; needed: number }[];
  reports: number;
  perDay: number;
  empty: boolean;
}

export function agentActivityView(
  activity: AgentActivityPayload,
  window: number
): AgentActivityView {
  const rows = activity.reports_by_day.slice(-window);
  const days = rows.map(row => ({
    date: dayLabel(row.day),
    quiet: row.quiet,
    reported: row.told,
    needed: row.needed_you,
  }));
  const reports = rows.reduce((total, row) => total + row.told + row.needed_you, 0);
  const checks = rows.reduce((total, row) => total + row.quiet, 0);
  return {
    days,
    reports,
    perDay: rows.length > 0 ? Math.round(reports / rows.length) : 0,
    empty: reports + checks === 0,
  };
}

function compactTokens(tokens: number) {
  if (tokens >= 1_000_000) return `${(tokens / 1_000_000).toFixed(1)}M tokens`;
  if (tokens >= 1_000) return `${Math.round(tokens / 1_000)}K tokens`;
  return `${tokens} tokens`;
}

export function agentCostView(activity: AgentActivityPayload) {
  const rows = activity.cost_by_project;
  const usd = rows.reduce((total, row) => total + row.usd_estimate, 0);
  const tokens = rows.reduce((total, row) => total + row.tokens, 0);
  return {
    cost: `≈ $${usd.toFixed(2)}`,
    tokens: compactTokens(tokens),
    projects: rows.map(row => ({
      id: row.workspace_id || row.scope,
      label: row.scope === "@user" ? "Global" : row.scope,
      value: usd > 0 ? row.usd_estimate / usd : 0,
      formattedValue: `$${row.usd_estimate.toFixed(2)}`,
    })),
  };
}

export function agentModelSummary(agent: Pick<AgentPayload, "model" | "provider">) {
  return agent.model || "Uses your default";
}

export function agentToolsSummary(agent: Pick<AgentPayload, "tools" | "toolsets">) {
  const count = (agent.tools?.length ?? 0) + (agent.toolsets?.length ?? 0);
  if (count === 0) return "Everything it's allowed";
  return `${count} ${count === 1 ? "tool" : "tools"}`;
}

/** The first line of what it does, for the setup list. */
export function agentBehaviorSummary(agent: Pick<AgentPayload, "prompt">) {
  return (
    agent.prompt
      .split("\n")
      .find(line => line.trim() !== "")
      ?.trim() ?? "Nothing written yet"
  );
}
