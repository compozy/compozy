import type { NetworkParticipationStrategy } from "@/lib/network-participation";

export interface SessionCreateDialogDraft {
  agentName: string;
  workspaceId: string;
  sessionName: string;
  networkParticipationMode: "local" | "live";
  networkChannelId: string;
  networkChannelStrategy: NetworkParticipationStrategy | "";
}

/** Fields hidden by Simple mode and reset when the operator leaves Advanced. */
export const ADVANCED_DEFAULTS = {
  networkParticipationMode: "local" as const,
  networkChannelId: "",
  networkChannelStrategy: "" as NetworkParticipationStrategy | "",
};

export const EMPTY_SESSION_CREATE_DRAFT: SessionCreateDialogDraft = {
  agentName: "",
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
