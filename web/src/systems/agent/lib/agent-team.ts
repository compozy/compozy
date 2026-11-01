/** The Agents page's view model: one member per definition, each with the places it works. */
import type {
  AgentAvatarAppearance,
  AgentAvatarState,
  Filter,
  FilterFieldsConfig,
  StatusDotTone,
} from "@compozy/ui";

import type {
  AgentActivityPayload,
  AgentInstancePauseReason,
  AgentInstancePayload,
  AgentPayload,
  AgentTeamItem,
} from "../types";
import { agentAppearanceFromPayload } from "./agent-appearance";
import { formatAgentOwner, isAgentReadOnly } from "./agent-ownership";
import { GLOBAL_SCOPE, agentInstanceAvatarState, agentScopeLabel } from "./agent-instance";
import { formatRelativeClock } from "./agent-time";

export type AgentTeamFocus = "all" | "needs-you" | "working" | "paused";

export interface AgentTeamPlace {
  instance: AgentInstancePayload;
  /** A workspace id, or `@user`. */
  scope: string;
  label: string;
  state: AgentAvatarState;
  pauseReason?: Exclude<AgentInstancePauseReason, "">;
  now?: string;
}

export interface AgentTeamMember {
  key: string;
  name: string;
  label: string;
  description: string;
  kind: "stays_on" | "on_demand";
  provider: string | null;
  appearance: AgentAvatarAppearance;
  agent: AgentPayload | null;
  places: readonly AgentTeamPlace[];
  /** Scope ids the definition resolves in. */
  scopes: readonly string[];
  /** Separates two definitions that share a name. */
  origin?: string;
  owner?: string;
  invalid: boolean;
  active: number;
  /** Every session it has run; absent when metrics didn't answer. */
  sessions?: number;
  lastActivityAt?: string | null;
}

export const AGENT_STATE_DOT: Partial<Record<AgentAvatarState, StatusDotTone>> = {
  working: "success",
  "needs-you": "brand",
  failed: "destructive",
  paused: "faint",
  idle: "faint",
  starting: "faint",
};

/** Why it stopped, the way you'd say it to a colleague. */
export const AGENT_PAUSE_PLAIN: Record<Exclude<AgentInstancePauseReason, "">, string> = {
  manual: "You paused it.",
  fatigue: "It stopped itself after 10 check-ins in a row went wrong.",
  definition_not_stays_on: "It was changed to on demand, so it can't stay on here anymore.",
  definition_missing: "Its setup was deleted, so it can't come back. What it remembers is kept.",
};

/** The same reason, short enough for a card. */
export const AGENT_PAUSE_SHORT: Record<Exclude<AgentInstancePauseReason, "">, string> = {
  manual: "You paused it",
  fatigue: "Stopped after 10 failed check-ins",
  definition_not_stays_on: "Now on demand, so it can't stay on",
  definition_missing: "Its setup was deleted",
};

const STATE_ORDER: readonly AgentAvatarState[] = [
  "needs-you",
  "failed",
  "working",
  "paused",
  "starting",
];

/** The state that speaks for an agent across all the places it works. */
export function agentOverallState(
  member: Pick<AgentTeamMember, "active" | "kind" | "places">
): AgentAvatarState {
  if (member.kind === "on_demand" && member.places.length === 0) {
    return member.active > 0 ? "working" : "idle";
  }
  const states = member.places.map(place => place.state);
  return STATE_ORDER.find(state => states.includes(state)) ?? "idle";
}

export function agentPlaceFromInstance(
  instance: AgentInstancePayload,
  names: ReadonlyMap<string, string>
): AgentTeamPlace {
  const scope = instance.workspace_id || GLOBAL_SCOPE;
  return {
    instance,
    scope,
    label:
      scope === GLOBAL_SCOPE ? "Global" : (names.get(scope) ?? agentScopeLabel(instance.scope)),
    state: agentInstanceAvatarState(instance),
    ...(instance.pause_reason ? { pauseReason: instance.pause_reason } : {}),
    ...(instance.now ? { now: instance.now } : {}),
  };
}

function memberFromItem(
  item: AgentTeamItem,
  places: readonly AgentTeamPlace[],
  names: ReadonlyMap<string, string>,
  shared: boolean
): AgentTeamMember {
  const agent = item.agent;
  const firstScope = item.scopes[0];
  return {
    key: `${item.name}\u0000${item.definition_digest}`,
    name: item.name,
    label: item.label,
    description: agent.description ?? "",
    kind: item.kind === "stays_on" ? "stays_on" : "on_demand",
    provider: agent.provider || null,
    appearance: agentAppearanceFromPayload(agent.resolved_appearance, item.name),
    agent,
    places,
    scopes: item.scopes,
    ...(shared && firstScope
      ? { origin: firstScope === GLOBAL_SCOPE ? "Global" : (names.get(firstScope) ?? firstScope) }
      : {}),
    ...(isAgentReadOnly(agent) ? { owner: formatAgentOwner(agent) } : {}),
    invalid: (agent.diagnostics?.length ?? 0) > 0,
    active: item.active,
    ...(item.sessions
      ? { sessions: item.sessions.total, lastActivityAt: item.sessions.last_activity_at }
      : {}),
  };
}

/** An instance whose definition is gone: the card stays, so its reason stays visible. */
function memberFromOrphan(
  name: string,
  instances: readonly AgentInstancePayload[],
  names: ReadonlyMap<string, string>
): AgentTeamMember {
  const first = instances[0]!;
  return {
    key: `${name}\u0000orphan`,
    name,
    label: first.label,
    description: "",
    kind: "stays_on",
    provider: first.provider ?? null,
    appearance: agentAppearanceFromPayload(first.resolved_appearance, name),
    agent: null,
    places: instances.map(instance => agentPlaceFromInstance(instance, names)),
    scopes: [],
    invalid: false,
    active: 0,
  };
}

/** Joins the grouped catalog with live instances. Same slug, different digest: separate cards. */
export function buildAgentTeam(input: {
  items: readonly AgentTeamItem[];
  instances: readonly AgentInstancePayload[];
  workspaces: readonly { id: string; name: string }[];
}): AgentTeamMember[] {
  const names = new Map(input.workspaces.map(workspace => [workspace.id, workspace.name]));
  const definitions = new Map<string, number>();
  for (const item of input.items) {
    definitions.set(item.name, (definitions.get(item.name) ?? 0) + 1);
  }
  const placed = new Set<string>();
  const members = input.items.map(item => {
    const shared = (definitions.get(item.name) ?? 0) > 1;
    const own = input.instances.filter(instance => {
      if (instance.agent_name !== item.name) return false;
      return !shared || item.scopes.includes(instance.workspace_id || GLOBAL_SCOPE);
    });
    for (const instance of own) placed.add(instance.id);
    const places = own.map(instance => agentPlaceFromInstance(instance, names));
    return memberFromItem(item, places, names, shared);
  });
  const orphans = new Map<string, AgentInstancePayload[]>();
  for (const instance of input.instances) {
    if (placed.has(instance.id)) continue;
    orphans.set(instance.agent_name, [...(orphans.get(instance.agent_name) ?? []), instance]);
  }
  for (const [name, instances] of orphans) {
    members.push(memberFromOrphan(name, instances, names));
  }
  return members;
}

const ATTENTION: Partial<Record<AgentAvatarState, number>> = {
  "needs-you": 0,
  failed: 1,
  working: 2,
  paused: 3,
};

/** People who need you come first, then whoever is busy. */
export function agentsByAttention(left: AgentTeamMember, right: AgentTeamMember) {
  return (
    (ATTENTION[agentOverallState(left)] ?? 4) - (ATTENTION[agentOverallState(right)] ?? 4) ||
    left.label.localeCompare(right.label)
  );
}

/** A definition that stopped being stays-on keeps its section while it has places to explain. */
export function agentStaysOn(member: AgentTeamMember) {
  return member.kind === "stays_on" || member.places.length > 0;
}

export function matchesAgentFocus(member: AgentTeamMember, focus: AgentTeamFocus) {
  return focus === "all" || agentOverallState(member) === focus;
}

function matchesQuery(member: AgentTeamMember, query: string, names: ReadonlyMap<string, string>) {
  const needle = query.trim().toLowerCase();
  if (!needle) return true;
  const places = member.places.map(place => place.label);
  const scopes = member.scopes.map(scope => names.get(scope) ?? scope);
  return [member.label, member.name, member.description, ...places, ...scopes].some(text =>
    text.toLowerCase().includes(needle)
  );
}

function worksIn(member: AgentTeamMember, project: string) {
  return member.scopes.includes(project) || member.places.some(place => place.scope === project);
}

function matchesChip(member: AgentTeamMember, chip: Filter<string>) {
  const value = chip.values[0];
  if (!value) return true;
  if (chip.field === "project") return worksIn(member, value);
  if (chip.field === "model") return member.provider === value;
  if (chip.field === "owner") return value === "extension" ? !!member.owner : !member.owner;
  return true;
}

export function matchesAgentFilters(
  member: AgentTeamMember,
  query: string,
  chips: readonly Filter<string>[],
  workspaces: readonly { id: string; name: string }[]
) {
  const names = new Map(workspaces.map(workspace => [workspace.id, workspace.name]));
  return matchesQuery(member, query, names) && chips.every(chip => matchesChip(member, chip));
}

export function buildAgentTeamFilterFields(
  workspaces: readonly { id: string; name: string }[],
  providers: readonly { id: string; name: string }[]
): FilterFieldsConfig<string> {
  return [
    {
      key: "project",
      label: "Where it works",
      type: "select",
      options: [
        { value: GLOBAL_SCOPE, label: "Global" },
        ...workspaces.map(workspace => ({ value: workspace.id, label: workspace.name })),
      ],
    },
    {
      key: "model",
      label: "Model",
      type: "select",
      options: providers.map(provider => ({ value: provider.id, label: provider.name })),
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

/** What a stays-on teammate is up to, in one plain sentence, with the tone of its dot. */
export function agentNowLine(member: AgentTeamMember, lastCheckIn: string | null) {
  const live = member.places.find(
    place => place.state === "needs-you" || place.state === "working"
  );
  if (live) {
    return { text: `${live.now ?? "Working"} in ${live.label}`, tone: live.state };
  }
  const paused = member.places.find(place => place.state === "paused");
  if (paused) {
    return {
      text: paused.pauseReason ? AGENT_PAUSE_SHORT[paused.pauseReason] : "Paused",
      tone: "paused" as const,
    };
  }
  if (member.places.length === 0) {
    return { text: "Waiting to be added to a project", tone: "idle" as const };
  }
  return {
    text: lastCheckIn ? `Resting. Last check-in ${lastCheckIn}` : "Resting",
    tone: "idle" as const,
  };
}

/** An on-demand agent's footer: running now, else when it last ran, then how much it has. */
export function agentUsageLine(
  member: Pick<AgentTeamMember, "active" | "lastActivityAt" | "sessions">,
  now = Date.now()
) {
  const last = formatRelativeClock(member.lastActivityAt, now);
  const state =
    member.active > 0
      ? `${member.active} running now`
      : member.sessions === 0
        ? "Never used"
        : last
          ? `Last used ${last}`
          : "Starts when you ask";
  if (!member.sessions) return state;
  return `${state} · ${member.sessions} ${member.sessions === 1 ? "session" : "sessions"}`;
}

const PULSE_DAYS = 14;

/** Daily report counts for the card's sparkline, with the recap beside it. */
export function agentPulse(activity: AgentActivityPayload | null | undefined) {
  if (!activity) return null;
  const days = activity.reports_by_day.slice(-PULSE_DAYS);
  const values = days.map(day => day.quiet + day.told + day.needed_you);
  const reports = days.reduce((total, day) => total + day.told + day.needed_you, 0);
  return { values, reports };
}
