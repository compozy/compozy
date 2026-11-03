/** Sessions and agents for the #1191 declutter review: one project, a folder, a thread. */
import type { AgentAvatarAppearance } from "@compozy/ui";

import type { AgentIdentitySource } from "@/systems/agent/lib/agent-identity";
import type { SessionLinkedWorkSummary } from "@/systems/session/lib/session-linked-work";
import { primarySessionFixture, sessionRuntime } from "@/systems/session/mocks/fixtures";
import type { SessionPayload } from "@/systems/session/types";

const NOW = Date.now();
const ago = (minutes: number) => new Date(NOW - minutes * 60_000).toISOString();

/** The neutral swatch (`PROFILE_IDENTITY_SWATCHES` Gray): a built-in reads as part of the app. */
export const BUILTIN_GRAY = "#8a8f98";

export type IconBackground = "plate" | "none";

/** Built-in on-demand agents and the icon each would ship with. */
export const BUILTIN_ICONS = {
  general: { label: "General", icon: "sparkles" },
  code_implementer: { label: "Code implementer", icon: "code-xml" },
  orchestrator: { label: "Orchestrator", icon: "workflow" },
  reviewer: { label: "Reviewer", icon: "scan-eye" },
  spec_conductor: { label: "Spec conductor", icon: "list-checks" },
  "open-design-designer": { label: "Open Design designer", icon: "palette" },
} as const;

export type BuiltinAgent = keyof typeof BUILTIN_ICONS;

export function builtinAppearance(
  agent: BuiltinAgent,
  background: IconBackground
): AgentAvatarAppearance {
  return {
    avatar: "icon",
    icon: BUILTIN_ICONS[agent].icon,
    color: BUILTIN_GRAY,
    ...(background === "none" ? { background } : {}),
  };
}

/** Agents people made: they keep their generated faces. */
const USER_AGENTS: AgentIdentitySource[] = [
  {
    name: "release-captain",
    label: "Release captain",
    resolved_appearance: { version: 1, avatar: "generated", seed: "release-captain.2" },
  },
  {
    name: "docs-writer",
    label: "Docs writer",
    resolved_appearance: {
      version: 1,
      avatar: "generated",
      seed: "docs-writer.4",
      color: "#e8b04a",
    },
  },
];

/** The catalog the rows resolve faces from: built-ins on a gray plate, bare, or today's faces. */
export function agentCatalog(background: IconBackground | "today"): readonly AgentIdentitySource[] {
  const builtins = (Object.keys(BUILTIN_ICONS) as BuiltinAgent[]).map(name => ({
    name,
    label: BUILTIN_ICONS[name].label,
    resolved_appearance:
      background === "today"
        ? { version: 1, avatar: "generated", seed: name }
        : { version: 1, ...builtinAppearance(name, background) },
  }));
  return [...builtins, ...USER_AGENTS];
}

function session(
  id: string,
  overrides: Partial<SessionPayload> & { provider?: string; minutes?: number }
): SessionPayload {
  const { provider = "claude", minutes = 5, ...rest } = overrides;
  return {
    ...primarySessionFixture,
    id,
    runtime: sessionRuntime(provider),
    state: "active",
    badge: "idle",
    archived_at: null,
    pull_request: null,
    linked_work: null,
    updated_at: ago(minutes),
    ...rest,
  };
}

function links(total: number, needsYou = 0): SessionLinkedWorkSummary {
  return { total, needs_you: needsYou, pull_requests: total, lead: null };
}

export const SESSIONS = {
  captain: session("d-captain", {
    name: "Release captain",
    agent_name: "release-captain",
    badge: "waiting",
    waiting_on: "monitor",
    minutes: 2,
  }),
  question: session("d-question", {
    name: "Which token scope does the GraphQL provider need?",
    agent_name: "general",
    badge: "waiting-for-input",
    provider: "codex",
    minutes: 6,
  }),
  chip: session("d-chip", {
    name: "Topbar PR chip — web slice",
    agent_name: "code_implementer",
    badge: "running",
    minutes: 0,
    linked_work: links(1),
  }),
  orchestrator: session("d-orchestrator", {
    name: "Orquestrar 3 sessions para deixar PRs verdes",
    agent_name: "orchestrator",
    badge: "waiting",
    minutes: 3,
    linked_work: links(3, 1),
    subagent_summary: { total: 4, live: 1, failed: 1, attention: 1, most_urgent: "attention" },
  }),
  childRunning: session("d-child-1", {
    name: "Fix #1191 declutter session list",
    agent_name: "code_implementer",
    badge: "running",
    minutes: 1,
    linked_work: links(1),
  }),
  childApproval: session("d-child-2", {
    name: "Fix #1188 terminal resize",
    agent_name: "code_implementer",
    badge: "waiting-for-auth",
    provider: "codex",
    minutes: 4,
    linked_work: links(1, 1),
  }),
  childDone: session("d-child-3", {
    name: "Fix #1187 stale model list",
    agent_name: "code_implementer",
    badge: "done",
    minutes: 22,
    linked_work: links(1),
  }),
  batchRunning: session("d-batch-1", {
    name: "Fix #1176 kanban drag preview",
    agent_name: "code_implementer",
    badge: "running",
    minutes: 0,
    linked_work: links(1),
  }),
  batchIdle: session("d-batch-2", {
    name: "Fix #1179 composer paste",
    agent_name: "code_implementer",
    badge: "idle",
    minutes: 35,
    linked_work: links(1),
  }),
  batchFailed: session("d-batch-3", {
    name: "Fix #1180 attachment rebase",
    agent_name: "code_implementer",
    badge: "failed",
    provider: "codex",
    minutes: 48,
    linked_work: links(1),
  }),
  batchDone: session("d-batch-4", {
    name: "Fix #1182 palette focus",
    agent_name: "code_implementer",
    badge: "done",
    minutes: 70,
    linked_work: links(2),
  }),
  board: session("d-board", {
    name: "Kanban board directions",
    agent_name: "open-design-designer",
    badge: "done",
    minutes: 95,
  }),
  review: session("d-review", {
    name: "Review PR #1174",
    agent_name: "reviewer",
    badge: "idle",
    minutes: 140,
  }),
  docs: session("d-docs", {
    name: "Agent appearance docs",
    agent_name: "docs-writer",
    badge: "idle",
    minutes: 260,
  }),
} satisfies Record<string, SessionPayload>;

export const THREAD_CHILDREN = [SESSIONS.childRunning, SESSIONS.childApproval, SESSIONS.childDone];

export const FOLDER_ROWS = [
  SESSIONS.batchRunning,
  SESSIONS.batchIdle,
  SESSIONS.batchFailed,
  SESSIONS.batchDone,
];

/** Sessions a stays-on agent started: today's rows wear its face as a "Started by" mark. */
export const STARTED_BY_CAPTAIN = new Set<string>([SESSIONS.chip.id, SESSIONS.batchRunning.id]);
