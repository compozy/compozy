import type { NetworkParticipationStrategy } from "@/lib/network-participation";

import {
  ROOT_ENVIRONMENT_TARGET,
  type SessionEnvironmentTarget,
} from "./session-environment-target";

export interface SessionCreateDialogDraft {
  agentName: string;
  workspaceId: string;
  sessionName: string;
  networkParticipationMode: "local" | "live";
  networkChannelId: string;
  networkChannelStrategy: NetworkParticipationStrategy | "";
  /** Where the session will run. Worktrees belong to one workspace, so this resets with it. */
  environment: SessionEnvironmentTarget;
}

/** Runtime's canonical built-in agent for a new session. */
export const DEFAULT_SESSION_AGENT_NAME = "general";

/** Fields hidden by Simple mode and reset when the operator leaves Advanced. */
export const ADVANCED_DEFAULTS = {
  networkParticipationMode: "local" as const,
  networkChannelId: "",
  networkChannelStrategy: "" as NetworkParticipationStrategy | "",
  environment: ROOT_ENVIRONMENT_TARGET,
};

export const EMPTY_SESSION_CREATE_DRAFT: SessionCreateDialogDraft = {
  agentName: DEFAULT_SESSION_AGENT_NAME,
  workspaceId: "",
  sessionName: "",
  ...ADVANCED_DEFAULTS,
};

/** Preserve the selected workspace while changing the session agent. */
export function applySessionAgentSelection(
  current: SessionCreateDialogDraft,
  nextAgentName: string,
  nextWorkspaceId: string
): SessionCreateDialogDraft {
  if (current.agentName === nextAgentName && current.workspaceId === nextWorkspaceId) {
    return current;
  }
  return {
    ...EMPTY_SESSION_CREATE_DRAFT,
    agentName: nextAgentName,
    workspaceId: nextWorkspaceId,
  };
}
