import type { NetworkParticipationStrategy } from "@/lib/network-participation";

export interface SessionCreateDialogDraft {
  agentName: string;
  workspaceId: string;
  sessionName: string;
  workspacePath: string;
  networkParticipationMode: "local" | "live";
  networkChannelId: string;
  networkChannelStrategy: NetworkParticipationStrategy | "";
}

/** Fields hidden by Simple mode and reset when the operator leaves Advanced. */
export const ADVANCED_DEFAULTS = {
  workspacePath: "",
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

type SessionWorkspacePathResolution =
  | { workspacePath: string }
  | { error: "Working path must be relative to the selected workspace." }
  | { error: "Working path must stay within the selected workspace." }
  | { error: "The selected workspace root must be an absolute path." };

/**
 * Resolves the dialog's relative working-path input into the absolute create-session contract.
 * The input cannot escape the workspace selected for the session.
 */
export function resolveSessionWorkspacePath(
  workspaceRoot: string,
  workingPath: string
): SessionWorkspacePathResolution {
  const trimmedWorkingPath = workingPath.trim();
  if (trimmedWorkingPath.length === 0) {
    return { workspacePath: "" };
  }
  if (trimmedWorkingPath.startsWith("/")) {
    return { error: "Working path must be relative to the selected workspace." };
  }
  if (trimmedWorkingPath.split("/").includes("..")) {
    return { error: "Working path must stay within the selected workspace." };
  }

  const trimmedWorkspaceRoot = workspaceRoot.trim();
  if (!trimmedWorkspaceRoot.startsWith("/")) {
    return { error: "The selected workspace root must be an absolute path." };
  }

  const normalizedWorkspaceRoot =
    trimmedWorkspaceRoot === "/" ? "" : trimmedWorkspaceRoot.replace(/\/+$/, "");
  return { workspacePath: `${normalizedWorkspaceRoot}/${trimmedWorkingPath}` };
}
