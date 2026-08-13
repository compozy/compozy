import type { NetworkParticipationStrategy } from "@/lib/network-participation";

import {
  ROOT_ENVIRONMENT_TARGET,
  type SessionEnvironmentTarget,
} from "./session-environment-target";

export interface SessionCreateDialogDraft {
  agentName: string;
  workspaceId: string;
  sessionName: string;
  /**
   * The message the session sends as soon as it is ready. Prose the operator
   * typed, so unlike every other launch field it is never silently discarded:
   * it stays out of {@link ADVANCED_DEFAULTS} and survives a mode switch.
   */
  firstMessage: string;
  networkParticipationMode: "local" | "live";
  networkChannelId: string;
  networkChannelStrategy: NetworkParticipationStrategy | "";
  /** Where the session will run. Worktrees belong to one workspace, so this resets with it. */
  environment: SessionEnvironmentTarget;
}

/** Fields hidden by Simple mode and reset when the operator leaves Advanced. */
export const ADVANCED_DEFAULTS = {
  networkParticipationMode: "local" as const,
  networkChannelId: "",
  networkChannelStrategy: "" as NetworkParticipationStrategy | "",
  environment: ROOT_ENVIRONMENT_TARGET,
};

export const EMPTY_SESSION_CREATE_DRAFT: SessionCreateDialogDraft = {
  agentName: "",
  workspaceId: "",
  sessionName: "",
  firstMessage: "",
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
