import { Eye, Lock, PencilLine, ShieldCheck, Zap } from "@compozy/ui/icons";
import type { ComponentType } from "react";

import type { AgentPayload, CreateAgentParams } from "../types";

/** The five stored permission levels, in order of how much an agent may do unasked. */
export type AgentPermissionLevel = NonNullable<CreateAgentParams["agent"]["permissions"]>;

export interface AgentPermissionLevelOption {
  value: AgentPermissionLevel;
  label: string;
  description: string;
  icon: ComponentType<{ className?: string; size?: number }>;
}

/** One plain-language vocabulary for the five levels, shared by agents and Settings. */
export const AGENT_PERMISSION_LEVELS: readonly AgentPermissionLevelOption[] = [
  {
    value: "deny-all",
    label: "Ask before every action",
    description: "Every action waits for your approval.",
    icon: Lock,
  },
  {
    value: "approve-reads",
    label: "Ask before changes",
    description: "Reading is automatic. Changes and commands still ask.",
    icon: Eye,
  },
  {
    value: "approve-edits",
    label: "Edit on its own",
    description:
      "Edits files and runs its own coordination. Commands, deletions and settings changes still ask.",
    icon: PencilLine,
  },
  {
    value: "auto",
    label: "Let a reviewer decide",
    description: "The provider's AI reviewer approves routine work and stops risky actions.",
    icon: ShieldCheck,
  },
  {
    value: "approve-all",
    label: "Never ask",
    description: "Acts without asking. For agents you trust.",
    icon: Zap,
  },
];

/** What a stays-on agent with no `permissions` runs at. */
export const STAYS_ON_DEFAULT_PERMISSION: AgentPermissionLevel = "approve-edits";

/** On-demand agents can leave the level to Settings by omitting `permissions`. */
export const FOLLOW_SETTINGS_LABEL = "Follow Settings";

const LEVEL_VALUES = new Set<string>(AGENT_PERMISSION_LEVELS.map(level => level.value));

export function isAgentPermissionLevel(
  value: string | null | undefined
): value is AgentPermissionLevel {
  return Boolean(value && LEVEL_VALUES.has(value));
}

export function permissionLevelOption(
  value: string | null | undefined
): AgentPermissionLevelOption | undefined {
  return AGENT_PERMISSION_LEVELS.find(level => level.value === value?.trim());
}

/** Plain label for a stored permission value; unknown modes print as-is. */
export function permissionLabel(value: string | null | undefined): string {
  const trimmed = value?.trim() ?? "";
  return permissionLevelOption(trimmed)?.label ?? trimmed;
}

/** The level an agent effectively asks at, and whether it came from a default. */
export function agentPermissionSummary(agent: Pick<AgentPayload, "kind" | "permissions">): string {
  const stored = agent.permissions?.trim() ?? "";
  if (stored) return permissionLabel(stored);
  return agent.kind === "stays_on"
    ? permissionLabel(STAYS_ON_DEFAULT_PERMISSION)
    : FOLLOW_SETTINGS_LABEL;
}

/** Why another page can't save until a level is picked on Permissions. */
export function permissionsBlockedMessage(legacy: string | null | undefined): string {
  const stored = legacy?.trim() ?? "";
  return stored
    ? `Its permission level (${stored}) isn't one CompozyOS knows. Pick a level on the Permissions page, then save here.`
    : "This agent has no permission level yet. Pick one on the Permissions page, then save here.";
}
