/** Realistic fixtures for the Stays-on agents drafts; field names follow `_dx.md`. */
import type { AgentAppearance, AgentState } from "./agent-state";

export type AgentKind = "stays_on" | "on_demand";
export type PauseReason = "manual" | "fatigue" | "definition_not_stays_on" | "definition_missing";

export interface DraftAgent {
  name: string;
  label: string;
  description: string;
  kind: AgentKind;
  provider: string;
  model: string;
  appearance: AgentAppearance;
  /** Extension that owns the definition; its name reads on every read-only row. */
  owner?: string;
  invalid?: boolean;
  scopes: readonly string[];
  active: number;
}

export interface DraftInstance {
  id: string;
  agent: DraftAgent;
  scope: string;
  state: AgentState;
  pauseReason?: PauseReason;
  lastCheck?: string;
  nextCheck?: string;
}

function generated(seed: string): AgentAppearance {
  return { avatar: "generated", seed };
}

export const ORCHESTRATOR: DraftAgent = {
  name: "orchestrator",
  label: "Orchestrator",
  description: "Delegates each spec task to a dedicated worker",
  kind: "stays_on",
  provider: "claude",
  model: "claude-opus-5-5",
  appearance: generated("orchestrator"),
  owner: "spec-cycle",
  scopes: ["compozy", "reloaddev"],
  active: 2,
};

export const TRIAGE: DraftAgent = {
  name: "triage",
  label: "Triage Lead",
  description: "Watches issues and proposes the next step",
  kind: "stays_on",
  provider: "claude",
  model: "claude-opus-5-5",
  appearance: generated("triage"),
  scopes: ["compozy", "advos"],
  active: 2,
};

export const RELEASE: DraftAgent = {
  name: "release-captain",
  label: "Release Captain",
  description: "Keeps the release branch moving and flags what blocks it",
  kind: "stays_on",
  provider: "codex",
  model: "gpt-6-astra",
  appearance: { avatar: "icon", icon: "compass", color: "#2aa8a8" },
  scopes: ["@user"],
  active: 1,
};

export const SCOUT: DraftAgent = {
  name: "scout",
  label: "Scout",
  description: "Reads what shipped overnight and tells you what changed",
  kind: "stays_on",
  provider: "claude",
  model: "claude-sonnet-5-5",
  appearance: { avatar: "emoji", emoji: "🦊", color: "#e8b04a" },
  scopes: [],
  active: 0,
};

export const GENERAL: DraftAgent = {
  name: "general",
  label: "General",
  description: "Compozy's default general-purpose agent",
  kind: "on_demand",
  provider: "claude",
  model: "claude-opus-5-5",
  appearance: generated("general"),
  scopes: ["@user", "advos", "compozy", "reloaddev"],
  active: 2,
};

export const CODE_ENGINEER: DraftAgent = {
  name: "code-engineer",
  label: "Code Engineer",
  description: "Implements one scoped change and opens the pull request",
  kind: "on_demand",
  provider: "codex",
  model: "gpt-6-astra",
  appearance: generated("code-engineer"),
  scopes: ["compozy", "reloaddev"],
  active: 1,
};

export const REVIEWER: DraftAgent = {
  name: "reviewer",
  label: "Reviewer",
  description: "Reads a diff and reports what would break",
  kind: "on_demand",
  provider: "claude",
  model: "claude-opus-5-5",
  appearance: generated("reviewer"),
  scopes: ["compozy"],
  active: 0,
};

export const STAYS_ON_AGENTS = [ORCHESTRATOR, TRIAGE, RELEASE, SCOUT] as const;
export const ON_DEMAND_AGENTS = [GENERAL, CODE_ENGINEER, REVIEWER] as const;

export const WORKSPACES = ["compozy", "reloaddev", "advos"] as const;

export const SCOPE_LABEL = (scope: string) => (scope === "@user" ? "Global" : scope);

export const PAUSE_COPY: Record<PauseReason, string> = {
  manual: "Paused by you.",
  fatigue: "Paused after 10 failed checks in a row.",
  definition_not_stays_on: "Paused because this agent no longer stays on.",
  definition_missing: "Paused because this agent's definition is gone. Its memory is kept.",
};

export const INSTANCES = {
  orchestratorCompozy: {
    id: "ainst-23a26081",
    agent: ORCHESTRATOR,
    scope: "compozy",
    state: "working",
    lastCheck: "12m ago",
    nextCheck: "in 18m",
  },
  triageCompozy: {
    id: "ainst-41c7e0fa",
    agent: TRIAGE,
    scope: "compozy",
    state: "needs-you",
    lastCheck: "4m ago",
    nextCheck: "in 26m",
  },
  orchestratorReloaddev: {
    id: "ainst-77d0b1c2",
    agent: ORCHESTRATOR,
    scope: "reloaddev",
    state: "idle",
    lastCheck: "31m ago",
    nextCheck: "in 29m",
  },
  triageAdvos: {
    id: "ainst-9e4410aa",
    agent: TRIAGE,
    scope: "advos",
    state: "paused",
    pauseReason: "manual",
    lastCheck: "2h ago",
  },
  triageAdvosGone: {
    id: "ainst-9e4410ab",
    agent: TRIAGE,
    scope: "advos",
    state: "paused",
    pauseReason: "definition_missing",
    lastCheck: "2h ago",
  },
  releaseGlobal: {
    id: "ainst-5b1c92de",
    agent: RELEASE,
    scope: "@user",
    state: "idle",
    lastCheck: "22m ago",
    nextCheck: "in 38m",
  },
} as const satisfies Record<string, DraftInstance>;
